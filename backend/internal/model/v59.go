package model

import "time"

// ============ V59 应急预案与演练管理 ============

// EmergencyPlan 应急预案（事前定义的应急处置程序卡，区别于事后记录的 FaultCase）
type EmergencyPlan struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Code            string     `gorm:"size:50;uniqueIndex" json:"code"` // EP-yyyyMMdd-NNN
	SiteID          uint       `gorm:"index" json:"site_id"`            // 0=全站通用
	Title           string     `gorm:"size:200;not null" json:"title"`
	Scenario        string     `gorm:"size:30;index" json:"scenario"` // power/ups/hvac/fire/network/water/security/other
	TriggerCond     string     `gorm:"type:text" json:"trigger_cond"` // 触发情形
	Steps           string     `gorm:"type:text" json:"steps"`        // 处置步骤（换行分隔，按序执行）
	OwnerID         uint       `gorm:"index" json:"owner_id"`         // 责任人
	Priority        int        `gorm:"default:2" json:"priority"`     // 1高 2中 3低
	MatchKeywords   string     `gorm:"size:255" json:"match_keywords"`   // 告警匹配关键词（逗号分隔）
	MatchPointTypes string     `gorm:"size:255" json:"match_point_types"` // 告警匹配测点类型（逗号分隔，对应 MonitoringPoint.Type）
	DrillCycleDays  int        `gorm:"default:0" json:"drill_cycle_days"` // 要求演练周期（天），0=无需定期演练
	LastDrillAt     *time.Time `json:"last_drill_at"`
	DrillCount      int        `gorm:"default:0" json:"drill_count"`
	Status          int        `gorm:"default:1;index" json:"status"` // 1启用 0停用
	Builtin         bool       `gorm:"default:false" json:"builtin"` // 内置种子预案（可编辑不可删）
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (EmergencyPlan) TableName() string { return "t_emergency_plan" }

// DrillRecord 演练记录（执行留痕：军队/等保机房检查硬性要求）
type DrillRecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PlanID      uint      `gorm:"index" json:"plan_id"`
	SiteID      uint      `gorm:"index" json:"site_id"`
	Title       string    `gorm:"size:200" json:"title"`
	DrillDate   time.Time `json:"drill_date"`
	Participants string   `gorm:"type:text" json:"participants"`
	DurationMin int       `json:"duration_min"`
	Result      string    `gorm:"size:20;index" json:"result"` // success/partial/failed
	Issues      string    `gorm:"type:text" json:"issues"` // 发现的问题
	WoID        uint      `json:"wo_id"`                 // 整改工单（问题转工单后回写）
	RecorderID  uint      `json:"recorder_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (DrillRecord) TableName() string { return "t_drill_record" }
