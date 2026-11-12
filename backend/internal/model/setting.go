package model

import "time"

// SystemSetting 系统级键值配置（管理员可改，如站点默认主题）
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"size:512" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "t_system_setting" }
