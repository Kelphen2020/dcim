package model

import "time"

// InspectionCheckin V19 扫码巡检签到（防补录：服务端在扫码瞬间记录时间，单次使用）
type InspectionCheckin struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	RefHash     string     `gorm:"size:64;index" json:"ref_hash"` // 扫码目标签名指纹（去重用）
	TargetType  string     `gorm:"size:20" json:"target_type"`    // cabinet/room
	TargetID    uint       `gorm:"index" json:"target_id"`
	SiteID      uint       `gorm:"index" json:"site_id"`
	UserID      uint       `json:"user_id"`
	CheckedInAt time.Time  `gorm:"index" json:"checked_in_at"` // 扫码瞬间服务端时间 = 巡检实际时间
	Used        bool       `gorm:"default:false" json:"used"`
	RecordID    uint       `json:"record_id"` // 提交后回填
	CreatedAt   time.Time  `json:"created_at"`
	UsedAt      *time.Time `json:"used_at"`
}

func (InspectionCheckin) TableName() string { return "t_inspection_checkin" }
