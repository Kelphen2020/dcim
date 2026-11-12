package api

import (
	"log"
	"time"

	"dcim/backend/internal/model"
)

// 巡检逾期扫描：启用中的计划若超过一个周期未执行，生成逾期记录(status=3)并通知责任人

func freqInterval(freq string) time.Duration {
	switch freq {
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 30 * 24 * time.Hour
	case "quarterly":
		return 90 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// ScanOverdueInspections 扫描逾期巡检计划，幂等：同一逾期窗口只生成一条记录
func (h *Handler) ScanOverdueInspections() int {
	var plans []model.InspectionPlan
	h.DB.Where("status = 1").Find(&plans)
	created := 0
	for _, p := range plans {
		interval := freqInterval(p.Freq)
		// 允许半个周期的宽限
		due := interval + interval/2
		anchor := time.Now().Add(-due)
		if p.LastRunAt != nil && p.LastRunAt.After(anchor) {
			continue // 周期内已执行
		}
		// once 类型不扫逾期
		if p.Freq == "once" {
			continue
		}
		// 幂等：该计划在本逾期窗口内已有逾期记录则跳过
		var exists int64
		h.DB.Model(&model.InspectionRecord{}).
			Where("plan_id = ? AND status = 3 AND inspected_at > ?", p.ID, anchor).Count(&exists)
		if exists > 0 {
			continue
		}
		rec := model.InspectionRecord{
			PlanID: p.ID, SiteID: p.SiteID, InspectorID: 0, Status: 3,
			Result: "", Abnormal: "超过巡检周期未执行",
			InspectedAt: time.Now(),
		}
		if err := h.DB.Create(&rec).Error; err != nil {
			continue
		}
		created++
		h.DB.Create(&model.Notification{
			UserID: p.AssigneeID, Kind: "inspection_overdue",
			Title:   "巡检逾期提醒",
			Content: "巡检计划「" + p.Name + "」已超过周期未执行，请尽快安排巡检",
			RefType: "inspection_plan", RefID: p.ID,
		})
	}
	if created > 0 {
		log.Printf("巡检逾期扫描：新增 %d 条逾期记录", created)
	}
	return created
}
