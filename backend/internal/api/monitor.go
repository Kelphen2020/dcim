package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 监控测点 CRUD ====================

// ListMonitoringPoints 测点列表（按站点+类型筛选）
func (h *Handler) ListMonitoringPoints(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	if role == "" {
		role = "viewer"
	}
	q := h.DB.Model(&model.MonitoringPoint{})
	if sid := c.Query("site_id"); sid != "" {
		if id, err := strconv.Atoi(sid); err == nil && id > 0 {
			q = q.Where("site_id = ?", id)
		}
	}
	if pid := c.Query("id"); pid != "" {
		if id, err := strconv.Atoi(pid); err == nil && id > 0 {
			q = q.Where("id = ?", id)
		}
	}
	if typ := c.Query("type"); typ != "" {
		q = q.Where("type = ?", typ)
	}
	if st := c.Query("status"); st != "" {
		if v, err := strconv.Atoi(st); err == nil {
			q = q.Where("status = ?", v)
		}
	}
	if cab := c.Query("cabinet_id"); cab != "" {
		if id, err := strconv.Atoi(cab); err == nil && id > 0 {
			q = q.Where("cabinet_id = ?", id)
		}
	}
	// V66 B2-1: 按绑定网络设备过滤（0=未绑定）
	if nd := c.Query("network_device_id"); nd != "" {
		if id, err := strconv.Atoi(nd); err == nil {
			if id > 0 {
				q = q.Where("network_device_id = ?", id)
			} else {
				q = q.Where("network_device_id IS NULL")
			}
		}
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("name ILIKE ? OR code ILIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	// 站点范围过滤
	if role != "global_admin" {
		siteIDs := h.userSiteIDs(userID)
		if len(siteIDs) == 0 {
			response.Success(c, gin.H{"items": []model.MonitoringPoint{}, "total": 0})
			return
		}
		q = q.Where("site_id IN ?", siteIDs)
	}
	var total int64
	q.Count(&total)
	page, size := pagination(c)
	var list []model.MonitoringPoint
	q.Order("site_id, type, name").Offset((page - 1) * size).Limit(size).Find(&list)
	response.Success(c, gin.H{"items": list, "total": total, "page": page, "size": size})
}

// CreateMonitoringPoint 创建测点
func (h *Handler) CreateMonitoringPoint(c *gin.Context) {
	var req model.MonitoringPoint
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if req.SiteID == 0 || req.Code == "" || req.Name == "" || req.Type == "" {
		response.Error(c, http.StatusBadRequest, "站点/编码/名称/类型不能为空")
		return
	}
	if req.Protocol == "" {
		req.Protocol = "manual"
	}
	// V66 B2-1: 创建时校验绑定设备存在
	if req.NetworkDeviceID != nil && *req.NetworkDeviceID > 0 {
		var ndc int64
		h.DB.Model(&model.NetworkDevice{}).Where("id = ?", *req.NetworkDeviceID).Count(&ndc)
		if ndc == 0 {
			response.Error(c, http.StatusBadRequest, "绑定的网络设备不存在")
			return
		}
	}
	if err := h.DB.Create(&req).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "create", "monitoring_point", fmt.Sprintf("%d %s", req.ID, req.Name))
	response.Success(c, req)
}

// UpdateMonitoringPoint 更新测点
func (h *Handler) UpdateMonitoringPoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id 无效")
		return
	}
	var existing model.MonitoringPoint
	if err := h.DB.First(&existing, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "测点不存在")
		return
	}
	// V45: 先读原始 body（ShouldBindJSON 会消费 body，之后 io.ReadAll 为空——必须先读再反序列化）
	rawBody, _ := io.ReadAll(c.Request.Body)
	var req model.MonitoringPoint
	if err := json.Unmarshal(rawBody, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	// 探测显式携带的键——局部 PUT 时未传的字段不清零，传了的（含零值）才更新
	var bodyMap map[string]interface{}
	_ = json.Unmarshal(rawBody, &bodyMap)
	has := func(k string) bool { _, ok := bodyMap[k]; return ok }
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.Unit != "" {
		updates["unit"] = req.Unit
	}
	if has("description") {
		updates["description"] = req.Description
	}
	if has("code") && req.Code != "" {
		updates["code"] = req.Code
	}
	// V45: 此前编辑保存会丢协议配置/数据类型/缩放/采集参数（updates 从未包含），逐项补上
	if has("data_type") && req.DataType != "" {
		updates["data_type"] = req.DataType
	}
	if has("scale_factor") {
		updates["scale_factor"] = req.ScaleFactor
	}
	if has("poll_interval") {
		updates["poll_interval"] = req.PollInterval
	}
	if has("alarm_delay") {
		updates["alarm_delay"] = req.AlarmDelay
	}
	if has("protocol_config") {
		updates["protocol_config"] = req.ProtocolConfig
	}
	if has("room_id") {
		updates["room_id"] = req.RoomID
	}
	if has("cabinet_id") {
		updates["cabinet_id"] = req.CabinetID
	}
	// V66 B2-1: 绑定/解绑网络设备（显式携带才更新，传 null=解绑）
	if has("network_device_id") {
		if req.NetworkDeviceID != nil && *req.NetworkDeviceID > 0 {
			var ndc int64
			h.DB.Model(&model.NetworkDevice{}).Where("id = ?", *req.NetworkDeviceID).Count(&ndc)
			if ndc == 0 {
				response.Error(c, http.StatusBadRequest, "绑定的网络设备不存在")
				return
			}
		}
		updates["network_device_id"] = req.NetworkDeviceID
	}
	if req.MinThreshold != nil {
		updates["min_threshold"] = req.MinThreshold
	}
	if req.MaxThreshold != nil {
		updates["max_threshold"] = req.MaxThreshold
	}
	if req.HighThreshold != nil {
		updates["high_threshold"] = req.HighThreshold
	}
	if err := h.DB.Model(&existing).Updates(updates).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	go middleware.WriteAudit(h.DB, c, "update", "monitoring_point", c.Param("id"))
	response.Success(c, nil)
}

// DeleteMonitoringPoint 删除测点
func (h *Handler) DeleteMonitoringPoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id 无效")
		return
	}
	if err := h.DB.Delete(&model.MonitoringPoint{}, id).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	go middleware.WriteAudit(h.DB, c, "delete", "monitoring_point", c.Param("id"))
	response.Success(c, nil)
}

// ==================== 时序读数 ====================

// PushReading 网关推送读数（无需登录，由网关Token认证）
func (h *Handler) PushReading(c *gin.Context) {
	// V47: 可选网关认证加固——DCIM_PUSH_REQUIRE_GATEWAY=1 时要求 X-Gateway-Code/Secret 头（默认关，兼容存量第三方直推）
	if os.Getenv("DCIM_PUSH_REQUIRE_GATEWAY") == "1" {
		if _, ok := h.GatewayAuth(c); !ok {
			return
		}
	}
	var req struct {
		PointCode string  `json:"point_code"`
		Value     float64 `json:"value"`
		Quality   int     `json:"quality"`
		Timestamp string  `json:"timestamp"` // RFC3339
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	var point model.MonitoringPoint
	if err := h.DB.Where("code = ?", req.PointCode).First(&point).Error; err != nil {
		response.Error(c, http.StatusNotFound, "测点不存在")
		return
	}
	ts := time.Now()
	if req.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			ts = t
		}
	}
	qty := 1
	if req.Quality > 0 {
		qty = req.Quality
	}
	reading := model.Reading{PointID: point.ID, Value: req.Value, Quality: qty, Timestamp: ts}
	if err := h.DB.Create(&reading).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "写入失败")
		return
	}
	// 更新测点最新值
	status := 1
	h.DB.Model(&point).Updates(map[string]interface{}{
		"current_value": req.Value,
		"status":        status,
		"updated_at":    ts,
	})
	// 阈值告警检测
	go h.checkThresholds(&point, reading)
	response.Success(c, gin.H{"id": reading.ID})
}

// BatchPushReadings 批量推送读数
func (h *Handler) BatchPushReadings(c *gin.Context) {
	if os.Getenv("DCIM_PUSH_REQUIRE_GATEWAY") == "1" {
		if _, ok := h.GatewayAuth(c); !ok {
			return
		}
	}
	var reqs []struct {
		PointCode string  `json:"point_code"`
		Value     float64 `json:"value"`
		Quality   int     `json:"quality"`
		Timestamp string  `json:"timestamp"`
	}
	if err := c.ShouldBindJSON(&reqs); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	now := time.Now()
	tx := h.DB.Begin()
	count := 0
	for _, r := range reqs {
		var point model.MonitoringPoint
		if err := tx.Where("code = ?", r.PointCode).First(&point).Error; err != nil {
			continue
		}
		ts := now
		if r.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, r.Timestamp); err == nil {
				ts = t
			}
		}
		qty := 1
		if r.Quality > 0 {
			qty = r.Quality
		}
		reading := model.Reading{PointID: point.ID, Value: r.Value, Quality: qty, Timestamp: ts}
		if err := tx.Create(&reading).Error; err != nil {
			continue
		}
		tx.Model(&point).Updates(map[string]interface{}{
			"current_value": r.Value,
			"status":        1,
			"updated_at":    ts,
		})
		count++
		// 异步告警检测
		go h.checkThresholds(&point, reading)
	}
	tx.Commit()
	response.Success(c, gin.H{"pushed": count})
}

// QueryReadings 查询时序读数
func (h *Handler) QueryReadings(c *gin.Context) {
	pointID, _ := strconv.Atoi(c.Query("point_id"))
	if pointID == 0 {
		response.Error(c, http.StatusBadRequest, "point_id 必填")
		return
	}
	start := c.Query("start")
	end := c.Query("end")
	q := h.DB.Where("point_id = ?", pointID)
	if start != "" {
		q = q.Where("timestamp >= ?", start)
	}
	if end != "" {
		q = q.Where("timestamp <= ?", end)
	}
	var readings []model.Reading
	q.Order("timestamp").Limit(1000).Find(&readings)
	response.Success(c, gin.H{"items": readings, "total": len(readings)})
}

// ==================== 告警规则 ====================

// ListAlertRules 告警规则列表
func (h *Handler) ListAlertRules(c *gin.Context) {
	var rules []model.AlertRule
	q := h.DB.Model(&model.AlertRule{})
	if pt := c.Query("point_type"); pt != "" {
		q = q.Where("point_type = ?", pt)
	}
	if pid := c.Query("point_id"); pid != "" {
		if id, err := strconv.Atoi(pid); err == nil && id > 0 {
			q = q.Where("point_id = ?", id)
		}
	}
	var total int64
	q.Count(&total)
	page, size := pagination(c)
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&rules)
	response.Success(c, gin.H{"items": rules, "total": total, "page": page, "size": size})
}

// CreateAlertRule 创建告警规则
func (h *Handler) CreateAlertRule(c *gin.Context) {
	var req model.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Name == "" || req.Operator == "" {
		response.Error(c, http.StatusBadRequest, "名称/运算符不能为空")
		return
	}
	if err := h.DB.Create(&req).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}
	response.Success(c, req)
}

// BatchCreateAlertRules 批量创建告警规则（按测点×模板生成，自动跳过重复）
func (h *Handler) BatchCreateAlertRules(c *gin.Context) {
	var req struct {
		Rules []model.AlertRule `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Rules) == 0 {
		response.Error(c, http.StatusBadRequest, "规则列表不能为空")
		return
	}
	if len(req.Rules) > 500 {
		response.Error(c, http.StatusBadRequest, "单次最多500条")
		return
	}
	created, skipped := 0, 0
	for _, r := range req.Rules {
		if r.Name == "" || r.Operator == "" {
			skipped++
			continue
		}
		// 同测点+同运算符+同阈值 视为重复，跳过
		var dup int64
		q := h.DB.Model(&model.AlertRule{}).Where("operator = ? AND threshold = ?", r.Operator, r.Threshold)
		if r.PointID != nil {
			q = q.Where("point_id = ?", *r.PointID)
		} else {
			q = q.Where("point_id IS NULL AND point_type = ?", r.PointType)
		}
		q.Count(&dup)
		if dup > 0 {
			skipped++
			continue
		}
		// 默认启用、严重度合法化
		if r.Severity < 1 || r.Severity > 3 {
			r.Severity = 2
		}
		if err := h.DB.Create(&r).Error; err == nil {
			created++
		} else {
			skipped++
		}
	}
	response.Success(c, gin.H{"created": created, "skipped": skipped})
}

// UpdateAlertRule 更新告警规则
func (h *Handler) UpdateAlertRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule model.AlertRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "规则不存在")
		return
	}
	// 先读原始请求体：既能绑定结构体，又能探测显式字段（局部更新如仅切换 enabled 时不清空适用目标）
	body, _ := io.ReadAll(c.Request.Body)
	var req model.AlertRule
	if err := json.Unmarshal(body, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(body, &raw)
	_, hasType := raw["point_type"]
	_, hasPoint := raw["point_id"]
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Operator != "" {
		updates["operator"] = req.Operator
	}
	// threshold/severity 可能合法为零值语义之外的局部更新：仅显式携带时更新，防止局部 PUT 清零
	if _, ok := raw["threshold"]; ok {
		updates["threshold"] = req.Threshold
	}
	if _, ok := raw["severity"]; ok {
		updates["severity"] = req.Severity
	}
	if _, ok := raw["enabled"]; ok {
		updates["enabled"] = req.Enabled
	}
	if _, ok := raw["auto_create_work_order"]; ok {
		updates["auto_create_work_order"] = req.AutoCreateWorkOrder
	}
	if _, ok := raw["escalate_after_min"]; ok {
		updates["escalate_after_min"] = req.EscalateAfterMin
	}
	if req.Message != "" {
		updates["message"] = req.Message
	}
	// 适用目标：仅在请求显式携带时更新（point_id=0/null 表示清除指定测点）
	if hasType {
		updates["point_type"] = req.PointType
	}
	if hasPoint {
		if req.PointID == nil || *req.PointID == 0 {
			updates["point_id"] = nil
		} else {
			updates["point_id"] = *req.PointID
		}
	}
	h.DB.Model(&rule).Updates(updates)
	response.Success(c, nil)
}

// DeleteAlertRule 删除告警规则
func (h *Handler) DeleteAlertRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.AlertRule{}, id)
	response.Success(c, nil)
}

// ==================== 告警记录 ====================

// ListAlerts 告警列表
func (h *Handler) ListAlerts(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	q := h.DB.Model(&model.Alert{})
	if sid := c.Query("site_id"); sid != "" {
		if id, _ := strconv.Atoi(sid); id > 0 {
			q = q.Where("site_id = ?", id)
		}
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if sv := c.Query("severity"); sv != "" {
		q = q.Where("severity = ?", sv)
	}
	if pid := c.Query("point_id"); pid != "" {
		if id, err := strconv.Atoi(pid); err == nil && id > 0 {
			q = q.Where("point_id = ?", id)
		}
	}
	if role != "global_admin" {
		siteIDs := h.userSiteIDs(userID)
		if len(siteIDs) == 0 {
			response.Success(c, gin.H{"items": []model.Alert{}, "total": 0})
			return
		}
		q = q.Where("site_id IN ?", siteIDs)
	}
	var total int64
	q.Count(&total)
	page, size := pagination(c)
	var alerts []model.Alert
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&alerts)
	// 附加测点名称与工单编码，前端免二次查询即可显示可读链接
	type alertView struct {
		model.Alert
		PointName string `json:"point_name"`
		PointCode string `json:"point_code"`
		WoCode    string `json:"wo_code"`
	}
	views := make([]alertView, 0, len(alerts))
	for _, a := range alerts {
		v := alertView{Alert: a}
		if a.PointID != nil {
			var pt model.MonitoringPoint
			if h.DB.Select("name", "code").First(&pt, *a.PointID).Error == nil {
				v.PointName, v.PointCode = pt.Name, pt.Code
			}
		}
		if a.WorkOrderID != nil {
			var wo model.WorkOrder
			if h.DB.Select("code").First(&wo, *a.WorkOrderID).Error == nil {
				v.WoCode = wo.Code
			}
		}
		views = append(views, v)
	}
	response.Success(c, gin.H{"items": views, "total": total, "page": page, "size": size})
}

// AcknowledgeAlert 确认告警
func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID := c.GetUint("user_id")
	now := time.Now()
	// 统一告警状态机: 0待处理 1已挂起 2已确认 3已结束（与 /alerts/:id/action 一致）
	h.DB.Model(&model.Alert{}).Where("id = ? AND status IN (0,1)", id).
		Updates(map[string]interface{}{"status": 2, "acknowledged_by": userID, "acknowledged_at": now})
	response.Success(c, nil)
}

// ResolveAlert 恢复告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	now := time.Now()
	h.DB.Model(&model.Alert{}).Where("id = ? AND status IN (0,1,2)", id).
		Updates(map[string]interface{}{"status": 3, "resolved_at": now})
	// 测点若无其他活跃告警则还原状态
	h.restorePointStatus(id)
	response.Success(c, nil)
}

// resolveAlertsByWorkOrder 工单完成/关闭时，自动结束关联的活跃告警并还原测点
func (h *Handler) resolveAlertsByWorkOrder(woID uint, reason string) {
	var alerts []model.Alert
	h.DB.Where("work_order_id = ? AND status < 3", woID).Find(&alerts)
	now := time.Now()
	for _, a := range alerts {
		h.DB.Model(&model.Alert{}).Where("id = ?", a.ID).
			Updates(map[string]interface{}{"status": 3, "resolved_at": now})
		h.DB.Create(&model.AlertActionLog{AlertID: a.ID, Action: "resolve", Reason: reason, Operator: 0})
		h.restorePointStatus(int(a.ID))
	}
}

// restorePointStatus 告警结束后，若该测点已无活跃告警，把测点状态从"告警"还原为"正常"
func (h *Handler) restorePointStatus(alertID int) {
	var a model.Alert
	if err := h.DB.First(&a, alertID).Error; err != nil || a.PointID == nil {
		return
	}
	var active int64
	h.DB.Model(&model.Alert{}).Where("point_id = ? AND status < 3", *a.PointID).Count(&active)
	if active == 0 {
		h.DB.Model(&model.MonitoringPoint{}).Where("id = ? AND status = 2", *a.PointID).Update("status", 1)
	}
}

// ==================== 告警引擎 ====================

// checkThresholds 检查阈值并触发告警
func (h *Handler) checkThresholds(point *model.MonitoringPoint, reading model.Reading) {
	// 1. 查找匹配的告警规则（先查测点专属规则，再查类型通用规则）
	var rules []model.AlertRule
	h.DB.Where("enabled = ?", true).Where(
		"(point_id = ?) OR (point_id IS NULL AND point_type = ?)",
		point.ID, point.Type,
	).Find(&rules)

	for _, rule := range rules {
		triggered := false
		switch rule.Operator {
		case "gt":
			triggered = reading.Value > rule.Threshold
		case "lt":
			triggered = reading.Value < rule.Threshold
		case "eq":
			triggered = math.Abs(reading.Value-rule.Threshold) < 0.001
		case "ne":
			triggered = math.Abs(reading.Value-rule.Threshold) >= 0.001
		}
		if !triggered {
			// 自动恢复：读数回归正常时结束该规则的活跃告警，并还原测点状态
			var actives []model.Alert
			h.DB.Where("rule_id = ? AND point_id = ? AND status < 3", rule.ID, point.ID).Find(&actives)
			for _, al := range actives {
				now := time.Now()
				h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).
					Updates(map[string]interface{}{"status": 3, "resolved_at": now})
				h.DB.Create(&model.AlertActionLog{AlertID: al.ID, Action: "resolve", Reason: "读数回归正常，系统自动恢复", Operator: 0})
				recoverMsg := fmt.Sprintf("%s 当前值 %.2f %s 已回归正常（阈值 %.2f %s），告警自动结束", point.Name, reading.Value, point.Unit, rule.Threshold, opName(rule.Operator))
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "alarm",
					Title:   fmt.Sprintf("[恢复] %s 告警已自动解除", point.Name),
					Content: recoverMsg,
					RefType: "alert", RefID: al.ID,
				})
				h.DispatchWebhook(model.EvAlarmResolve, fmt.Sprintf("[恢复] %s 告警已自动解除", point.Name), recoverMsg, map[string]interface{}{"alert_id": al.ID, "point_name": point.Name})
				// 关联工单若仍未完成则附注提示
				if al.WorkOrderID != nil {
					var wo model.WorkOrder
					if h.DB.First(&wo, *al.WorkOrderID).Error == nil && wo.Status < 3 {
						h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "progress", Content: "关联告警已自动恢复，请确认现场后关单"})
					}
				}
			}
			if len(actives) > 0 {
				h.restorePointStatus(int(actives[0].ID))
			}
			continue
		}
		// 检查是否已有同规则活跃告警（避免重复）
		var existing int64
		h.DB.Model(&model.Alert{}).Where("rule_id = ? AND point_id = ? AND status = 0", rule.ID, point.ID).Count(&existing)
		if existing > 0 {
			// V21 修复: PushReading 先把 status 重置为 1，告警持续期间须重新断言告警态
			h.DB.Model(&model.MonitoringPoint{}).Where("id = ? AND status <> 2", point.ID).Update("status", 2)
			continue
		}

		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("%s 测点 %s 当前值 %.2f %s，触发阈值 %.2f", typeName(point.Type), point.Name, reading.Value, point.Unit, rule.Threshold)
		}
		detail, _ := json.Marshal(gin.H{
			"point_code": point.Code, "point_name": point.Name, "point_type": point.Type,
			"current_value": reading.Value, "unit": point.Unit,
			"threshold": rule.Threshold, "operator": rule.Operator,
		})
		alert := model.Alert{
			SiteID:      point.SiteID,
			PointID:     &point.ID,
			RuleID:      &rule.ID,
			Severity:    rule.Severity,
			Message:     msg,
			Detail:      string(detail),
			Status:      0,
			TriggeredAt: time.Now(),
			CreatedAt:   time.Now(),
		}
		h.DB.Create(&alert)

		// 更新测点状态为告警
		h.DB.Model(point).Update("status", 2)

		// V22-② 风暴检测: 归入风暴且被抑制的告警不再通知/开工单(根因告警正常走)
		stormID, suppressed := h.DetectAlertStorm(&alert, point)
		if suppressed && stormID > 0 {
			continue
		}

		// 自动创建工单
		if rule.AutoCreateWorkOrder {
			h.createAlertWorkOrder(point, &alert, &rule)
		}

		// 推通知
		severityLabel := map[int]string{1: "提示", 2: "警告", 3: "严重"}[rule.Severity]
		h.DB.Create(&model.Notification{
			UserID:  0,
			Kind:    "alarm",
			Title:   fmt.Sprintf("[%s] %s 告警", severityLabel, point.Name),
			Content: msg,
			RefType: "alert", RefID: alert.ID,
		})
		// V18 外发 Webhook
		h.DispatchWebhook(model.EvAlarm, fmt.Sprintf("[%s] %s 告警", severityLabel, point.Name), msg, map[string]interface{}{
			"alert_id": alert.ID, "point_code": point.Code, "point_name": point.Name,
			"severity": rule.Severity, "site_id": point.SiteID,
		})
	}

	// V21-④ 内置过高线（high_threshold）：不依赖规则表，超过即 severity=3 严重告警
	h.checkHighThreshold(point, reading)
}

// checkHighThreshold 三级阈值的「过高线」内置告警：
// reading > high_threshold → severity 3 告警（幂等：kind标记去重）；回落 → 自动恢复
func (h *Handler) checkHighThreshold(point *model.MonitoringPoint, reading model.Reading) {
	if point.HighThreshold == nil {
		return
	}
	hi := *point.HighThreshold
	// 查该测点已有的内置过高告警（rule_id IS NULL 且 detail 含 high_line 标记）
	var actives []model.Alert
	h.DB.Where("point_id = ? AND rule_id IS NULL AND status < 3 AND detail LIKE ?", point.ID, "%high_line%").Find(&actives)
	if reading.Value > hi {
		if len(actives) > 0 {
			// 已有活跃过高告警，幂等；但 PushReading 会先把 status 重置为 1，须重新断言告警态
			h.DB.Model(&model.MonitoringPoint{}).Where("id = ? AND status <> 2", point.ID).Update("status", 2)
			return
		}
		msg := fmt.Sprintf("%s 测点 %s 当前值 %.2f %s，超过过高线 %.2f（三级阈值·严重）", typeName(point.Type), point.Name, reading.Value, point.Unit, hi)
		detail, _ := json.Marshal(gin.H{
			"point_code": point.Code, "point_name": point.Name, "point_type": point.Type,
			"current_value": reading.Value, "unit": point.Unit,
			"threshold": hi, "operator": "gt", "kind": "high_line",
		})
		alert := model.Alert{
			SiteID: point.SiteID, PointID: &point.ID,
			Severity: 3, Message: msg, Detail: string(detail),
			Status: 0, TriggeredAt: time.Now(), CreatedAt: time.Now(),
		}
		h.DB.Create(&alert)
		h.DB.Model(point).Update("status", 2)
		h.DB.Create(&model.Notification{UserID: 0, Kind: "alarm",
			Title: fmt.Sprintf("[严重·过高] %s", point.Name), Content: msg,
			RefType: "alert", RefID: alert.ID})
		h.DispatchWebhook(model.EvAlarm, fmt.Sprintf("[严重·过高] %s", point.Name), msg, map[string]interface{}{
			"alert_id": alert.ID, "point_code": point.Code, "point_name": point.Name,
			"severity": 3, "site_id": point.SiteID, "kind": "high_line",
		})
		return
	}
	// 回落 → 恢复
	for _, al := range actives {
		now := time.Now()
		h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).Updates(map[string]interface{}{"status": 3, "resolved_at": now})
		h.DB.Create(&model.AlertActionLog{AlertID: al.ID, Action: "resolve", Reason: "读数回落至过高线以下，系统自动恢复", Operator: 0})
		msg := fmt.Sprintf("%s 当前值 %.2f %s 已回落至过高线 %.2f 以下，告警自动结束", point.Name, reading.Value, point.Unit, hi)
		h.DB.Create(&model.Notification{UserID: 0, Kind: "alarm",
			Title: fmt.Sprintf("[恢复] %s 过高告警解除", point.Name), Content: msg,
			RefType: "alert", RefID: al.ID})
		h.DispatchWebhook(model.EvAlarmResolve, fmt.Sprintf("[恢复] %s 过高告警解除", point.Name), msg, map[string]interface{}{"alert_id": al.ID, "point_name": point.Name, "kind": "high_line"})
		h.restorePointStatus(int(al.ID))
	}
}

// createAlertWorkOrder 告警自动创建工单
func (h *Handler) createAlertWorkOrder(point *model.MonitoringPoint, alert *model.Alert, rule *model.AlertRule) {
	code := h.genCode("WO")
	now := time.Now()
	wo := model.WorkOrder{
		SiteID:      point.SiteID,
		Code:        code,
		Title:       fmt.Sprintf("告警：%s", alert.Message),
		Type:        "alarm",
		Priority:    rule.Severity,
		Status:      1,
		Description: alert.Message,
		RequesterID: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	// V22-③ 自动指派当班人
	duty := h.OnDutyUser(point.SiteID)
	assignNote := ""
	if duty != nil {
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
	h.applySLA(&wo) // V18 SLA 时限
	if err := h.DB.Create(&wo).Error; err == nil {
		h.DB.Model(alert).Update("work_order_id", wo.ID)
		h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "create", Content: "告警自动创建" + assignNote})
		h.DispatchWebhook(model.EvWorkOrder, "告警工单 "+wo.Code, fmt.Sprintf("%s%s", wo.Title, assignNote), map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority})
	}
}

// opName 比较符中文名
func opName(op string) string {
	names := map[string]string{"gt": "＞", "lt": "＜", "eq": "＝", "ne": "≠"}
	if n, ok := names[op]; ok {
		return n
	}
	return op
}

// typeName 测点类型中文名
func typeName(t string) string {
	names := map[string]string{
		"power_dist":    "配电",
		"ups":           "UPS",
		"hvac":          "空调",
		"temp_humidity": "温湿度",
		"water_leak":    "漏水",
		"smoke":         "烟感",
		"cdu":           "液冷CDU",
		"leak_rope":     "漏液绳",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return t
}

// MonitoringSummary 动环监控摘要（供仪表盘）
func (h *Handler) MonitoringSummary(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	typeSummary := func(typ string) gin.H {
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
		return gin.H{"total": total, "normal": normal, "alert": alert, "offline": offline}
	}
	summary := gin.H{
		"power_dist":    typeSummary("power_dist"),
		"ups":           typeSummary("ups"),
		"hvac":          typeSummary("hvac"),
		"temp_humidity": typeSummary("temp_humidity"),
		"water_leak":    typeSummary("water_leak"),
		"smoke":         typeSummary("smoke"),
		"cdu":           typeSummary("cdu"),
		"leak_rope":     typeSummary("leak_rope"),
	}
	// 活跃告警总数
	var activeAlerts int64
	h.DB.Model(&model.Alert{}).Where("status = 0").Count(&activeAlerts)
	summary["active_alerts"] = activeAlerts
	response.Success(c, summary)
}

// ==================== 历史数据清理（供定时任务调用） ====================

// CleanOldReadings 清理超过 retention 天的旧读数
func (h *Handler) CleanOldReadings(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := h.DB.Where("timestamp < ?", cutoff).Delete(&model.Reading{})
	return result.RowsAffected, result.Error
}

// ==================== 辅助 ====================

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 { page = 1 }
	if size < 1 || size > 100 { size = 20 }
	return page, size
}

func (h *Handler) userSiteIDs(userID uint) []uint {
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil { return nil }
	if user.Role == "global_admin" || user.SiteIDs == "" {
		var ids []uint
		h.DB.Model(&model.Site{}).Pluck("id", &ids)
		return ids
	}
	return parseSiteIDs(user.SiteIDs)
}