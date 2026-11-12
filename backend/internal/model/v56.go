package model

import "time"

// CapacityHistory V56 站点容量每日快照（site_id+day 唯一；启动回填+每小时幂等写当天）
type CapacityHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"uniqueIndex:uk_cap_hist,priority:1" json:"site_id"`
	Day          string    `gorm:"size:10;uniqueIndex:uk_cap_hist,priority:2" json:"day"` // 2006-01-02
	UsedU        int       `json:"used_u"`
	TotalU       int       `json:"total_u"`
	UsedPowerW   int64     `json:"used_power_w"`
	RatedPowerW  int64     `json:"rated_power_w"`
	UsedWeightKG float64   `json:"used_weight_kg"`
	AssetCount   int       `json:"asset_count"`
	CreatedAt    time.Time `json:"created_at"`
}

func (CapacityHistory) TableName() string { return "t_capacity_history" }

// CapacityForecast V56 站点容量饱和预测记录（site_id 唯一）
type CapacityForecast struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	SiteID     uint   `gorm:"uniqueIndex" json:"site_id"`
	Grade      string `gorm:"size:20;index" json:"grade"` // ok / warn / risk / stable(无增长)
	USlope     float64 `json:"u_slope"`                   // U/天
	UFullAt    string `gorm:"size:10" json:"u_full_at"`   // 预计饱和日，空=不会饱和
	UR2        float64 `json:"u_r2"`
	PowerSlope float64 `json:"power_slope"` // W/天
	PowerFullAt string `gorm:"size:10" json:"power_full_at"`
	PowerR2    float64 `json:"power_r2"`
	LowConfidence bool  `json:"low_confidence"` // r²<0.3：仅展示不报警（V29 教训）
	UsedU      int     `json:"used_u"`
	TotalU     int     `json:"total_u"`
	UsedPowerW int64   `json:"used_power_w"`
	RatedPowerW int64  `json:"rated_power_w"`
	OpenWorkOrderID   uint      `json:"open_work_order_id"`
	LastNotifiedGrade string    `gorm:"size:20" json:"last_notified_grade"`
	ComputedAt        time.Time `json:"computed_at"`
}

func (CapacityForecast) TableName() string { return "t_capacity_forecast" }

// EvCapacityForecast V18 Webhook 事件：容量饱和预警/恢复
const EvCapacityForecast = "capacity_forecast"

const (
	CapGradeOK     = "ok"
	CapGradeWarn   = "warn"
	CapGradeRisk   = "risk"
	CapGradeStable = "stable"
)
