package model

import "time"

// V62 巡检任务自动生成与漏检治理
// 闭环: 计划(既有 t_inspection_plan) → 按周期自动生成任务(t_inspection_task) → 巡检提交自动核销 → 周期结束未执行判漏检(通知+Webhook) → 豁免/改派留痕 → 治理统计

// InspectionTask 巡检任务（按计划周期生成的应执行实例）
type InspectionTask struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PlanID    uint      `gorm:"uniqueIndex:uk_task_plan_period;index" json:"plan_id"`
	PeriodKey string    `gorm:"size:20;uniqueIndex:uk_task_plan_period" json:"period_key"` // 2026-11-06 / 2026-W45 / 2026-11 / 2026-Q4
	SiteID    uint      `gorm:"index" json:"site_id"`
	RoomID    uint      `gorm:"index" json:"room_id"`
	Freq      string    `gorm:"size:20" json:"freq"` // 周期快照（计划后续改周期不影响已生成任务）
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	AssigneeID  uint      `gorm:"index" json:"assignee_id"` // 责任人（计划责任人，缺省取生成时当班人）
	Status      int       `gorm:"default:0;index" json:"status"` // 0待巡检 1已完成 2漏检 3已豁免
	RecordID    uint      `json:"record_id"`                     // 完成时关联的巡检记录
	DoneAt      *time.Time `json:"done_at"`
	ExemptReason string    `gorm:"size:200" json:"exempt_reason"`
	MissNotified bool      `json:"miss_notified"` // 漏检通知幂等标记
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (InspectionTask) TableName() string { return "t_inspection_task" }
