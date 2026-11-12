package model

import "time"

// PowerRedundancy V54 站点供电冗余评估记录（site_id 唯一，ScanPowerRedundancy 维护）
type PowerRedundancy struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SiteID            uint      `gorm:"uniqueIndex" json:"site_id"`
	Score             int       `json:"score"`
	Grade             string    `gorm:"size:20;index" json:"grade"` // ok / warn / risk
	Issues            string    `gorm:"type:text" json:"issues"`    // JSON []powerIssue
	NodeCount         int       `json:"node_count"`
	CabinetCount      int       `json:"cabinet_count"`
	OpenWorkOrderID   uint      `json:"open_work_order_id"`
	LastNotifiedGrade string    `gorm:"size:20" json:"last_notified_grade"`
	ComputedAt        time.Time `json:"computed_at"`
}

func (PowerRedundancy) TableName() string { return "t_power_redundancy" }

// EvPowerRedundancy V18 Webhook 事件：供电冗余恶化/恢复
const EvPowerRedundancy = "power_redundancy"

const (
	PowerGradeOK   = "ok"
	PowerGradeWarn = "warn"
	PowerGradeRisk = "risk"
)
