package model

import "time"

// FaultCase V22-① 故障案例库：工单完成沉淀「现象→原因→解决方案」，同类告警/设备自动推荐
type FaultCase struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex" json:"code"` // FC-yyyyMMdd-NNN
	SiteID      uint      `gorm:"index" json:"site_id"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	Symptom     string    `gorm:"type:text" json:"symptom"`   // 故障现象
	Cause       string    `gorm:"type:text" json:"cause"`     // 根本原因
	Solution    string    `gorm:"type:text" json:"solution"`  // 解决方案
	Category    string    `gorm:"size:30;index" json:"category"`  // power/ups/hvac/network/server/storage/security/other
	PointType   string    `gorm:"size:30;index" json:"point_type"` // 关联测点类型(temp_humidity/ups/hvac/power_dist/water_leak/smoke)
	AssetID     uint      `gorm:"index" json:"asset_id"`          // 关联资产(可空)
	Keywords    string    `gorm:"size:255" json:"keywords"`       // 逗号分隔关键词(推荐匹配用)
	SourceWOID  uint      `json:"source_wo_id"`                   // 沉淀来源工单
	DowntimeMin int       `json:"downtime_min"`                   // 故障历时(分钟)
	UsageCount  int       `gorm:"default:0" json:"usage_count"`   // 被推荐查看次数
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (FaultCase) TableName() string { return "t_fault_case" }

// AlertStorm V22-② 告警风暴：同一供电子树/机柜短时间内批量告警聚合为一条根因事件
type AlertStorm struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	SiteID         uint       `gorm:"index" json:"site_id"`
	RootKind       string     `gorm:"size:30" json:"root_kind"` // power_node/cabinet/room/site
	RootID         uint       `gorm:"index" json:"root_id"`
	RootName       string     `gorm:"size:100" json:"root_name"` // 根因对象名(如 UPS-1#)
	AlertCount     int        `gorm:"default:0" json:"alert_count"`
	SuppressedCnt  int        `gorm:"default:0" json:"suppressed_count"`
	Status         int        `gorm:"default:0;index" json:"status"` // 0 活跃 1 已结束
	FirstAlertAt   time.Time  `json:"first_alert_at"`
	LastAlertAt    time.Time  `json:"last_alert_at"`
	EndedAt        *time.Time `json:"ended_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (AlertStorm) TableName() string { return "t_alert_storm" }

// DutyShift V22-③ 值班排班：date+shift+user；0=全站点通用
type DutyShift struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"index" json:"site_id"` // 0=全部站点
	UserID    uint      `gorm:"index" json:"user_id"`
	DutyDate  time.Time `gorm:"index" json:"duty_date"` // 值班日期(零点)
	Shift     string    `gorm:"size:20;default:'day'" json:"shift"` // day 白班 / night 夜班 / full 全天
	Remark    string    `gorm:"size:200" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

func (DutyShift) TableName() string { return "t_duty_shift" }
