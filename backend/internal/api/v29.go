package api

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V29-① 备件台账 ====================

// ListSpareParts GET /spare-parts?site_id=&category=&keyword=&low_only=1
func (h *Handler) ListSpareParts(c *gin.Context) {
	q := h.DB.Model(&model.SparePart{})
	if sid, _ := strconv.Atoi(c.Query("site_id")); sid > 0 {
		q = q.Where("site_id = ?", sid)
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("code ILIKE ? OR name ILIKE ? OR spec ILIKE ?", like, like, like)
	}
	if c.Query("low_only") == "1" {
		q = q.Where("stock <= safety_stock")
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size <= 0 || size > 200 {
		size = 20
	}
	var total int64
	q.Count(&total)
	var list []model.SparePart
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	response.Paginated(c, list, total, page, size)
}

// CreateSparePart POST /spare-parts
func (h *Handler) CreateSparePart(c *gin.Context) {
	var sp model.SparePart
	if err := c.ShouldBindJSON(&sp); err != nil || sp.Name == "" {
		response.BadRequest(c, "name 必填")
		return
	}
	if sp.SiteID == 0 {
		sp.SiteID = 1
	}
	if !middleware.InSiteScope(c, sp.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if sp.Code == "" {
		sp.Code = fmt.Sprintf("SP%04d", time.Now().UnixMilli()%10000)
	}
	if sp.Unit == "" {
		sp.Unit = "个"
	}
	if err := h.DB.Create(&sp).Error; err != nil {
		response.BadRequest(c, "创建失败(编号重复?)")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_spare", sp.Code, sp.Name)
	// 初始库存入库流水
	if sp.Stock > 0 {
		h.DB.Create(&model.SpareMovement{
			SpareID: sp.ID, Type: "in", Qty: sp.Stock, StockAfter: sp.Stock,
			OperatorID: uint(middleware.GetUserID(c)), Operator: usernameOf(c), Reason: "初始入库",
		})
	}
	response.Created(c, sp)
}

// UpdateSparePart PUT /spare-parts/:id (台账字段, 不含库存——库存走出入库)
func (h *Handler) UpdateSparePart(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, id).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Category    string  `json:"category"`
		Spec        string  `json:"spec"`
		Unit        string  `json:"unit"`
		SafetyStock int     `json:"safety_stock"`
		Location    string  `json:"location"`
		Price       float64 `json:"price"`
		Supplier    string  `json:"supplier"`
		Remark      string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	if req.Name != "" {
		sp.Name = req.Name
	}
	sp.Category, sp.Spec, sp.Unit = req.Category, req.Spec, req.Unit
	sp.SafetyStock = req.SafetyStock
	sp.Location, sp.Supplier, sp.Remark = req.Location, req.Supplier, req.Remark
	if req.Price > 0 {
		sp.Price = req.Price
	}
	h.DB.Save(&sp)
	middleware.WriteAudit(h.DB, c, "update_spare", sp.Code, sp.Name)
	response.Success(c, sp)
}

// DeleteSparePart DELETE /spare-parts/:id
func (h *Handler) DeleteSparePart(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, id).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	var mvCount int64
	h.DB.Model(&model.SpareMovement{}).Where("spare_id = ?", id).Count(&mvCount)
	if mvCount > 0 {
		response.BadRequest(c, "该备件已有出入库流水，不可删除(可停用)")
		return
	}
	h.DB.Delete(&sp)
	response.Success(c, gin.H{"deleted": id})
}

// SpareMovements GET /spare-parts/:id/movements 出入库流水
func (h *Handler) SpareMovements(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var list []model.SpareMovement
	h.DB.Where("spare_id = ?", id).Order("id DESC").Limit(200).Find(&list)
	response.Success(c, gin.H{"list": list})
}

// MoveSpare POST /spare-parts/:id/move {type: in/out, qty, reason, work_order_id}
// 出入库统一入口: 出库校验库存充足; 变动后检查低库存→通知+Webhook
func (h *Handler) MoveSpare(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, id).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	var req struct {
		Type        string `json:"type"` // in/out
		Qty         int    `json:"qty"`
		Reason      string `json:"reason"`
		WorkOrderID uint   `json:"work_order_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Type != "in" && req.Type != "out") || req.Qty <= 0 {
		response.BadRequest(c, "type=in/out 且 qty>0")
		return
	}
	if req.Type == "out" && req.Qty > sp.Stock {
		response.BadRequest(c, fmt.Sprintf("库存不足: 现有 %d%s, 需 %d", sp.Stock, sp.Unit, req.Qty))
		return
	}
	if req.Type == "out" {
		sp.Stock -= req.Qty
	} else {
		sp.Stock += req.Qty
	}
	h.DB.Save(&sp)
	mv := model.SpareMovement{
		SpareID: sp.ID, Type: req.Type, Qty: req.Qty, StockAfter: sp.Stock,
		WorkOrderID: req.WorkOrderID,
		OperatorID:  uint(middleware.GetUserID(c)), Operator: usernameOf(c),
		Reason: req.Reason,
	}
	h.DB.Create(&mv)
	middleware.WriteAudit(h.DB, c, "spare_"+req.Type, sp.Code, fmt.Sprintf("%s %d%s (%s)", req.Type, req.Qty, sp.Unit, req.Reason))
	// 工单联动: 出库且带工单 → 写工单流转日志
	if req.Type == "out" && req.WorkOrderID > 0 {
		var wo model.WorkOrder
		if h.DB.First(&wo, req.WorkOrderID).Error == nil {
			h.DB.Create(&model.WorkOrderLog{
				OrderID: wo.ID, UserID: uint(middleware.GetUserID(c)), Username: usernameOf(c),
				Action:  "spare_out",
				Content: fmt.Sprintf("领用备件 %s(%s) ×%d%s，库存余 %d", sp.Name, sp.Code, req.Qty, sp.Unit, sp.Stock),
			})
		}
	}
	h.checkSpareLow(&sp)
	response.Success(c, gin.H{"movement": mv, "stock": sp.Stock})
}

// checkSpareLow 低库存检查: stock<=safety 且未通知过 → 站内通知+Webhook; 恢复则复位标记
func (h *Handler) checkSpareLow(sp *model.SparePart) {
	if sp.SafetyStock <= 0 {
		return
	}
	if sp.Stock <= sp.SafetyStock {
		if sp.LowWarned {
			return
		}
		msg := fmt.Sprintf("备件 %s(%s) 库存 %d%s ≤ 安全水位 %d，请及时补货。存放: %s",
			sp.Name, sp.Code, sp.Stock, sp.Unit, sp.SafetyStock, sp.Location)
		h.DB.Create(&model.Notification{
			Kind: "spare_low", Title: "备件低库存: " + sp.Name, Content: msg,
			RefType: "spare_part", RefID: sp.ID,
		})
		h.DispatchWebhook(model.EvMaintenanceDue, "[备件低库存] "+sp.Name, msg, map[string]interface{}{
			"spare_id": sp.ID, "spare_code": sp.Code, "stock": sp.Stock, "safety_stock": sp.SafetyStock,
		})
		h.DB.Model(sp).Update("low_warned", true)
	} else if sp.LowWarned {
		h.DB.Model(sp).Update("low_warned", false)
	}
}

// SpareStats GET /spare-parts/stats 台账概览
func (h *Handler) SpareStats(c *gin.Context) {
	type row struct {
		N        int64   `json:"n"`
		Low      int64   `json:"low"`
		Value    float64 `json:"value"`
		Category string  `json:"category"`
	}
	var total row
	h.DB.Raw(`SELECT COUNT(*) n,
		COALESCE(SUM(CASE WHEN stock<=safety_stock AND safety_stock>0 THEN 1 ELSE 0 END),0) low,
		COALESCE(SUM(stock*price),0) value FROM t_spare_part`).Scan(&total)
	var byCat []row
	h.DB.Raw(`SELECT COALESCE(category,'其他') category, COUNT(*) n,
		COALESCE(SUM(stock*price),0) value FROM t_spare_part GROUP BY category ORDER BY n DESC`).Scan(&byCat)
	// 近30天出库TOP5(消耗最快, 补货参考)
	type top struct {
		SpareID uint   `json:"spare_id"`
		Name    string `json:"name"`
		Qty     int64  `json:"qty"`
	}
	var topOut []top
	h.DB.Raw(`SELECT m.spare_id, s.name, SUM(m.qty) qty FROM t_spare_movement m
		JOIN t_spare_part s ON s.id=m.spare_id
		WHERE m.type='out' AND m.created_at > NOW() - INTERVAL '30 days'
		GROUP BY m.spare_id, s.name ORDER BY qty DESC LIMIT 5`).Scan(&topOut)
	response.Success(c, gin.H{
		"total_kinds": total.N, "low_stock": total.Low,
		"stock_value": math.Round(total.Value*100) / 100,
		"by_category": byCat, "top_out_30d": topOut,
	})
}

// ==================== V29-③ 趋势预测(线性回归) ====================

// linearRegression 最小二乘: x=小时偏移, y=读数; 返回 slope/intercept/r2
func linearRegression(xs, ys []float64) (slope, intercept, r2 float64) {
	n := float64(len(xs))
	if n < 2 {
		return 0, 0, 0
	}
	var sx, sy, sxy, sxx, syy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxy += xs[i] * ys[i]
		sxx += xs[i] * xs[i]
		syy += ys[i] * ys[i]
	}
	denom := n*sxx - sx*sx
	if math.Abs(denom) < 1e-12 {
		return 0, sy / n, 0
	}
	slope = (n*sxy - sx*sy) / denom
	intercept = (sy - slope*sx) / n
	// r2
	num := n*sxy - sx*sy
	d1 := n*sxx - sx*sx
	d2 := n*syy - sy*sy
	if d2 > 1e-12 {
		r2 = num * num / (d1 * d2)
	}
	return
}

// forecastPoint 单测点预测: 取近 hours 小时读数回归, 对照阈值算越限时间
func (h *Handler) forecastPoint(p *model.MonitoringPoint, hours int) *model.TrendForecast {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	var readings []model.Reading
	h.DB.Where("point_id = ? AND quality = 1 AND timestamp >= ?", p.ID, since).
		Order("timestamp").Find(&readings)
	f := &model.TrendForecast{
		PointID: p.ID, PointCode: p.Code, PointName: p.Name, Unit: p.Unit,
		HoursToLimit: -1, Samples: len(readings),
	}
	if p.CurrentValue != nil {
		f.Current = *p.CurrentValue
	}
	if len(readings) < 5 {
		f.Confidence = "low"
		return f
	}
	t0 := readings[0].Timestamp
	xs := make([]float64, 0, len(readings))
	ys := make([]float64, 0, len(readings))
	for _, r := range readings {
		xs = append(xs, r.Timestamp.Sub(t0).Hours())
		ys = append(ys, r.Value)
	}
	slope, intercept, r2 := linearRegression(xs, ys)
	f.Slope = math.Round(slope*1e6) / 1e6
	f.Intercept = intercept
	f.R2 = math.Round(r2*1000) / 1000
	// 当前值锚定预测(用最新读数而非回归线端点, 减少漂移)
	lastX := xs[len(xs)-1]
	f.Predicted24 = math.Round((ys[len(ys)-1]+slope*24)*100) / 100
	_ = lastX
	switch {
	case slope > 1e-6:
		f.Direction = "rising"
	case slope < -1e-6:
		f.Direction = "falling"
	default:
		f.Direction = "flat"
	}
	if f.Samples >= 24 {
		f.Confidence = "high"
	} else if f.Samples >= 12 {
		f.Confidence = "medium"
	} else {
		f.Confidence = "low"
	}
	if r2 < 0.3 {
		f.Confidence = "low" // 拟合差降级
	}
	// 参照阈值: rising→看上限(max/high取小者即先碰到的), falling→看下限
	cur := ys[len(ys)-1]
	if f.Direction == "rising" {
		limits := []float64{}
		if p.MaxThreshold != nil && *p.MaxThreshold > cur {
			limits = append(limits, *p.MaxThreshold)
		}
		if p.HighThreshold != nil && *p.HighThreshold > cur {
			limits = append(limits, *p.HighThreshold)
		}
		if len(limits) > 0 {
			sort.Float64s(limits)
			f.Threshold = limits[0]
			hh := (f.Threshold - cur) / slope
			if hh >= 0 && hh <= 168 { // 一周内
				f.HoursToLimit = math.Round(hh*10) / 10
				f.WillBreach = hh <= 24
			}
		}
	} else if f.Direction == "falling" && p.MinThreshold != nil && *p.MinThreshold < cur {
		f.Threshold = *p.MinThreshold
		hh := (cur - f.Threshold) / (-slope)
		if hh >= 0 && hh <= 168 {
			f.HoursToLimit = math.Round(hh*10) / 10
			f.WillBreach = hh <= 24
		}
	}
	return f
}

// TrendForecastAll GET /trend-forecast?hours=24&breach_only=1 全测点预测扫描
func (h *Handler) TrendForecastAll(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	var points []model.MonitoringPoint
	q := h.DB.Where("data_type = 'float'")
	if sid, _ := strconv.Atoi(c.Query("site_id")); sid > 0 {
		q = q.Where("site_id = ?", sid)
	}
	q.Find(&points)
	list := make([]*model.TrendForecast, 0, len(points))
	breaches := 0
	for i := range points {
		f := h.forecastPoint(&points[i], hours)
		if f.WillBreach {
			breaches++
		}
		if c.Query("breach_only") == "1" && !f.WillBreach {
			continue
		}
		if f.Direction == "flat" && c.Query("moving_only") == "1" {
			continue
		}
		list = append(list, f)
	}
	// 越限风险排前, 其余按|slope|降序
	sort.Slice(list, func(i, j int) bool {
		if list[i].WillBreach != list[j].WillBreach {
			return list[i].WillBreach
		}
		if list[i].WillBreach {
			return list[i].HoursToLimit < list[j].HoursToLimit
		}
		return math.Abs(list[i].Slope) > math.Abs(list[j].Slope)
	})
	response.Success(c, gin.H{"list": list, "breaches": breaches, "hours": hours})
}

// TrendForecastOne GET /trend-forecast/:point_id 单测点预测(测点详情用)
func (h *Handler) TrendForecastOne(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var p model.MonitoringPoint
	if h.DB.First(&p, id).Error != nil {
		response.NotFound(c, "测点不存在")
		return
	}
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	response.Success(c, h.forecastPoint(&p, hours))
}


// ScanTrendForecasts V29-③ 后台趋势预测扫描(每10分钟): 24h内将越限的测点→通知+Webhook
// 去重: 同一测点6小时内只通知一次, 防刷屏
func (h *Handler) ScanTrendForecasts() int {
	var points []model.MonitoringPoint
	h.DB.Where("data_type = 'float' AND (max_threshold IS NOT NULL OR high_threshold IS NOT NULL OR min_threshold IS NOT NULL)").Find(&points)
	notified := 0
	since := time.Now().Add(-6 * time.Hour)
	for i := range points {
		p := &points[i]
		if p.Status == 2 {
			continue // 已告警的不重复预测
		}
		f := h.forecastPoint(p, 24)
		if !f.WillBreach || f.HoursToLimit < 0 {
			continue
		}
		// V44: 低置信度(r²<0.3)预测不推送——模拟器随机游走斜率纯属偶然，会刷屏误报；趋势页仍可查
		if f.Confidence == "low" {
			continue
		}
		// 去重: 6小时内已通知过该测点则跳过
		var cnt int64
		h.DB.Model(&model.Notification{}).
			Where("kind = 'trend_forecast' AND ref_id = ? AND created_at > ?", p.ID, since).
			Count(&cnt)
		if cnt > 0 {
			continue
		}
		dirTxt := "上升"
		if f.Direction == "falling" {
			dirTxt = "下降"
		}
		msg := fmt.Sprintf("%s 当前 %.2f%s，按近24h趋势预计 %.1f 小时后%s至阈值 %.2f%s（24h后预测值 %.2f，置信度%s）。建议提前检查。",
			p.Name, f.Current, p.Unit, f.HoursToLimit, dirTxt, f.Threshold, p.Unit, f.Predicted24, confTxt(f.Confidence))
		h.DB.Create(&model.Notification{
			Kind: "trend_forecast", Title: "[趋势预警] " + p.Name, Content: msg,
			RefType: "monitoring_point", RefID: p.ID,
		})
		h.DispatchWebhook(model.EvAlarm, "[趋势预警] "+p.Name, msg, map[string]interface{}{
			"point_id": p.ID, "point_name": p.Name, "hours_to_limit": f.HoursToLimit,
			"current": f.Current, "threshold": f.Threshold, "direction": f.Direction,
		})
		notified++
	}
	return notified
}

func confTxt(c string) string {
	switch c {
	case "high":
		return "高"
	case "medium":
		return "中"
	default:
		return "低"
	}
}
