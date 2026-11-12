package model

import "time"

// NotifyChannel V18 外发通知通道（Webhook：企业微信/飞书/钉钉/通用）
type NotifyChannel struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:100;not null" json:"name"`
	Kind       string     `gorm:"size:20;not null;default:'webhook'" json:"kind"` // wecom/feishu/dingtalk/webhook
	WebhookURL string     `gorm:"size:500;not null" json:"webhook_url"`
	Events     string     `gorm:"size:255;default:'*'" json:"events"` // 逗号分隔事件种类，*=全部
	Enabled    bool       `gorm:"default:true" json:"enabled"`
	Remark     string     `gorm:"size:200" json:"remark"`
	LastStatus string     `gorm:"size:200" json:"last_status"` // ok / fail:xxx
	LastSentAt *time.Time `json:"last_sent_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (NotifyChannel) TableName() string { return "t_notify_channel" }

// SLA 事件种类常量（与前端勾选一致）
const (
	EvAlarm          = "alarm"
	EvAlarmResolve   = "alarm_resolve"
	EvAlarmEscalate  = "alarm_escalation"
	EvWorkOrder      = "work_order"
	EvSlaWarn        = "sla_warn"
	EvSlaBreach      = "sla_breach"
	EvMaintenanceDue = "maintenance_due"
	EvInspectionMissed = "inspection_missed" // V62 巡检漏检
)
