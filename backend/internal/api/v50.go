package api

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V50 设备健康度与预测性维护 ============
//
// 闭环链路：
//   采集(既有测点/告警/维保/供电链路) → 评估(ScanAssetHealth 扣分制8因子)
//   → 报警(等级恶化: 通知+Webhook+自动工单) → 使用(资产列表/详情/Dashboard)
//   → 维护(工单处理/维保续期) → 分数回升 → 恢复通知
//
// 因子与扣分（每项带证据，可解释、可追溯）：
//   active_alert   当前活跃告警         sev3=15/sev2=8/sev1=3，上限20
//   alarm_30d      近30天告警史         sev3×6+sev2×3+sev1×1，上限30
//   threshold      阈值裕度             当前值越限=20 / ≥90%阈值=10 / ≥80%=5（取最差测点）
//   trend          7天劣化趋势          r²≥0.3 且 14天内预计越限=15（复用 V29 回归，吸取误报教训）
//   offline        绑定测点离线率       >50%=10 / >0=5
//   age            服役年限             ≥参考寿命=15 / ≥80%=10 / ≥60%=5
//   warranty       维保覆盖             无有效合同=8 / 30天内到期=4
//   repair_status  资产维修中           status=3 → 10
// 等级：≥85 healthy / ≥70 attention / ≥55 sub_healthy / <55 risk（health_grade_thresholds 可配）

type healthFactor struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Deduct   int    `json:"deduct"`
	Evidence string `json:"evidence"`
}

// healthCategoryLifespan 分类参考寿命（年），服役年限因子基准
var healthCategoryLifespan = map[string]float64{
	"power": 10, "cooling": 12, "server": 6, "network": 7,
	"storage": 6, "security": 8, "other": 8,
}

// healthCategoryPointTypes 辅助房间资产按分类关联的测点类型
var healthCategoryPointTypes = map[string][]string{
	"power":   {"power_dist", "ups"},
	"cooling": {"hvac", "cdu"},
	"server":  {"temp_humidity", "smoke", "water_leak", "leak_rope"},
	"network": {"temp_humidity", "smoke", "water_leak", "leak_rope"},
	"storage": {"temp_humidity", "smoke", "water_leak", "leak_rope"},
	"other":   {"temp_humidity"},
}

func (h *Handler) healthSetting(key, fallback string) string {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", key).Error; err != nil || strings.TrimSpace(s.Value) == "" {
		return fallback
	}
	return strings.TrimSpace(s.Value)
}

// healthThresholds 等级阈值（healthy≥a, attention≥b, sub_healthy≥c, 其余 risk）
func (h *Handler) healthThresholds() (int, int, int) {
	a, b, c := 85, 70, 55
	parts := strings.Split(h.healthSetting("health_grade_thresholds", "85,70,55"), ",")
	if len(parts) == 3 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && v > 0 && v <= 100 {
			a = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && v > 0 && v < a {
			b = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil && v > 0 && v < b {
			c = v
		}
	}
	return a, b, c
}

func healthGradeOf(score int, a, b, c int) string {
	switch {
	case score >= a:
		return model.HealthGradeHealthy
	case score >= b:
		return model.HealthGradeAttention
	case score >= c:
		return model.HealthGradeSubHealth
	default:
		return model.HealthGradeRisk
	}
}

func healthGradeLabel(g string) string {
	switch g {
	case model.HealthGradeHealthy:
		return "健康"
	case model.HealthGradeAttention:
		return "关注"
	case model.HealthGradeSubHealth:
		return "亚健康"
	case model.HealthGradeRisk:
		return "风险"
	}
	return g
}

func healthIsBad(g string) bool {
	return g == model.HealthGradeSubHealth || g == model.HealthGradeRisk
}

// healthPointsForAsset 解析资产绑定测点：供电链路绑定 > 机柜绑定 > 房间+分类映射（去重）
func (h *Handler) healthPointsForAsset(a *model.Asset, chainPts map[uint][]uint, points []model.MonitoringPoint) []model.MonitoringPoint {
	seen := map[uint]bool{}
	out := []model.MonitoringPoint{}
	add := func(p model.MonitoringPoint) {
		if !seen[p.ID] {
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	for _, pid := range chainPts[a.ID] {
		for _, p := range points {
			if p.ID == pid {
				add(p)
			}
		}
	}
	if a.CabinetID > 0 {
		for _, p := range points {
			if p.CabinetID != nil && *p.CabinetID == a.CabinetID {
				add(p)
			}
		}
	}
	if a.RoomID > 0 {
		types := healthCategoryPointTypes[a.Category]
		if types == nil {
			types = healthCategoryPointTypes["other"]
		}
		tset := map[string]bool{}
		for _, t := range types {
			tset[t] = true
		}
		for _, p := range points {
			if p.RoomID != nil && *p.RoomID == a.RoomID && tset[p.Type] {
				add(p)
			}
		}
	}
	return out
}

// dailyAvgRow 7天日均值（趋势因子）
type dailyAvgRow struct {
	PointID uint    `json:"point_id"`
	Day     string  `json:"day"`
	AvgVal  float64 `json:"avg_val"`
}

// computeAssetHealth 计算单个资产健康度，返回分数+因子列表
func (h *Handler) computeAssetHealth(a *model.Asset, pts []model.MonitoringPoint,
	alerts30d map[uint][]model.Alert, trendData map[uint][]dailyAvgRow,
	warranty map[uint]time.Time, contractCnt map[uint]int) (int, []healthFactor) {

	factors := []healthFactor{}
	deduct := 0
	addF := func(key, label string, d int, ev string) {
		if d > 0 {
			factors = append(factors, healthFactor{Key: key, Label: label, Deduct: d, Evidence: ev})
			deduct += d
		}
	}

	// 1) 活跃告警 + 30天告警史
	activeSev := map[int]int{}
	hist3, hist2, hist1 := 0, 0, 0
	for _, p := range pts {
		for _, al := range alerts30d[p.ID] {
			if al.Status < 3 {
				activeSev[al.Severity]++
			}
			switch al.Severity {
			case 3:
				hist3++
			case 2:
				hist2++
			default:
				hist1++
			}
		}
	}
	if n := activeSev[3] + activeSev[2] + activeSev[1]; n > 0 {
		d := activeSev[3]*15 + activeSev[2]*8 + activeSev[1]*3
		if d > 20 {
			d = 20
		}
		addF("active_alert", "活跃告警", d, fmt.Sprintf("绑定测点存在 %d 条活跃告警（严重%d/警告%d/提示%d）", n, activeSev[3], activeSev[2], activeSev[1]))
	}
	if hist3+hist2+hist1 > 0 {
		d := hist3*6 + hist2*3 + hist1
		if d > 30 {
			d = 30
		}
		addF("alarm_30d", "近30天告警史", d, fmt.Sprintf("近30天累计告警 %d 次（严重%d/警告%d/提示%d）", hist3+hist2+hist1, hist3, hist2, hist1))
	}

	// 2) 阈值裕度（取最差绑定测点）
	worstD, worstEv := 0, ""
	for _, p := range pts {
		if p.CurrentValue == nil {
			continue
		}
		cur := *p.CurrentValue
		hi := 0.0
		if p.HighThreshold != nil {
			hi = *p.HighThreshold
		} else if p.MaxThreshold != nil {
			hi = *p.MaxThreshold
		}
		d, ev := 0, ""
		if hi > 0 {
			if cur > hi {
				d = 20
				ev = fmt.Sprintf("测点「%s」当前值 %.2f%s 已越限（上限 %.2f）", p.Name, cur, p.Unit, hi)
			} else if ratio := cur / hi; ratio >= 0.9 {
				d = 10
				ev = fmt.Sprintf("测点「%s」当前值 %.2f%s 已达上限 %.0f%%", p.Name, cur, p.Unit, ratio*100)
			} else if ratio := cur / hi; ratio >= 0.8 {
				d = 5
				ev = fmt.Sprintf("测点「%s」当前值 %.2f%s 已达上限 %.0f%%", p.Name, cur, p.Unit, ratio*100)
			}
		}
		if p.MinThreshold != nil && cur < *p.MinThreshold {
			if d < 10 {
				d = 10
				ev = fmt.Sprintf("测点「%s」当前值 %.2f%s 低于下限 %.2f", p.Name, cur, p.Unit, *p.MinThreshold)
			}
		}
		if d > worstD {
			worstD, worstEv = d, ev
		}
	}
	addF("threshold", "阈值裕度", worstD, worstEv)

	// 3) 7天劣化趋势（r²≥0.3 且按斜率14天内越限才扣分——吸取 V29 误报教训）
	trD, trEv := 0, ""
	for _, p := range pts {
		hi := 0.0
		if p.HighThreshold != nil {
			hi = *p.HighThreshold
		} else if p.MaxThreshold != nil {
			hi = *p.MaxThreshold
		}
		if hi <= 0 {
			continue
		}
		rows := trendData[p.ID]
		if len(rows) < 4 {
			continue
		}
		xs := make([]float64, 0, len(rows))
		ys := make([]float64, 0, len(rows))
		for i, r := range rows {
			xs = append(xs, float64(i))
			ys = append(ys, r.AvgVal)
		}
		slope, _, r2 := linearRegression(xs, ys)
		if r2 < 0.3 || slope <= 0 {
			continue
		}
		last := ys[len(ys)-1]
		if last >= hi {
			continue // 已越限由阈值因子负责
		}
		daysToLimit := (hi - last) / slope
		if daysToLimit <= 14 {
			d := 15
			ev := fmt.Sprintf("测点「%s」近7天持续上升（日均 +%.2f%s，拟合度 %.2f），预计 %.0f 天后越限 %.2f",
				p.Name, slope, p.Unit, r2, daysToLimit, hi)
			if d > trD {
				trD, trEv = d, ev
			}
		}
	}
	addF("trend", "劣化趋势", trD, trEv)

	// 4) 测点离线率（数据质量）
	if len(pts) > 0 {
		off := 0
		for _, p := range pts {
			if p.Status == 0 {
				off++
			}
		}
		if off > 0 {
			d := 5
			if float64(off)/float64(len(pts)) > 0.5 {
				d = 10
			}
			addF("offline", "监测数据缺失", d, fmt.Sprintf("绑定 %d 个测点中 %d 个离线，健康评估数据不完整", len(pts), off))
		}
	}

	// 5) 服役年限
	if a.PurchaseAt != nil {
		life := healthCategoryLifespan[a.Category]
		if life <= 0 {
			life = 8
		}
		years := time.Since(*a.PurchaseAt).Hours() / 24 / 365
		ratio := years / life
		if ratio >= 1 {
			addF("age", "服役年限", 15, fmt.Sprintf("已服役 %.1f 年，超过该类设备参考寿命 %.0f 年", years, life))
		} else if ratio >= 0.8 {
			addF("age", "服役年限", 10, fmt.Sprintf("已服役 %.1f 年，达参考寿命 %.0f 年的 %.0f%%", years, life, ratio*100))
		} else if ratio >= 0.6 {
			addF("age", "服役年限", 5, fmt.Sprintf("已服役 %.1f 年，达参考寿命 %.0f 年的 %.0f%%", years, life, ratio*100))
		}
	}

	// 6) 维保覆盖
	if contractCnt[a.ID] == 0 {
		if end, ok := warranty[a.ID]; ok && end.Before(time.Now().Add(30*24*time.Hour)) {
			addF("warranty", "维保覆盖", 4, fmt.Sprintf("维保合同将于 %s 到期（30天内）", end.Format("2006-01-02")))
		} else {
			addF("warranty", "维保覆盖", 8, "无有效维保合同覆盖，故障时缺乏厂商支持")
		}
	}

	// 7) 维修中状态
	if a.Status == 3 {
		addF("repair_status", "维修中", 10, "资产当前处于维修中状态")
	}

	score := 100 - deduct
	if score < 0 {
		score = 0
	}
	return score, factors
}

// ScanAssetHealth 全量健康评估（定时15分钟 + 手动触发共用）
func (h *Handler) ScanAssetHealth() map[string]interface{} {
	thA, thB, thC := h.healthThresholds()
	autoWO := h.healthSetting("health_auto_work_order", "1") == "1"

	var assets []model.Asset
	h.DB.Where("status IN ?", []int{1, 3}).Find(&assets)
	if len(assets) == 0 {
		return map[string]interface{}{"evaluated": 0}
	}

	// 预加载：站点测点 / 供电链路绑定 / 30天告警 / 维保 / 已有健康记录
	var points []model.MonitoringPoint
	h.DB.Find(&points)
	chainPts := map[uint][]uint{}
	{
		type cpRow struct {
			AssetID uint    `json:"asset_id"`
			PointID uint    `json:"point_id"`
		}
		var rows []cpRow
		h.DB.Raw(`SELECT asset_id, point_id FROM t_power_chain WHERE asset_id > 0 AND point_id > 0`).Scan(&rows)
		for _, r := range rows {
			chainPts[r.AssetID] = append(chainPts[r.AssetID], r.PointID)
		}
	}
	alerts30d := map[uint][]model.Alert{}
	{
		var alerts []model.Alert
		h.DB.Where("triggered_at > ?", time.Now().Add(-30*24*time.Hour)).Find(&alerts)
		for _, al := range alerts {
			if al.PointID != nil {
				alerts30d[*al.PointID] = append(alerts30d[*al.PointID], al)
			}
		}
	}
	trendData := map[uint][]dailyAvgRow{}
	{
		var rows []dailyAvgRow
		h.DB.Raw(`SELECT point_id, to_char(date_trunc('day', timestamp),'YYYY-MM-DD') as day, AVG(value) as avg_val
			FROM t_reading WHERE timestamp > ? GROUP BY point_id, day ORDER BY point_id, day`,
			time.Now().Add(-7*24*time.Hour)).Scan(&rows)
		for _, r := range rows {
			trendData[r.PointID] = append(trendData[r.PointID], r)
		}
	}
	warranty := map[uint]time.Time{}   // asset_id -> 最近有效合同到期日
	contractCnt := map[uint]int{}      // asset_id -> 有效合同数
	{
		type wRow struct {
			AssetID uint      `json:"asset_id"`
			EndAt   time.Time `json:"end_at"`
		}
		var rows []wRow
		h.DB.Raw(`SELECT asset_id, end_at FROM t_maintenance WHERE kind='contract' AND status=1 AND end_at > NOW()`).Scan(&rows)
		for _, r := range rows {
			contractCnt[r.AssetID]++
			if cur, ok := warranty[r.AssetID]; !ok || r.EndAt.After(cur) {
				warranty[r.AssetID] = r.EndAt
			}
		}
	}
	oldHealth := map[uint]model.AssetHealth{}
	{
		var rows []model.AssetHealth
		h.DB.Find(&rows)
		for _, r := range rows {
			oldHealth[r.AssetID] = r
		}
	}
	today := time.Now().Format("2006-01-02")

	stats := map[string]interface{}{"evaluated": 0, "risk": 0, "sub_healthy": 0, "attention": 0, "healthy": 0, "notified": 0, "work_orders": 0, "recovered": 0}
	for i := range assets {
		a := &assets[i]
		pts := h.healthPointsForAsset(a, chainPts, points)
		score, factors := h.computeAssetHealth(a, pts, alerts30d, trendData, warranty, contractCnt)
		grade := healthGradeOf(score, thA, thB, thC)
		fj, _ := json.Marshal(factors)

		old, existed := oldHealth[a.ID]
		rec := model.AssetHealth{
			AssetID: a.ID, SiteID: a.SiteID, Score: score, Grade: grade,
			Factors: string(fj), PointCount: len(pts), ComputedAt: time.Now(),
		}
		if existed {
			rec.ID = old.ID
			rec.OpenWorkOrderID = old.OpenWorkOrderID
			rec.LastNotifiedGrade = old.LastNotifiedGrade
			// 关联工单已闭环则清零幂等键
			if rec.OpenWorkOrderID > 0 {
				var wo model.WorkOrder
				if err := h.DB.First(&wo, rec.OpenWorkOrderID).Error; err != nil || wo.Status >= 3 {
					rec.OpenWorkOrderID = 0
				}
			}
			h.DB.Model(&model.AssetHealth{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
				"site_id": rec.SiteID, "score": score, "grade": grade, "factors": string(fj),
				"point_count": len(pts), "open_work_order_id": rec.OpenWorkOrderID,
				"last_notified_grade": rec.LastNotifiedGrade, "computed_at": rec.ComputedAt,
			})
		} else {
			h.DB.Create(&rec)
		}

		// 每日快照（asset_id+day 唯一，当日首评写入）
		var snapCnt int64
		h.DB.Model(&model.AssetHealthHistory{}).Where("asset_id = ? AND day = ?", a.ID, today).Count(&snapCnt)
		if snapCnt == 0 {
			h.DB.Create(&model.AssetHealthHistory{AssetID: a.ID, Day: today, Score: score, Grade: grade})
		}

		stats["evaluated"] = stats["evaluated"].(int) + 1
		stats[grade] = stats[grade].(int) + 1

		// 等级变迁 → 报警/恢复（幂等：LastNotifiedGrade 相同不重发）
		oldGrade := ""
		if existed {
			oldGrade = old.Grade
		}
		if grade != oldGrade {
			if healthIsBad(grade) && rec.LastNotifiedGrade != grade {
				title := fmt.Sprintf("设备健康预警：%s（%s）健康分 %d", a.Name, a.Code, score)
				content := h.healthNotifyContent(a, score, grade, factors)
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "asset_health", Title: title, Content: content,
					RefType: "asset", RefID: a.ID,
				})
				h.DispatchWebhook(model.EvAssetHealth, title, content, map[string]interface{}{
					"asset_id": a.ID, "asset_code": a.Code, "asset_name": a.Name,
					"score": score, "grade": grade, "site_id": a.SiteID,
				})
				stats["notified"] = stats["notified"].(int) + 1
				// 风险等级自动开预测性维护工单（幂等：无未闭环健康工单才开）
				if autoWO && grade == model.HealthGradeRisk && rec.OpenWorkOrderID == 0 {
					if woID := h.createHealthWorkOrder(a, score, grade, factors, true); woID > 0 {
						h.DB.Model(&model.AssetHealth{}).Where("id = ?", rec.ID).Update("open_work_order_id", woID)
						stats["work_orders"] = stats["work_orders"].(int) + 1
					}
				}
				h.DB.Model(&model.AssetHealth{}).Where("id = ?", rec.ID).Update("last_notified_grade", grade)
			} else if existed && healthIsBad(oldGrade) && !healthIsBad(grade) && healthIsBad(rec.LastNotifiedGrade) {
				title := fmt.Sprintf("设备健康恢复：%s（%s）健康分回升至 %d", a.Name, a.Code, score)
				content := fmt.Sprintf("资产 %s（%s）经维护后健康分由 %s 区间回升至 %d 分（%s），预测性维护闭环完成。",
					a.Name, a.Code, healthGradeLabel(oldGrade), score, healthGradeLabel(grade))
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "asset_health", Title: title, Content: content,
					RefType: "asset", RefID: a.ID,
				})
				h.DispatchWebhook(model.EvAssetHealth, title, content, map[string]interface{}{
					"asset_id": a.ID, "asset_code": a.Code, "score": score, "grade": grade,
					"recovered": true, "site_id": a.SiteID,
				})
				stats["recovered"] = stats["recovered"].(int) + 1
				h.DB.Model(&model.AssetHealth{}).Where("id = ?", rec.ID).Update("last_notified_grade", grade)
			}
		}
	}
	return stats
}

func (h *Handler) healthNotifyContent(a *model.Asset, score int, grade string, factors []healthFactor) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("资产 %s（%s，%s）健康评分 %d/100，等级【%s】。主要问题：",
		a.Name, a.Code, a.Category, score, healthGradeLabel(grade)))
	n := 0
	for _, f := range factors {
		if n >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("%s（-%d分：%s）；", f.Label, f.Deduct, f.Evidence))
		n++
	}
	if grade == model.HealthGradeRisk {
		sb.WriteString("建议立即安排预防性维护。")
	} else {
		sb.WriteString("建议纳入近期维护计划。")
	}
	return sb.String()
}

// createHealthWorkOrder 创建预测性维护工单（幂等：已有未闭环健康工单则跳过），返回工单ID
func (h *Handler) createHealthWorkOrder(a *model.Asset, score int, grade string, factors []healthFactor, auto bool) uint {
	// 幂等检查：该资产是否已有未闭环的预测性维护工单
	var openCnt int64
	h.DB.Model(&model.WorkOrder{}).Where("asset_id = ? AND title LIKE ? AND status <= 2", a.ID, "【预测性维护】%").Count(&openCnt)
	if openCnt > 0 {
		return 0
	}
	priority := 3
	if grade == model.HealthGradeRisk {
		priority = 2
	}
	desc := fmt.Sprintf("健康评分 %d/100（%s）。评估因子：\n", score, healthGradeLabel(grade))
	for _, f := range factors {
		desc += fmt.Sprintf("- %s（-%d分）：%s\n", f.Label, f.Deduct, f.Evidence)
	}
	if auto {
		desc += "\n本工单由健康评估引擎自动生成。"
	}
	now := time.Now()
	wo := model.WorkOrder{
		SiteID: a.SiteID, Code: h.genCode("WO"),
		Title:       fmt.Sprintf("【预测性维护】%s 健康分 %d", a.Name, score),
		Type:        "task", Priority: priority, Status: 1,
		Description: desc, AssetID: a.ID, CabinetID: a.CabinetID,
		CreatedAt: now, UpdatedAt: now,
	}
	// 与告警自动工单同规格：指派当班人 + SLA 时限
	assignNote := ""
	if duty := h.OnDutyUser(a.SiteID); duty != nil {
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
		src = "健康评估引擎自动创建"
	}
	h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "create", Content: src + assignNote})
	h.DispatchWebhook(model.EvWorkOrder, "预测性维护工单 "+wo.Code, wo.Title+assignNote, map[string]interface{}{
		"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "asset_id": a.ID, "source": "asset_health",
	})
	return wo.ID
}

// ============ HTTP Handlers ============

// GetAssetHealthSummary GET /asset-health/summary
func (h *Handler) GetAssetHealthSummary(c *gin.Context) {
	q := h.DB.Model(&model.AssetHealth{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		q = q.Where("site_id = ?", sid)
	}
	var rows []model.AssetHealth
	q.Find(&rows)
	grades := map[string]int{"healthy": 0, "attention": 0, "sub_healthy": 0, "risk": 0}
	sum, openWO := 0, 0
	scanned := time.Time{}
	for _, r := range rows {
		grades[r.Grade]++
		sum += r.Score
		if r.OpenWorkOrderID > 0 {
			openWO++
		}
		if r.ComputedAt.After(scanned) {
			scanned = r.ComputedAt
		}
	}
	avg := 0
	if len(rows) > 0 {
		avg = int(math.Round(float64(sum) / float64(len(rows))))
	}
	// TOP 风险资产（带可读信息）
	type riskRow struct {
		AssetID  uint   `json:"asset_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
		Category string `json:"category"`
		SiteID   uint   `json:"site_id"`
		Score    int    `json:"score"`
		Grade    string `json:"grade"`
	}
	var risks []riskRow
	rq := h.DB.Raw(`SELECT ah.asset_id, COALESCE(a.code,'') as code, a.name, COALESCE(a.category,'other') as category,
		ah.site_id, ah.score, ah.grade FROM t_asset_health ah JOIN t_asset a ON a.id = ah.asset_id`)
	rq.Scan(&risks)
	// 手动排序取前5（按分数升序）
	for i := 0; i < len(risks); i++ {
		for j := i + 1; j < len(risks); j++ {
			if risks[j].Score < risks[i].Score {
				risks[i], risks[j] = risks[j], risks[i]
			}
		}
		if i >= 4 {
			break
		}
	}
	if len(risks) > 5 {
		risks = risks[:5]
	}
	thA, thB, thC := h.healthThresholds()
	response.Success(c, gin.H{
		"total": len(rows), "avg_score": avg, "grades": grades,
		"open_work_orders": openWO, "top_risks": risks, "scanned_at": scanned,
		"thresholds": gin.H{"healthy": thA, "attention": thB, "sub_healthy": thC},
		"auto_work_order": h.healthSetting("health_auto_work_order", "1") == "1",
	})
}

// ListAssetHealth GET /asset-health
func (h *Handler) ListAssetHealth(c *gin.Context) {
	page, size := h.paging(c)
	where := []string{}
	args := []interface{}{}
	if scope := middleware.GetSiteScope(c); scope != nil {
		where = append(where, "ah.site_id IN ?")
		args = append(args, scope)
	}
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		where = append(where, "ah.site_id = ?")
		args = append(args, sid)
	}
	if g := c.Query("grade"); g != "" {
		where = append(where, "ah.grade = ?")
		args = append(args, g)
	}
	if cat := c.Query("category"); cat != "" {
		where = append(where, "a.category = ?")
		args = append(args, cat)
	}
	if kw := c.Query("keyword"); kw != "" {
		where = append(where, "(a.name LIKE ? OR a.code LIKE ?)")
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}
	type item struct {
		AssetID   uint      `json:"asset_id"`
		Code      string    `json:"code"`
		Name      string    `json:"name"`
		Category  string    `json:"category"`
		SiteID    uint      `json:"site_id"`
		Score     int       `json:"score"`
		Grade     string    `json:"grade"`
		PointCnt  int       `json:"point_count"`
		OpenWo    uint      `json:"open_work_order_id"`
		ComputedAt time.Time `json:"computed_at"`
	}
	var total int64
	h.DB.Raw("SELECT COUNT(*) FROM t_asset_health ah JOIN t_asset a ON a.id=ah.asset_id "+wsql, args...).Scan(&total)
	var items []item
	h.DB.Raw(`SELECT ah.asset_id, COALESCE(a.code,'') as code, a.name, COALESCE(a.category,'other') as category,
		ah.site_id, ah.score, ah.grade, ah.point_count as point_cnt, ah.open_work_order_id as open_wo, ah.computed_at
		FROM t_asset_health ah JOIN t_asset a ON a.id=ah.asset_id `+wsql+
		` ORDER BY ah.score ASC, ah.asset_id DESC LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...).Scan(&items)
	if items == nil {
		items = []item{}
	}
	response.Paginated(c, items, total, page, size)
}

// GetAssetHealthDetail GET /asset-health/:asset_id
func (h *Handler) GetAssetHealthDetail(c *gin.Context) {
	assetID, _ := strconv.Atoi(c.Param("asset_id"))
	var a model.Asset
	if err := h.DB.First(&a, assetID).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	var rec model.AssetHealth
	err := h.DB.Where("asset_id = ?", assetID).First(&rec).Error
	factors := []healthFactor{}
	if err == nil && rec.Factors != "" {
		json.Unmarshal([]byte(rec.Factors), &factors)
	}
	// 90天历史
	var hist []model.AssetHealthHistory
	h.DB.Where("asset_id = ? AND day >= ?", assetID, time.Now().AddDate(0, 0, -90).Format("2006-01-02")).
		Order("day").Find(&hist)
	if hist == nil {
		hist = []model.AssetHealthHistory{}
	}
	// 绑定测点 + 近期告警
	chainPts := map[uint][]uint{}
	{
		type cpRow struct {
			AssetID uint `json:"asset_id"`
			PointID uint `json:"point_id"`
		}
		var rows []cpRow
		h.DB.Raw(`SELECT asset_id, point_id FROM t_power_chain WHERE asset_id = ? AND point_id > 0`, assetID).Scan(&rows)
		for _, r := range rows {
			chainPts[r.AssetID] = append(chainPts[r.AssetID], r.PointID)
		}
	}
	var allPts []model.MonitoringPoint
	h.DB.Where("site_id = ?", a.SiteID).Find(&allPts)
	pts := h.healthPointsForAsset(&a, chainPts, allPts)
	type ptLite struct {
		ID     uint    `json:"id"`
		Code   string  `json:"code"`
		Name   string  `json:"name"`
		Type   string  `json:"type"`
		Status int     `json:"status"`
		Value  *float64 `json:"current_value"`
		Unit   string  `json:"unit"`
	}
	ptLites := make([]ptLite, 0, len(pts))
	ptIDs := make([]uint, 0, len(pts))
	for _, p := range pts {
		ptLites = append(ptLites, ptLite{p.ID, p.Code, p.Name, p.Type, p.Status, p.CurrentValue, p.Unit})
		ptIDs = append(ptIDs, p.ID)
	}
	type alertLite struct {
		ID       uint      `json:"id"`
		Severity int       `json:"severity"`
		Message  string    `json:"message"`
		Status   int       `json:"status"`
		PointNm  string    `json:"point_name"`
		At       time.Time `json:"triggered_at"`
	}
	alerts := []alertLite{}
	if len(ptIDs) > 0 {
		h.DB.Raw(`SELECT al.id, al.severity, al.message, al.status, COALESCE(mp.name,'') as point_nm, al.triggered_at as at
			FROM t_alert al LEFT JOIN t_monitoring_point mp ON mp.id = al.point_id
			WHERE al.point_id IN ? ORDER BY al.triggered_at DESC LIMIT 10`, ptIDs).Scan(&alerts)
	}
	// 维保与工单
	type maintLite struct {
		Kind    string    `json:"kind"`
		Vendor  string    `json:"vendor"`
		EndAt   time.Time `json:"end_at"`
		Status  int       `json:"status"`
	}
	maints := []maintLite{}
	h.DB.Raw(`SELECT kind, COALESCE(vendor,'') as vendor, end_at, status FROM t_maintenance
		WHERE asset_id = ? AND kind='contract' ORDER BY end_at DESC LIMIT 3`, assetID).Scan(&maints)
	var openWO *model.WorkOrder
	if rec.OpenWorkOrderID > 0 {
		var wo model.WorkOrder
		if err := h.DB.First(&wo, rec.OpenWorkOrderID).Error; err == nil && wo.Status <= 2 {
			openWO = &wo
		}
	}
	thA, thB, thC := h.healthThresholds()
	out := gin.H{
		"asset":      gin.H{"id": a.ID, "code": a.Code, "name": a.Name, "category": a.Category, "site_id": a.SiteID, "status": a.Status, "purchase_at": a.PurchaseAt},
		"points":     ptLites,
		"alerts":     alerts,
		"maintenances": maints,
		"history":    hist,
		"thresholds": gin.H{"healthy": thA, "attention": thB, "sub_healthy": thC},
		"open_work_order": openWO,
	}
	if err == nil {
		out["current"] = gin.H{
			"score": rec.Score, "grade": rec.Grade, "grade_label": healthGradeLabel(rec.Grade),
			"factors": factors, "point_count": rec.PointCount, "computed_at": rec.ComputedAt,
		}
	} else {
		out["current"] = nil
	}
	response.Success(c, out)
}

// EvaluateAssetHealth POST /asset-health/evaluate 手动触发全量评估（管理员）
func (h *Handler) EvaluateAssetHealth(c *gin.Context) {
	stats := h.ScanAssetHealth()
	go middleware.WriteAudit(h.DB, c, "evaluate", "asset_health", fmt.Sprintf("手动健康评估: evaluated=%v risk=%v notified=%v work_orders=%v",
		stats["evaluated"], stats["risk"], stats["notified"], stats["work_orders"]))
	response.Success(c, stats)
}

// CreateHealthWorkOrder POST /asset-health/:asset_id/work-order 手动创建预测性维护工单
func (h *Handler) CreateHealthWorkOrderAPI(c *gin.Context) {
	assetID, _ := strconv.Atoi(c.Param("asset_id"))
	var a model.Asset
	if err := h.DB.First(&a, assetID).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	var rec model.AssetHealth
	if err := h.DB.Where("asset_id = ?", assetID).First(&rec).Error; err != nil {
		response.BadRequest(c, "该资产尚无健康评估记录，请先执行评估")
		return
	}
	factors := []healthFactor{}
	json.Unmarshal([]byte(rec.Factors), &factors)
	woID := h.createHealthWorkOrder(&a, rec.Score, rec.Grade, factors, false)
	if woID == 0 {
		response.BadRequest(c, "该资产已有未闭环的预测性维护工单，无需重复创建")
		return
	}
	h.DB.Model(&model.AssetHealth{}).Where("id = ?", rec.ID).Update("open_work_order_id", woID)
	var wo model.WorkOrder
	h.DB.First(&wo, woID)
	response.Created(c, gin.H{"work_order_id": woID, "code": wo.Code})
}

// GetAssetHealthConfig GET /asset-health/config 健康评估参数（含说明，写入走 PUT /system-setting）
func (h *Handler) GetAssetHealthConfig(c *gin.Context) {
	thA, thB, thC := h.healthThresholds()
	response.Success(c, gin.H{
		"health_grade_thresholds": h.healthSetting("health_grade_thresholds", "85,70,55"),
		"health_auto_work_order":  h.healthSetting("health_auto_work_order", "1"),
		"labels": []gin.H{
			{"key": "health_grade_thresholds", "label": "等级阈值", "desc": fmt.Sprintf("健康≥%d / 关注≥%d / 亚健康≥%d / 风险<%d，逗号3值", thA, thB, thC, thC)},
			{"key": "health_auto_work_order", "label": "风险级自动开工单", "desc": "1=健康分跌入风险级时自动创建预测性维护工单，0=仅通知"},
		},
		"factors": []gin.H{
			{"key": "active_alert", "label": "活跃告警", "desc": "绑定测点当前未恢复告警，按严重度扣分（上限20）"},
			{"key": "alarm_30d", "label": "近30天告警史", "desc": "历史告警频次反映设备稳定性（上限30）"},
			{"key": "threshold", "label": "阈值裕度", "desc": "当前值逼近/超过阈值上限"},
			{"key": "trend", "label": "劣化趋势", "desc": "7天日均值回归，14天内预计越限（r²≥0.3 才采信）"},
			{"key": "offline", "label": "监测数据缺失", "desc": "绑定测点离线比例"},
			{"key": "age", "label": "服役年限", "desc": "对照分类参考寿命（服务器6年/配电10年/制冷12年等）"},
			{"key": "warranty", "label": "维保覆盖", "desc": "有效维保合同状态"},
			{"key": "repair_status", "label": "维修中", "desc": "资产处于维修状态"},
		},
	})
}

// ============ 站点权限 ============
// （GetSiteScope 返回 nil=不限制，与 ListAssets 同语义）
