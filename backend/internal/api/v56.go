package api

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V56 容量饱和预测 ============
// 闭环：采集(资产/机柜 created_at 回填120天历史+每小时幂等快照) → 评估(线性回归外推 U位/功率饱和日+r²置信度)
//       → 使用(容量页预测区块+历史/预测曲线) → 报警(<90天=risk 通知+Webhook+自动【容量扩容规划】工单；缓解→恢复通知)
// 口径：used_u/used_power 只算 status=1 且在架(cabinet_id>0) 资产，与 CabinetCapacity uMap 一致；
//       历史回填近似：现存资产视为自 created_at 起持续占用（忽略中途下架），代码即文档。

const capHistDays = 120 // 回填天数
const capForecastWindow = 90 // 回归窗口天数

// BackfillCapacityHistory 按站点幂等回填：对"没有任何历史行"的站点按 created_at 回填 120 天（内存聚合）。
// 启动、每小时定时器、手动评估都会调用——新站点最迟 1 小时后自动获得历史曲线。
func (h *Handler) BackfillCapacityHistory() int {
	var sites []model.Site
	h.DB.Where("status = 1").Find(&sites)
	if len(sites) == 0 {
		return 0
	}
	targets := make([]model.Site, 0, len(sites))
	for _, s := range sites {
		var cnt int64
		h.DB.Model(&model.CapacityHistory{}).Where("site_id = ?", s.ID).Count(&cnt)
		if cnt == 0 {
			targets = append(targets, s)
		}
	}
	if len(targets) == 0 {
		return 0
	}
	type assetRow struct {
		SiteID    uint
		CabinetID uint
		Status    int
		HeightU   int
		PowerW    int
		WeightKG  float64
		CreatedAt time.Time
	}
	type cabRow struct {
		SiteID       uint
		TotalU       int
		RatedPowerKW float64
		CreatedAt    time.Time
	}
	var assets []assetRow
	h.DB.Raw(`SELECT site_id, cabinet_id, status, height_u, power_w, weight_kg, created_at FROM t_asset`).Scan(&assets)
	var cabs []cabRow
	h.DB.Raw(`SELECT site_id, total_u, rated_power_kw, created_at FROM t_cabinet`).Scan(&cabs)

	today := time.Now()
	rows := make([]model.CapacityHistory, 0, len(targets)*capHistDays)
	for _, s := range targets {
		for d := capHistDays - 1; d >= 0; d-- {
			day := today.AddDate(0, 0, -d)
			dayEnd := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, day.Location())
			var totalU int
			var ratedW int64
			for _, c := range cabs {
				if c.SiteID == s.ID && !c.CreatedAt.After(dayEnd) {
					totalU += c.TotalU
					ratedW += int64(c.RatedPowerKW * 1000)
				}
			}
			if totalU == 0 && d > 0 {
				continue // 站点当天还没建机柜，跳过空行（今天除外）
			}
			var usedU, usedW, n int
			var usedKG float64
			for _, a := range assets {
				if a.SiteID == s.ID && a.Status == 1 && a.CabinetID > 0 && !a.CreatedAt.After(dayEnd) {
					usedU += a.HeightU
					usedW += a.PowerW
					usedKG += a.WeightKG
					n++
				}
			}
			rows = append(rows, model.CapacityHistory{
				SiteID: s.ID, Day: day.Format("2006-01-02"),
				UsedU: usedU, TotalU: totalU, UsedPowerW: int64(usedW), RatedPowerW: ratedW,
				UsedWeightKG: usedKG, AssetCount: n,
			})
		}
	}
	if len(rows) > 0 {
		h.DB.CreateInBatches(&rows, 200)
	}
	return len(rows)
}

// SnapshotCapacityToday 每小时幂等写当天快照（upsert site_id+day）
func (h *Handler) SnapshotCapacityToday() {
	var sites []model.Site
	h.DB.Where("status = 1").Find(&sites)
	today := time.Now().Format("2006-01-02")
	for _, s := range sites {
		var agg struct {
			TotalU  int
			RatedKW float64
		}
		h.DB.Raw(`SELECT COALESCE(SUM(total_u),0) AS total_u, COALESCE(SUM(rated_power_kw),0) AS rated_kw FROM t_cabinet WHERE site_id = ?`, s.ID).Scan(&agg)
		if agg.TotalU == 0 {
			continue
		}
		var a2 struct {
			UsedU  int
			UsedW  int64
			UsedKG float64
			N      int
		}
		h.DB.Raw(`SELECT COALESCE(SUM(height_u),0) AS used_u, COALESCE(SUM(power_w),0) AS used_w, COALESCE(SUM(weight_kg),0) AS used_kg, COUNT(*) AS n FROM t_asset WHERE site_id = ? AND status = 1 AND cabinet_id > 0`, s.ID).Scan(&a2)
		var rec model.CapacityHistory
		if err := h.DB.Where("site_id = ? AND day = ?", s.ID, today).First(&rec).Error; err == nil {
			h.DB.Model(&rec).Updates(map[string]interface{}{
				"used_u": a2.UsedU, "total_u": agg.TotalU, "used_power_w": a2.UsedW,
				"rated_power_w": int64(agg.RatedKW * 1000), "used_weight_kg": a2.UsedKG, "asset_count": a2.N,
			})
		} else {
			h.DB.Create(&model.CapacityHistory{
				SiteID: s.ID, Day: today, UsedU: a2.UsedU, TotalU: agg.TotalU,
				UsedPowerW: a2.UsedW, RatedPowerW: int64(agg.RatedKW * 1000),
				UsedWeightKG: a2.UsedKG, AssetCount: a2.N,
			})
		}
	}
}

func capGradeLabel(g string) string {
	switch g {
	case model.CapGradeRisk:
		return "90天内饱和"
	case model.CapGradeWarn:
		return "180天内饱和"
	case model.CapGradeStable:
		return "无增长"
	default:
		return "容量充足"
	}
}

func capIsBad(g string) bool { return g == model.CapGradeWarn || g == model.CapGradeRisk }

// ForecastCapacity 预测引擎：每站点回归最近90天历史 → 饱和日 + 等级变迁报警
func (h *Handler) ForecastCapacity() map[string]interface{} {
	autoWO := h.healthSetting("capacity_forecast_auto_work_order", "1") == "1"
	stats := map[string]interface{}{"evaluated": 0, "risk": 0, "warn": 0, "ok": 0, "stable": 0, "notified": 0, "recovered": 0, "work_orders": 0}
	var sites []model.Site
	h.DB.Where("status = 1").Find(&sites)
	today := time.Now()
	for _, s := range sites {
		var hist []model.CapacityHistory
		h.DB.Where("site_id = ? AND day >= ?", s.ID, today.AddDate(0, 0, -capForecastWindow).Format("2006-01-02")).
			Order("day ASC").Find(&hist)
		if len(hist) < 7 {
			continue // 数据点不足不预测
		}
		last := hist[len(hist)-1]
		if last.TotalU <= 0 {
			continue
		}
		xs := make([]float64, 0, len(hist))
		ysU := make([]float64, 0, len(hist))
		ysW := make([]float64, 0, len(hist))
		base, _ := time.Parse("2006-01-02", hist[0].Day)
		for _, r := range hist {
			d, _ := time.Parse("2006-01-02", r.Day)
			xs = append(xs, d.Sub(base).Hours()/24)
			ysU = append(ysU, float64(r.UsedU))
			ysW = append(ysW, float64(r.UsedPowerW))
		}
		uSlope, _, uR2 := linearRegression(xs, ysU)
		wSlope, _, wR2 := linearRegression(xs, ysW)

		fullAt := func(cur float64, limit float64, slope, r2 float64) string {
			if slope < 0.001 || r2 < 0.3 || limit <= 0 || cur >= limit {
				if cur >= limit && limit > 0 {
					return today.Format("2006-01-02") // 已饱和
				}
				return ""
			}
			days := (limit - cur) / slope
			if days > 3650 {
				return ""
			}
			return today.AddDate(0, 0, int(math.Ceil(days))).Format("2006-01-02")
		}
		uFull := fullAt(float64(last.UsedU), float64(last.TotalU), uSlope, uR2)
		wFull := fullAt(float64(last.UsedPowerW), float64(last.RatedPowerW), wSlope, wR2)

		grade := model.CapGradeOK
		lowConf := (uR2 < 0.3 && uSlope >= 0.001) || (wR2 < 0.3 && wSlope >= 0.001)
		daysUntil := func(fs string) int {
			if fs == "" {
				return 99999
			}
			t, _ := time.Parse("2006-01-02", fs)
			return int(t.Sub(today).Hours() / 24)
		}
		minDays := daysUntil(uFull)
		if d := daysUntil(wFull); d < minDays {
			minDays = d
		}
		confident := func(fs string, r2 float64) bool { return fs != "" && r2 >= 0.3 }
		if (confident(uFull, uR2) || confident(wFull, wR2)) && !lowConfOnly(lowConf, uFull, uR2, wFull, wR2) {
			if minDays <= 90 {
				grade = model.CapGradeRisk
			} else if minDays <= 180 {
				grade = model.CapGradeWarn
			}
		}
		if uSlope < 0.001 && wSlope < 0.001 && uFull == "" && wFull == "" {
			grade = model.CapGradeStable
		}

		var rec model.CapacityForecast
		err := h.DB.Where("site_id = ?", s.ID).First(&rec).Error
		existed := err == nil
		oldGrade := ""
		if existed {
			oldGrade = rec.Grade
			if rec.OpenWorkOrderID > 0 {
				var wo model.WorkOrder
				if h.DB.Select("id", "status").First(&wo, rec.OpenWorkOrderID).Error == nil && wo.Status > 2 {
					rec.OpenWorkOrderID = 0
				}
			}
		} else {
			rec = model.CapacityForecast{SiteID: s.ID}
		}
		rec.Grade, rec.USlope, rec.UFullAt, rec.UR2 = grade, uSlope, uFull, uR2
		rec.PowerSlope, rec.PowerFullAt, rec.PowerR2 = wSlope, wFull, wR2
		rec.LowConfidence = lowConf
		rec.UsedU, rec.TotalU = last.UsedU, last.TotalU
		rec.UsedPowerW, rec.RatedPowerW = last.UsedPowerW, last.RatedPowerW
		rec.ComputedAt = time.Now()

		// 等级变迁 → 报警/恢复（低置信度不推送——V29 刷屏教训）
		if grade != oldGrade && !lowConf {
			if capIsBad(grade) && rec.LastNotifiedGrade != grade {
				title := fmt.Sprintf("容量饱和预警：%s %s", s.Name, capGradeLabel(grade))
				content := h.capForecastContent(&s, &rec)
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "capacity_forecast", Title: title, Content: content,
					RefType: "site", RefID: s.ID,
				})
				h.DispatchWebhook(model.EvCapacityForecast, title, content, map[string]interface{}{
					"site_id": s.ID, "site_name": s.Name, "grade": grade,
					"u_full_at": uFull, "power_full_at": wFull,
				})
				stats["notified"] = stats["notified"].(int) + 1
				if autoWO && grade == model.CapGradeRisk && rec.OpenWorkOrderID == 0 {
					if woID := h.createCapacityWorkOrder(&s, &rec, true); woID > 0 {
						rec.OpenWorkOrderID = woID
						stats["work_orders"] = stats["work_orders"].(int) + 1
					}
				}
				rec.LastNotifiedGrade = grade
			} else if existed && capIsBad(oldGrade) && !capIsBad(grade) && capIsBad(rec.LastNotifiedGrade) {
				title := fmt.Sprintf("容量预警解除：%s 增长放缓/扩容生效", s.Name)
				content := fmt.Sprintf("站点「%s」容量饱和风险由 %s 缓解至 %s，U位 %d/%d、功率 %.1f/%.1f kW。",
					s.Name, capGradeLabel(oldGrade), capGradeLabel(grade), rec.UsedU, rec.TotalU,
					float64(rec.UsedPowerW)/1000, float64(rec.RatedPowerW)/1000)
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "capacity_forecast", Title: title, Content: content,
					RefType: "site", RefID: s.ID,
				})
				h.DispatchWebhook(model.EvCapacityForecast, title, content, map[string]interface{}{
					"site_id": s.ID, "site_name": s.Name, "grade": grade, "recovered": true,
				})
				stats["recovered"] = stats["recovered"].(int) + 1
				rec.LastNotifiedGrade = grade
			}
		}

		if existed {
			h.DB.Model(&model.CapacityForecast{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
				"grade": rec.Grade, "u_slope": rec.USlope, "u_full_at": rec.UFullAt, "ur2": rec.UR2,
				"power_slope": rec.PowerSlope, "power_full_at": rec.PowerFullAt, "power_r2": rec.PowerR2,
				"low_confidence": rec.LowConfidence, "used_u": rec.UsedU, "total_u": rec.TotalU,
				"used_power_w": rec.UsedPowerW, "rated_power_w": rec.RatedPowerW,
				"open_work_order_id": rec.OpenWorkOrderID, "last_notified_grade": rec.LastNotifiedGrade,
				"computed_at": rec.ComputedAt,
			})
		} else {
			h.DB.Create(&rec)
		}
		stats["evaluated"] = stats["evaluated"].(int) + 1
		stats[grade] = stats[grade].(int) + 1
	}
	return stats
}

// lowConfOnly 所有"会饱和"的维度都是低置信 → 不报警（只展示）
func lowConfOnly(lowConf bool, uFull string, uR2 float64, wFull string, wR2 float64) bool {
	if !lowConf {
		return false
	}
	uConfident := uFull != "" && uR2 >= 0.3
	wConfident := wFull != "" && wR2 >= 0.3
	return !uConfident && !wConfident
}

func (h *Handler) capForecastContent(s *model.Site, rec *model.CapacityForecast) string {
	c := fmt.Sprintf("站点「%s」容量预测（%s）：\n", s.Name, capGradeLabel(rec.Grade))
	c += fmt.Sprintf("- U 位：%d/%d，增速 %.2f U/天", rec.UsedU, rec.TotalU, rec.USlope)
	if rec.UFullAt != "" {
		c += fmt.Sprintf("，预计 %s 饱和（置信度 r²=%.2f）", rec.UFullAt, rec.UR2)
	}
	c += "\n"
	c += fmt.Sprintf("- 功率：%.1f/%.1f kW，增速 %.0f W/天", float64(rec.UsedPowerW)/1000, float64(rec.RatedPowerW)/1000, rec.PowerSlope)
	if rec.PowerFullAt != "" {
		c += fmt.Sprintf("，预计 %s 饱和（置信度 r²=%.2f）", rec.PowerFullAt, rec.PowerR2)
	}
	c += "\n建议提前规划扩容：新增机柜/提高单柜功率密度/迁移低负载设备。"
	return c
}

// createCapacityWorkOrder 容量扩容规划工单（幂等）
func (h *Handler) createCapacityWorkOrder(s *model.Site, rec *model.CapacityForecast, auto bool) uint {
	var openCnt int64
	h.DB.Model(&model.WorkOrder{}).Where("site_id = ? AND title LIKE ? AND status <= 2", s.ID, "【容量扩容规划】%").Count(&openCnt)
	if openCnt > 0 {
		return 0
	}
	desc := h.capForecastContent(s, rec)
	if auto {
		desc += "\n\n本工单由容量饱和预测引擎自动生成。"
	}
	now := time.Now()
	wo := model.WorkOrder{
		SiteID: s.ID, Code: h.genCode("WO"),
		Title:       fmt.Sprintf("【容量扩容规划】%s %s", s.Name, capGradeLabel(rec.Grade)),
		Type:        "task", Priority: 2, Status: 1,
		Description: desc,
		CreatedAt:   now, UpdatedAt: now,
	}
	assignNote := ""
	if duty := h.OnDutyUser(s.ID); duty != nil {
		wo.AssigneeID = duty.UserID
		var u model.User
		if h.DB.Select("display_name", "username").First(&u, duty.UserID).Error == nil {
			nm := u.DisplayName
			if nm == "" {
				nm = u.Username
			}
			assignNote = "，自动指派当班人 " + nm
		}
	}
	h.applySLA(&wo)
	if err := h.DB.Create(&wo).Error; err != nil {
		return 0
	}
	src := "手动创建"
	if auto {
		src = "容量预测引擎自动创建"
	}
	h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "create", Content: src + assignNote})
	h.DispatchWebhook(model.EvWorkOrder, "容量扩容规划工单 "+wo.Code, wo.Title+assignNote, map[string]interface{}{
		"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "site_id": s.ID, "source": "capacity_forecast",
	})
	return wo.ID
}

// ============ HTTP Handlers ============

// ListCapacityForecast GET /capacity/forecast
func (h *Handler) ListCapacityForecast(c *gin.Context) {
	var recs []model.CapacityForecast
	q := h.DB.Model(&model.CapacityForecast{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	q.Find(&recs)
	siteName := map[uint]string{}
	{
		var ss []model.Site
		h.DB.Select("id", "name").Find(&ss)
		for _, s := range ss {
			siteName[s.ID] = s.Name
		}
	}
	items := make([]gin.H, 0, len(recs))
	sum := map[string]int{"ok": 0, "warn": 0, "risk": 0, "stable": 0}
	for _, r := range recs {
		sum[r.Grade]++
		days := -1
		fullAt := r.UFullAt
		if fullAt == "" || (r.PowerFullAt != "" && r.PowerFullAt < fullAt) {
			fullAt = r.PowerFullAt
		}
		if fullAt != "" {
			if t, err := time.Parse("2006-01-02", fullAt); err == nil {
				days = int(time.Until(t).Hours() / 24)
			}
		}
		items = append(items, gin.H{
			"site_id": r.SiteID, "site_name": siteName[r.SiteID], "grade": r.Grade,
			"u_full_at": r.UFullAt, "power_full_at": r.PowerFullAt, "days_to_full": days,
			"u_slope": r.USlope, "u_r2": r.UR2, "power_slope": r.PowerSlope, "power_r2": r.PowerR2,
			"low_confidence": r.LowConfidence,
			"used_u": r.UsedU, "total_u": r.TotalU, "used_power_w": r.UsedPowerW, "rated_power_w": r.RatedPowerW,
			"open_work_order_id": r.OpenWorkOrderID, "computed_at": r.ComputedAt,
		})
	}
	sort.Slice(items, func(a, b int) bool {
		da, db := items[a]["days_to_full"].(int), items[b]["days_to_full"].(int)
		if da < 0 {
			da = 99999
		}
		if db < 0 {
			db = 99999
		}
		return da < db
	})
	response.Success(c, gin.H{"items": items, "summary": sum})
}

// GetCapacityForecastSite GET /capacity/forecast/:site_id 历史+预测曲线数据
func (h *Handler) GetCapacityForecastSite(c *gin.Context) {
	siteID, ok := parseSiteIDParam(c)
	if !ok {
		return
	}
	if scope := middleware.GetSiteScope(c); scope != nil {
		allowed := false
		for _, s := range scope {
			if s == siteID {
				allowed = true
			}
		}
		if !allowed {
			response.Forbidden(c, "无权查看该站点")
			return
		}
	}
	var site model.Site
	if err := h.DB.First(&site, siteID).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var hist []model.CapacityHistory
	h.DB.Where("site_id = ?", siteID).Order("day ASC").Find(&hist)
	history := make([]gin.H, 0, len(hist))
	for _, r := range hist {
		history = append(history, gin.H{
			"day": r.Day, "used_u": r.UsedU, "total_u": r.TotalU,
			"used_power_w": r.UsedPowerW, "rated_power_w": r.RatedPowerW, "asset_count": r.AssetCount,
		})
	}
	var rec model.CapacityForecast
	h.DB.Where("site_id = ?", siteID).First(&rec)
	// 预测外推线：从今天起按斜率外推（最长180天或到饱和日+30天）
	forecast := make([]gin.H, 0)
	if rec.ID > 0 {
		today := time.Now()
		n := 180
		if rec.UFullAt != "" {
			if t, err := time.Parse("2006-01-02", rec.UFullAt); err == nil {
				if d := int(t.Sub(today).Hours()/24) + 30; d < n {
					n = d
				}
			}
		}
		for d := 0; d <= n; d += 3 {
			day := today.AddDate(0, 0, d)
			fu := float64(rec.UsedU) + rec.USlope*float64(d)
			fw := float64(rec.UsedPowerW) + rec.PowerSlope*float64(d)
			if fu > float64(rec.TotalU)*1.05 {
				fu = float64(rec.TotalU) * 1.05
			}
			forecast = append(forecast, gin.H{
				"day": day.Format("2006-01-02"),
				"u":   math.Round(fu*10) / 10,
				"w":   math.Round(fw),
			})
		}
	}
	response.Success(c, gin.H{
		"site":     gin.H{"id": site.ID, "name": site.Name},
		"history":  history,
		"forecast": forecast,
		"record":   rec,
	})
}

// EvaluateCapacityForecast POST /capacity/forecast/evaluate 手动触发（admin）
func (h *Handler) EvaluateCapacityForecast(c *gin.Context) {
	h.BackfillCapacityHistory() // 新站点先补历史
	h.SnapshotCapacityToday()   // 再刷今天快照，最后预测
	stats := h.ForecastCapacity()
	detail, _ := json.Marshal(stats)
	go middleware.WriteAudit(h.DB, c, "evaluate", "capacity_forecast", string(detail))
	response.Success(c, stats)
}

// CreateCapacityWorkOrderAPI POST /capacity/forecast/:site_id/work-order 手动开扩容工单（ops）
func (h *Handler) CreateCapacityWorkOrderAPI(c *gin.Context) {
	siteID, ok := parseSiteIDParam(c)
	if !ok {
		return
	}
	var site model.Site
	if err := h.DB.First(&site, siteID).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var rec model.CapacityForecast
	if h.DB.Where("site_id = ?", siteID).First(&rec).Error != nil {
		response.BadRequest(c, "该站点暂无预测记录，请先执行评估")
		return
	}
	if rec.OpenWorkOrderID > 0 {
		var wo model.WorkOrder
		if h.DB.Select("id", "code", "status").First(&wo, rec.OpenWorkOrderID).Error == nil && wo.Status <= 2 {
			response.BadRequest(c, "该站点已有未闭环扩容工单 "+wo.Code)
			return
		}
	}
	woID := h.createCapacityWorkOrder(&site, &rec, false)
	if woID == 0 {
		response.BadRequest(c, "已存在未闭环的容量扩容规划工单")
		return
	}
	h.DB.Model(&model.CapacityForecast{}).Where("site_id = ?", siteID).Update("open_work_order_id", woID)
	go middleware.WriteAudit(h.DB, c, "create", "work_order", fmt.Sprintf("手动创建容量扩容工单 site=%d wo=%d", siteID, woID))
	response.Success(c, gin.H{"work_order_id": woID})
}
