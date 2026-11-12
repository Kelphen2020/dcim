package model

import "time"

// V50 设备健康度与预测性维护
//
// 设计原则（完整闭环）：
//   采集：复用既有测点读数/告警/维保/供电链路绑定，不新增采集通道
//   评估：ScanAssetHealth 定期扣分制评分（8 因子，每项带证据），结果落 AssetHealth
//   报警：等级恶化 → 通知中心 + Webhook 外发 + 可选自动开预测性维护工单（幂等）
//   使用：资产列表健康分列/筛选、详情抽屉因子证据+趋势、Dashboard 汇总卡穿透
//   闭环：维护工单完成 → 告警清理/维保续期 → 下轮评估分数回升 → 恢复通知

// AssetHealth 资产健康度当前态（每资产一行，评估时 UPSERT）
type AssetHealth struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	AssetID           uint      `gorm:"uniqueIndex;not null" json:"asset_id"`
	SiteID            uint      `gorm:"index" json:"site_id"`
	Score             int       `gorm:"index" json:"score"`             // 0-100
	Grade             string    `gorm:"size:20;index" json:"grade"`     // healthy/attention/sub_healthy/risk
	Factors           string    `gorm:"type:text" json:"factors"`       // JSON [{key,label,deduct,evidence}]
	PointCount        int       `json:"point_count"`                    // 参与评估的绑定测点数
	OpenWorkOrderID   uint      `json:"open_work_order_id"`             // 未闭环的预测性维护工单（幂等键）
	LastNotifiedGrade string    `gorm:"size:20" json:"last_notified_grade"` // 上次已通知的恶化等级（防重复通知）
	ComputedAt        time.Time `gorm:"index" json:"computed_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (AssetHealth) TableName() string { return "t_asset_health" }

// AssetHealthHistory 健康度每日快照（趋势曲线数据源，asset_id+day 唯一）
type AssetHealthHistory struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	AssetID   uint      `gorm:"uniqueIndex:uk_ah_asset_day;index" json:"asset_id"`
	Day       string    `gorm:"size:10;uniqueIndex:uk_ah_asset_day" json:"day"` // YYYY-MM-DD
	Score     int       `json:"score"`
	Grade     string    `gorm:"size:20" json:"grade"`
	CreatedAt time.Time `json:"created_at"`
}

func (AssetHealthHistory) TableName() string { return "t_asset_health_history" }

// EvAssetHealth V18 Webhook 事件：健康度恶化/恢复
const EvAssetHealth = "asset_health"

// 健康度等级常量
const (
	HealthGradeHealthy   = "healthy"
	HealthGradeAttention = "attention"
	HealthGradeSubHealth = "sub_healthy"
	HealthGradeRisk      = "risk"
)
