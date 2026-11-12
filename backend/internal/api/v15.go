package api

import (
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V15 监控系统实用化 ====================

// MonitoringSummaryRich V15 富化监控摘要（告警趋势 + 告警中测点 + 在线率）
func (h *Handler) MonitoringSummaryRich(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	type typeStat struct {
		Total     int64   `json:"total"`
		Normal    int64   `json:"normal"`
		Alert     int64   `json:"alert"`
		Offline   int64   `json:"offline"`
		OnlinePct float64 `json:"online_pct"`
		AlertPct  float64 `json:"alert_pct"`
	}
	typeTrend := func(typ string) typeStat {
		q := h.DB.Model(&model.MonitoringPoint{}).Where("type = ?", typ)
		if role != "global_admin" {
			siteIDs := h.userSiteIDs(userID)
			if len(siteIDs) > 0 {
				q = q.Where("site_id IN ?", siteIDs)
			}
		}
		var total, normal, alert, offline int64
		q.Count(&total)
		if total > 0 {
			q.Where("status = 1").Count(&normal)
			q.Where("status = 2").Count(&alert)
			q.Where("status = 0").Count(&offline)
		}
		s := typeStat{Total: total, Normal: normal, Alert: alert, Offline: offline}
		if total > 0 {
			s.OnlinePct = float64(normal+alert) / float64(total) * 100
			s.AlertPct = float64(alert) / float64(total) * 100
		}
		return s
	}

	// 六大类统计
	types := []string{"power_dist", "ups", "hvac", "temp_humidity", "water_leak", "smoke", "cdu", "leak_rope"}
	summary := gin.H{}
	for _, t := range types {
		summary[t] = typeTrend(t)
	}

	// 24h 告警趋势（按类型×小时桶）
	trends := make(map[string][]int64)
	var rows []struct {
		Hour  string
		PtT   string `gorm:"column:pt_type"`
		Count int64
	}
	h.DB.Model(&model.Alert{}).
		Select(`to_char(date_trunc('hour', triggered_at), 'YYYY-MM-DD HH24') as hour, COALESCE(p.type,'') as pt_type, COUNT(*) as count`).
		Joins("LEFT JOIN t_monitoring_point p ON p.id = t_alert.point_id").
		Where("t_alert.triggered_at > ?", cutoff).
		Group("hour, pt_type").Order("hour").Scan(&rows)

	for _, t := range types {
		trends[t] = make([]int64, 24)
	}
	for _, r := range rows {
		t, _ := time.Parse("2006-01-02 15", r.Hour)
		slot := int(now.Sub(t).Hours())
		if slot >= 0 && slot < 24 && trends[r.PtT] != nil {
			trends[r.PtT][23-slot] = r.Count
		}
	}
	summary["trends"] = trends

	// 告警中测点 Top 20（按活跃告警数降序）
	var alertPtRows []struct {
		ID           uint     `json:"id"`
		Name         string   `json:"name"`
		Code         string   `json:"code"`
		Type         string   `json:"type"`
		CurrentValue *float64 `json:"current_value"`
		Unit         string   `json:"unit"`
		MinThreshold  *float64 `json:"min_threshold"`
		MaxThreshold  *float64 `json:"max_threshold"`
		HighThreshold *float64 `json:"high_threshold"`
		Status       int      `json:"status"`
		AlertCount   int64    `json:"alert_count"`
	}
	h.DB.Model(&model.MonitoringPoint{}).
		Select(`t_monitoring_point.id, name, code, type, current_value, unit, min_threshold, max_threshold, status,
		        (SELECT COUNT(*) FROM t_alert WHERE point_id = t_monitoring_point.id AND status = 0) as alert_count`).
		Where("status = 2").
		Order("alert_count DESC").Limit(20).Scan(&alertPtRows)
	if alertPtRows == nil {
		alertPtRows = []struct {
			ID            uint     `json:"id"`
			Name          string   `json:"name"`
			Code          string   `json:"code"`
			Type          string   `json:"type"`
			CurrentValue  *float64 `json:"current_value"`
			Unit          string   `json:"unit"`
			MinThreshold  *float64 `json:"min_threshold"`
			MaxThreshold  *float64 `json:"max_threshold"`
			HighThreshold *float64 `json:"high_threshold"`
			Status        int      `json:"status"`
			AlertCount    int64    `json:"alert_count"`
		}{}
	}
	summary["alarming_points"] = alertPtRows

	// 活跃告警总数
	var activeAlerts int64
	h.DB.Model(&model.Alert{}).Where("status = 0").Count(&activeAlerts)
	summary["active_alerts"] = activeAlerts

	response.Success(c, summary)
}

// BatchMonitoringPoints V15 批量启停/删除测点
func (h *Handler) BatchMonitoringPoints(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Action string `json:"action"` // enable/disable/delete
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 200 {
		response.Error(c, 400, "参数错误: ids 不能为空且最多 200")
		return
	}
	if req.Action != "enable" && req.Action != "disable" && req.Action != "delete" {
		response.Error(c, 400, "action 须为 enable/disable/delete 之一")
		return
	}
	switch req.Action {
	case "enable":
		h.DB.Model(&model.MonitoringPoint{}).Where("id IN ?", req.IDs).Update("status", 1)
	case "disable":
		h.DB.Model(&model.MonitoringPoint{}).Where("id IN ?", req.IDs).Update("status", 0)
	case "delete":
		h.DB.Where("point_id IN ?", req.IDs).Delete(&model.Reading{})
		h.DB.Where("point_id IN ?", req.IDs).Delete(&model.Alert{})
		h.DB.Where("id IN ?", req.IDs).Delete(&model.AlertRule{})
		h.DB.Where("id IN ?", req.IDs).Delete(&model.MonitoringPoint{})
	}
	response.Success(c, gin.H{"affected": len(req.IDs)})
}

// BatchAlerts V15 批量确认/结束告警
func (h *Handler) BatchAlerts(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Action string `json:"action"` // acknowledge/resolve
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 200 {
		response.Error(c, 400, "参数错误: ids 不能为空且最多 200")
		return
	}
	if req.Action != "acknowledge" && req.Action != "resolve" {
		response.Error(c, 400, "action 须为 acknowledge/resolve 之一")
		return
	}
	now := time.Now()
	uid := c.GetUint("user_id")
	for _, id := range req.IDs {
		var a model.Alert
		if h.DB.First(&a, id).Error != nil {
			continue
		}
		switch req.Action {
		case "acknowledge":
			if a.Status == 0 || a.Status == 1 {
				h.DB.Model(&a).Updates(map[string]interface{}{
					"status": 2, "acknowledged_by": uid, "acknowledged_at": now,
				})
				h.DB.Create(&model.AlertActionLog{AlertID: a.ID, Action: "acknowledge", Reason: req.Reason, Operator: uid})
			}
		case "resolve":
			if a.Status < 3 {
				h.DB.Model(&a).Updates(map[string]interface{}{"status": 3, "resolved_at": now})
				h.DB.Create(&model.AlertActionLog{AlertID: a.ID, Action: "resolve", Reason: req.Reason, Operator: uid})
				h.restorePointStatus(int(a.ID))
			}
		}
	}
	response.Success(c, gin.H{"processed": len(req.IDs)})
}

// TrendSeriesCompare V15 多测点对比分析（基准测点 + 偏差/百分比）
func (h *Handler) TrendSeriesCompare(c *gin.Context) {
	baseID := c.Query("base_point_id")
	ids := splitCSV(c.Query("point_ids"))
	if len(ids) == 0 {
		ids = c.QueryArray("point_id")
	}
	if baseID == "" || len(ids) == 0 {
		response.Error(c, 400, "须传 base_point_id 和 point_ids")
		return
	}
	allIDs := append([]string{baseID}, ids...)
	// 复用既有降采样逻辑，先取基准序列
	startStr := c.DefaultQuery("start", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	var basePts []model.Reading
	h.DB.Where("point_id = ? AND timestamp BETWEEN ? AND ?", baseID, startStr, endStr).
		Order("timestamp").Limit(3000).Find(&basePts)

	type cmpPoint struct {
		Time    string  `json:"time"`
		BaseVal float64 `json:"base_val"`
		Diff    float64 `json:"diff"`
		Pct     float64 `json:"pct"`
	}
	comparisons := make([]cmpPoint, 0, len(basePts))
	for _, b := range basePts {
		var targetVals []struct {
			PointID uint
			Val     float64
		}
		h.DB.Model(&model.Reading{}).
			Select("point_id, value as val").
			Where("point_id IN ? AND timestamp = ?", ids, b.Timestamp).Limit(len(ids)).Scan(&targetVals)
		if len(targetVals) > 0 {
			avgTarget := 0.0
			for _, tv := range targetVals {
				avgTarget += tv.Val
			}
			avgTarget /= float64(len(targetVals))
			c := cmpPoint{
				Time:    b.Timestamp.Format("2006-01-02T15:04:05"),
				BaseVal: b.Value,
				Diff:    avgTarget - b.Value,
			}
			if b.Value != 0 {
				c.Pct = (avgTarget - b.Value) / b.Value * 100
			}
			comparisons = append(comparisons, c)
		}
	}
	if comparisons == nil {
		comparisons = []cmpPoint{}
	}

	// 阈值标注：基准测点的阈值线
	var pt model.MonitoringPoint
	maxLine, minLine := (*float64)(nil), (*float64)(nil)
	if h.DB.First(&pt, baseID).Error == nil {
		if pt.MaxThreshold != nil {
			v := *pt.MaxThreshold
			maxLine = &v
		}
		if pt.MinThreshold != nil {
			v := *pt.MinThreshold
			minLine = &v
		}
	}

	var points []model.Reading
	h.DB.Where("point_id IN ? AND timestamp BETWEEN ? AND ?", allIDs, startStr, endStr).
		Order("timestamp").Limit(5000).Find(&points)
	if points == nil {
		points = []model.Reading{}
	}

	response.Success(c, gin.H{
		"comparisons":      comparisons,
		"points":           points,
		"base_max":         maxLine,
		"base_min":         minLine,
		"base_unit":        pt.Unit,
		"point_count":      len(points),
		"comparison_count": len(comparisons),
	})
}

// TrendThresholds V15 测点阈值线与越限时间点
func (h *Handler) TrendThresholds(c *gin.Context) {
	// 兼容两种传参: point_ids=1,2,3 (逗号串, axios 默认) 或 point_id=1&point_id=2
	ids := splitCSV(c.Query("point_ids"))
	if len(ids) == 0 {
		ids = c.QueryArray("point_id")
	}
	if len(ids) == 0 {
		response.Error(c, 400, "须传 point_ids")
		return
	}
	type ptThresh struct {
		ID            uint     `json:"id"`
		Name          string   `json:"name"`
		Unit          string   `json:"unit"`
		MaxThreshold  *float64 `json:"max_threshold"`
		MinThreshold  *float64 `json:"min_threshold"`
		HighThreshold *float64 `json:"high_threshold"` // V21-④ 过高线
	}
	var pts []model.MonitoringPoint
	h.DB.Where("id IN ?", ids).Find(&pts)
	thresholds := make([]ptThresh, 0, len(pts))
	for _, p := range pts {
		thresholds = append(thresholds, ptThresh{p.ID, p.Name, p.Unit, p.MaxThreshold, p.MinThreshold, p.HighThreshold})
	}
	if thresholds == nil {
		thresholds = []ptThresh{}
	}

	// 最近 7 天的越限事件（读数 > max 或 < min 的时间点）
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var violations []struct {
		PointID   uint      `json:"point_id"`
		Timestamp time.Time `json:"timestamp"`
		Value     float64   `json:"value"`
		Direction string    `json:"direction"` // high/low
	}
	var allReadings []model.Reading
	h.DB.Where("point_id IN ? AND timestamp > ?", ids, cutoff).Order("timestamp").Limit(50000).Find(&allReadings)
	pmap := map[uint]*model.MonitoringPoint{}
	for i := range pts {
		pmap[pts[i].ID] = &pts[i]
	}
	for _, r := range allReadings {
		p := pmap[r.PointID]
		if p == nil {
			continue
		}
		if p.MaxThreshold != nil && r.Value > *p.MaxThreshold {
			violations = append(violations, struct {
				PointID   uint      `json:"point_id"`
				Timestamp time.Time `json:"timestamp"`
				Value     float64   `json:"value"`
				Direction string    `json:"direction"`
			}{r.PointID, r.Timestamp, r.Value, "high"})
		}
		if p.MinThreshold != nil && r.Value < *p.MinThreshold {
			violations = append(violations, struct {
				PointID   uint      `json:"point_id"`
				Timestamp time.Time `json:"timestamp"`
				Value     float64   `json:"value"`
				Direction string    `json:"direction"`
			}{r.PointID, r.Timestamp, r.Value, "low"})
		}
	}
	if violations == nil {
		violations = []struct {
			PointID   uint      `json:"point_id"`
			Timestamp time.Time `json:"timestamp"`
			Value     float64   `json:"value"`
			Direction string    `json:"direction"`
		}{}
	}
	// 降采样: 动态步长确保结果 <= 1000
	if len(violations) > 1000 {
		step := len(violations)/1000 + 1
		down := make([]struct {
			PointID   uint      `json:"point_id"`
			Timestamp time.Time `json:"timestamp"`
			Value     float64   `json:"value"`
			Direction string    `json:"direction"`
		}, 0, len(violations)/step+1)
		for i, v := range violations {
			if i%step == 0 {
				down = append(down, v)
			}
		}
		violations = down
	}

	response.Success(c, gin.H{
		"thresholds":         thresholds,
		"violations":         violations,
		"violation_count":    len(violations),
		"window_days":        7,
	})
}

// TrendDownsample V15 LTTB 降采样（长区间优化，减少前端数据量）
func (h *Handler) TrendDownsample(c *gin.Context) {
	ids := splitCSV(c.Query("point_ids"))
	if len(ids) == 0 {
		ids = c.QueryArray("point_id")
	}
	if len(ids) == 0 {
		response.Error(c, 400, "须传 point_ids")
		return
	}
	startStr := c.DefaultQuery("start", time.Now().Add(-30*24*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))
	maxStr := c.DefaultQuery("max_points", "500")
	maxPoints, _ := strconv.Atoi(maxStr)
	if maxPoints <= 0 || maxPoints > 10000 {
		maxPoints = 500
	}

	var readings []model.Reading
	h.DB.Where("point_id IN ? AND timestamp BETWEEN ? AND ?", ids, startStr, endStr).
		Order("timestamp").Find(&readings)

	if len(readings) > maxPoints {
		downsampled := lttb(readings, maxPoints)
		readings = downsampled
	}
	if readings == nil {
		readings = []model.Reading{}
	}
	response.Success(c, gin.H{"items": readings, "total": len(readings), "max_points": maxPoints})
}

// LTTB 降采样: 保留极值的大视觉保真算法
func lttb(data []model.Reading, target int) []model.Reading {
	if len(data) <= target {
		return data
	}
	result := make([]model.Reading, target)
	result[0] = data[0]
	result[target-1] = data[len(data)-1]
	bucketSize := float64(len(data)-2) / float64(target-2)

	for i := 1; i < target-1; i++ {
		start := int(float64(i-1)*bucketSize) + 1
		end := int(float64(i)*bucketSize) + 1
		if end > len(data)-1 {
			end = len(data) - 1
		}
		if start >= end {
			result[i] = data[start]
			continue
		}
		prev := result[i-1]
		next := data[end]
		prevX, prevY := float64(prev.Timestamp.UnixMilli()), prev.Value
		nextX, nextY := float64(next.Timestamp.UnixMilli()), next.Value
		denom := nextX - prevX
		if denom == 0 {
			denom = 1
		}
		maxArea := -1.0
		bestIdx := start
		for j := start; j < end; j++ {
			x := float64(data[j].Timestamp.UnixMilli())
			area := abs((nextX-prevX)*(data[j].Value-prevY) - (nextY-prevY)*(x-prevX))
			if area > maxArea {
				maxArea = area
				bestIdx = j
			}
		}
		result[i] = data[bestIdx]
	}
	return result
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// PointsLive V15 测点实时值批量查询（供列表轮询刷新）
func (h *Handler) PointsLive(c *gin.Context) {
	idsStr := c.Query("ids")
	if idsStr == "" {
		response.Error(c, 400, "须传 ids(逗号分隔)")
		return
	}
	parts := splitCSV(idsStr)
	ids := make([]uint, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			ids = append(ids, uint(n))
		}
	}
	if len(ids) == 0 {
		response.Error(c, 400, "ids 无有效数字")
		return
	}
	var pts []model.MonitoringPoint
	h.DB.Select("id", "current_value", "status", "unit").Where("id IN ?", ids).Find(&pts)
	if pts == nil {
		pts = []model.MonitoringPoint{}
	}
	type livePoint struct {
		ID           uint     `json:"id"`
		CurrentValue *float64 `json:"current_value"`
		Status       int      `json:"status"`
		Unit         string   `json:"unit"`
	}
	out := make([]livePoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, livePoint{p.ID, p.CurrentValue, p.Status, p.Unit})
	}
	response.Success(c, gin.H{"items": out, "total": len(out)})
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}