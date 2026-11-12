package api

// V70 B3-1 高噪规则处置动作（G3 治理动作化）
// 闭环定位：V64 质量治理只"评"不"治"——本版为噪声榜提供三个带预览的处置动作：
//   ① 调阈值（GET /alert-rules/:id/tuning-preview 近7天噪声统计+读数样本+建议阈值+预计降幅 → POST /alert-rules/:id/tune）
//   ② 停用规则（POST /alert-rules/:id/disable，审计留痕）
//   ③ 转规则优化工单（POST /alert-rules/:id/optimization-order，幂等：同规则未闭环工单唯一，指派当班人+SLA）
// 全部动作=人工确认后触发（默认"建议+确认"模式），动作全审计留痕。

import (
	"fmt"
	"math"
	"sort"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// v70NoiseStats 近7天规则噪声统计（口径同 V64）
type v70NoiseStats struct {
	Triggers   int     `json:"triggers"`
	Suspected  int     `json:"suspected"`
	Suppressed int     `json:"suppressed"`
	AckRate    float64 `json:"ack_rate"`
	NoiseScore int     `json:"noise_score"`
}

func (h *Handler) v70RuleStats(ruleID uint) v70NoiseStats {
	s := v70NoiseStats{}
	var alerts []model.Alert
	h.DB.Where("rule_id = ? AND created_at > ?", ruleID, time.Now().AddDate(0, 0, -7)).Find(&alerts)
	acked := 0
	for i := range alerts {
		a := &alerts[i]
		if v64IsSuspectedFalse(a) {
			s.Suspected++
		}
		if a.Suppressed {
			s.Suppressed++
		}
		if a.AcknowledgedAt != nil {
			acked++
		}
	}
	s.Triggers = len(alerts)
	if s.Triggers > 0 {
		s.AckRate = math.Round(float64(acked)*1000/float64(s.Triggers)) / 10
	}
	s.NoiseScore = s.Suspected*2 + s.Suppressed // 同 V64 噪声分口径
	return s
}

func v70Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// v70RulePointIDs 规则关联测点集合：指定测点 > 类型匹配（上限200）
func (h *Handler) v70RulePointIDs(rule *model.AlertRule) []uint {
	if rule.PointID != nil && *rule.PointID > 0 {
		return []uint{*rule.PointID}
	}
	var ids []uint
	q := h.DB.Model(&model.MonitoringPoint{})
	if rule.PointType != "" {
		q = q.Where("type = ?", rule.PointType)
	} else {
		return nil // 全局规则无样本可仿真
	}
	q.Limit(200).Pluck("id", &ids)
	return ids
}

// v70Exceed 按规则算子判断读数是否越限
func v70Exceed(op string, v, th float64) bool {
	switch op {
	case "gt":
		return v > th
	case "lt":
		return v < th
	case "eq":
		return v == th
	case "ne":
		return v != th
	}
	return false
}

// RuleTuningPreview GET /alert-rules/:id/tuning-preview（admin）
func (h *Handler) RuleTuningPreview(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rule model.AlertRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		response.BadRequest(c, "规则不存在")
		return
	}
	stats := h.v70RuleStats(rule.ID)
	out := gin.H{
		"rule":  rule,
		"stats": stats,
	}
	pids := h.v70RulePointIDs(&rule)
	if len(pids) == 0 {
		out["sample"] = nil
		out["explain"] = "该规则为全局规则（未绑定测点/类型），无读数样本可仿真，建议结合噪声榜人工评估。"
		response.Success(c, out)
		return
	}
	var vals []float64
	h.DB.Model(&model.Reading{}).
		Where("point_id IN ? AND timestamp > ?", pids, time.Now().AddDate(0, 0, -7)).
		Order("timestamp DESC").Limit(5000).Pluck("value", &vals)
	if len(vals) < 10 {
		out["sample"] = gin.H{"count": len(vals)}
		out["explain"] = "近 7 天读数样本不足（<10 条），无法仿真降幅，可直接停用或转优化工单。"
		response.Success(c, out)
		return
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	curTrigger := 0
	for _, v := range vals {
		if v70Exceed(rule.Operator, v, rule.Threshold) {
			curTrigger++
		}
	}
	// 建议阈值：gt → P95（仅约5%读数越限）；lt → P5；eq/ne 不建议调数值
	suggest := rule.Threshold
	if rule.Operator == "gt" {
		suggest = math.Round(v70Percentile(sorted, 95)*100) / 100
	} else if rule.Operator == "lt" {
		suggest = math.Round(v70Percentile(sorted, 5)*100) / 100
	}
	simTrigger := 0
	for _, v := range vals {
		if v70Exceed(rule.Operator, v, suggest) {
			simTrigger++
		}
	}
	reduction := 0.0
	if curTrigger > 0 {
		reduction = math.Round(float64(curTrigger-simTrigger)*1000/float64(curTrigger)) / 10
		if reduction < 0 {
			reduction = 0
		}
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	out["sample"] = gin.H{
		"count":            len(vals),
		"point_count":      len(pids),
		"min":              math.Round(sorted[0]*100) / 100,
		"avg":              math.Round(sum/float64(len(vals))*100) / 100,
		"max":              math.Round(sorted[len(sorted)-1]*100) / 100,
		"p50":              math.Round(v70Percentile(sorted, 50)*100) / 100,
		"p95":              math.Round(v70Percentile(sorted, 95)*100) / 100,
		"cur_trigger":      curTrigger,
		"sim_trigger":      simTrigger,
		"est_reduction_pct": reduction,
	}
	out["suggest_threshold"] = suggest
	out["explain"] = fmt.Sprintf("近 7 天 %d 个测点共 %d 条读数：当前阈值 %.2f 会触发 %d 次；建议阈值 %.2f 仿真仅触发 %d 次（预计减少 %.1f%%）。",
		len(pids), len(vals), rule.Threshold, curTrigger, suggest, simTrigger, reduction)
	response.Success(c, out)
}

// RuleTune POST /alert-rules/:id/tune（admin）调阈值
func (h *Handler) RuleTune(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rule model.AlertRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		response.BadRequest(c, "规则不存在")
		return
	}
	var req struct {
		NewThreshold *float64 `json:"new_threshold"`
		Note         string   `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NewThreshold == nil {
		response.BadRequest(c, "请提供 new_threshold")
		return
	}
	old := rule.Threshold
	if err := h.DB.Model(&rule).Updates(map[string]interface{}{"threshold": *req.NewThreshold}).Error; err != nil {
		response.Error(c, 500, "更新失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "tune", "alert_rule",
		fmt.Sprintf("规则调优: id=%d name=%s threshold %.2f→%.2f note=%s", rule.ID, rule.Name, old, *req.NewThreshold, req.Note))
	response.Success(c, gin.H{"id": rule.ID, "old_threshold": old, "new_threshold": *req.NewThreshold})
}

// RuleDisable POST /alert-rules/:id/disable（admin）停用规则
func (h *Handler) RuleDisable(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rule model.AlertRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		response.BadRequest(c, "规则不存在")
		return
	}
	if !rule.Enabled {
		response.BadRequest(c, "规则已处于停用状态")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.DB.Model(&rule).Updates(map[string]interface{}{"enabled": false}).Error; err != nil {
		response.Error(c, 500, "停用失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "disable", "alert_rule",
		fmt.Sprintf("停用高噪规则: id=%d name=%s note=%s", rule.ID, rule.Name, req.Note))
	response.Success(c, gin.H{"id": rule.ID, "enabled": false})
}

// RuleOptimizationOrder POST /alert-rules/:id/optimization-order（admin）转规则优化工单（幂等）
func (h *Handler) RuleOptimizationOrder(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rule model.AlertRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		response.BadRequest(c, "规则不存在")
		return
	}
	title := "【规则优化】" + rule.Name
	var existing model.WorkOrder
	if err := h.DB.Where("title = ? AND status <= 2", title).First(&existing).Error; err == nil {
		response.Success(c, gin.H{"work_order": existing, "existed": true})
		return
	}
	stats := h.v70RuleStats(rule.ID)
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	siteID := uint(0)
	if rule.PointID != nil && *rule.PointID > 0 {
		var pt model.MonitoringPoint
		if h.DB.First(&pt, *rule.PointID).Error == nil {
			siteID = pt.SiteID
		}
	}
	desc := fmt.Sprintf("告警规则「%s」(ID=%d) 近 7 天噪声统计：触发 %d 次，疑似误报 %d，被抑制 %d，确认率 %.1f%%，噪声分 %d。\n",
		rule.Name, rule.ID, stats.Triggers, stats.Suspected, stats.Suppressed, stats.AckRate, stats.NoiseScore)
	desc += fmt.Sprintf("当前条件：%s %.2f（级别 %d，自动工单 %v）。\n", rule.Operator, rule.Threshold, rule.Severity, rule.AutoCreateWorkOrder)
	desc += "请结合质量治理页噪声榜与读数分布评估：调整阈值 / 增加告警延迟 / 停用规则。\n"
	if req.Note != "" {
		desc += "发起人备注：" + req.Note + "\n"
	}
	desc += "\n本工单由告警质量治理「转优化工单」动作生成。"
	now := time.Now()
	wo := model.WorkOrder{
		SiteID: siteID, Code: h.genCode("WO"),
		Title: title, Type: "task", Priority: 3, Status: 1,
		Description: desc,
		RequesterID: c.GetUint("user_id"),
		CreatedBy:   c.GetUint("user_id"),
		CreatedAt:   now, UpdatedAt: now,
	}
	if duty := h.OnDutyUser(siteID); duty != nil {
		wo.AssigneeID = duty.ID
	}
	h.applySLA(&wo)
	if err := h.DB.Create(&wo).Error; err != nil {
		response.Error(c, 500, "创建工单失败: "+err.Error())
		return
	}
	if wo.AssigneeID > 0 {
		h.DB.Create(&model.Notification{
			UserID: wo.AssigneeID, Kind: "work_order",
			Title:   "规则优化工单待处理",
			Content: fmt.Sprintf("工单 %s「%s」已指派给你（来源：告警质量治理）", wo.Code, wo.Title),
			RefType: "work_order", RefID: wo.ID,
		})
	}
	go middleware.WriteAudit(h.DB, c, "create", "alert_rule",
		fmt.Sprintf("高噪规则转优化工单: rule_id=%d name=%s wo=%s note=%s", rule.ID, rule.Name, wo.Code, req.Note))
	response.Success(c, gin.H{"work_order": wo, "existed": false})
}
