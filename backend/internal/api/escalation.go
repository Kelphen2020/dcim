package api

import (
	"fmt"
	"log"
	"time"

	"dcim/backend/internal/model"
)

// ==================== V13 告警升级 ====================
// 规则配置 escalate_after_min > 0 时：告警触发后超过 N 分钟仍为「待处理(status=0)」
// → severity+1(封顶3)、标记 escalated、写留痕、推通知、严重级自动开工单。

// ScanAlertEscalation 扫描应升级的活跃告警，返回升级条数（幂等：escalated=true 不重复升级）
func (h *Handler) ScanAlertEscalation() int {
	var alerts []model.Alert
	// 仅扫未升级的待处理告警
	h.DB.Where("status = 0 AND escalated = ?", false).Find(&alerts)
	now := time.Now()
	upgraded := 0
	for _, al := range alerts {
		if al.RuleID == nil {
			continue
		}
		var rule model.AlertRule
		if err := h.DB.First(&rule, *al.RuleID).Error; err != nil {
			continue
		}
		if rule.EscalateAfterMin <= 0 || !rule.Enabled {
			continue
		}
		if now.Sub(al.TriggeredAt) < time.Duration(rule.EscalateAfterMin)*time.Minute {
			continue
		}
		newSev := al.Severity + 1
		if newSev > 3 {
			newSev = 3
		}
		updates := map[string]interface{}{
			"escalated": true, "escalated_at": now, "severity": newSev,
		}
		h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).Updates(updates)
		h.DB.Create(&model.AlertActionLog{
			AlertID: al.ID, Action: "escalate",
			Reason: fmt.Sprintf("超过 %d 分钟未确认，系统自动升级（严重度 %d→%d）", rule.EscalateAfterMin, al.Severity, newSev),
			Operator: 0,
		})
		pointName := fmt.Sprintf("告警#%d", al.ID)
		if al.PointID != nil {
			var pt model.MonitoringPoint
			if h.DB.Select("name").First(&pt, *al.PointID).Error == nil {
				pointName = pt.Name
			}
		}
		escMsg := fmt.Sprintf("%s（严重度已升至 %d）触发超过 %d 分钟仍无人确认，请立即处理", al.Message, newSev, rule.EscalateAfterMin)
		h.DB.Create(&model.Notification{
			UserID: 0, Kind: "alarm_escalation",
			Title:   fmt.Sprintf("[升级] %s 告警超时未处理", pointName),
			Content: escMsg,
			RefType: "alert", RefID: al.ID,
		})
		h.DispatchWebhook(model.EvAlarmEscalate, fmt.Sprintf("[升级] %s 告警超时未处理", pointName), escMsg, map[string]interface{}{"alert_id": al.ID, "point_name": pointName, "severity": newSev})
		// 升级到严重(3)且规则要求自动开工单、且尚无关联工单 → 补开工单
		if newSev == 3 && rule.AutoCreateWorkOrder && al.WorkOrderID == nil {
			var pt model.MonitoringPoint
			if al.PointID != nil && h.DB.First(&pt, *al.PointID).Error == nil {
				al2 := al
				al2.Severity = newSev
				h.createAlertWorkOrder(&pt, &al2, &rule)
			}
		}
		upgraded++
	}
	if upgraded > 0 {
		log.Printf("告警升级扫描：%d 条告警已升级", upgraded)
	}
	return upgraded
}
