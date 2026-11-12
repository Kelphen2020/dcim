package model

import "time"

// 告警五态：0 待处理 1 已挂起 2 已确认 3 已结束；另有 已关注/已屏蔽 辅助标记

// AlertActionLog 告警操作留痕（每次状态变更一条）
type AlertActionLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AlertID   uint      `gorm:"index;not null" json:"alert_id"`
	Action    string    `gorm:"size:30;not null" json:"action"` // suspend/resume/acknowledge/resolve/watch/block/unwatch/unblock
	Reason    string    `gorm:"size:500" json:"reason"`
	Operator  uint      `json:"operator"`
	CreatedAt time.Time `json:"created_at"`
}

func (AlertActionLog) TableName() string { return "t_alert_action_log" }

// TrendTemplate 趋势分析模板（测点组合+时间范围+视图类型）
type TrendTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	PointIDs    string    `gorm:"size:500" json:"point_ids"`                    // 逗号分隔测点ID
	RangePreset string    `gorm:"size:20;default:'1h'" json:"range_preset"`     // 1h/24h/1w/1m/3m
	ViewType    string    `gorm:"size:20;default:'chart'" json:"view_type"`     // chart/table
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (TrendTemplate) TableName() string { return "t_trend_template" }
