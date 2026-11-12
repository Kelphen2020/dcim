package model

import "time"

// PUERecord PUE 能效记录
type PUERecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SiteID      uint      `gorm:"index;not null" json:"site_id"`
	TotalPower  float64   `json:"total_power"`  // 总功率 kW
	ITPower     float64   `json:"it_power"`     // IT设备功率 kW
	PUE         float64   `json:"pue"`          // PUE = TotalPower / ITPower
	RecordedAt  time.Time `gorm:"index" json:"recorded_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (PUERecord) TableName() string { return "t_pue_record" }