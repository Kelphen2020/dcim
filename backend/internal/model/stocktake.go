package model

import "time"

// ==================== V13 资产盘点 ====================

// StocktakeTask 盘点任务
// Status: 0 草稿 1 盘点中 2 已完成
type StocktakeTask struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	SiteID       uint       `gorm:"index;not null" json:"site_id"`
	Code         string     `gorm:"size:50;uniqueIndex;not null" json:"code"` // ST-yyyyMMdd-NNN
	Name         string     `gorm:"size:100;not null" json:"name"`
	CabinetID    uint       `gorm:"default:0" json:"cabinet_id"` // 0=全站盘点，否则限定机柜
	Status       int        `gorm:"default:0;index" json:"status"`
	TotalCount   int        `gorm:"default:0" json:"total_count"`   // 账面资产数
	CheckedCount int        `gorm:"default:0" json:"checked_count"` // 已核对数
	DiffCount    int        `gorm:"default:0" json:"diff_count"`    // 差异数（盘亏+盘盈+位置不符）
	CreatedBy    uint       `json:"created_by"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Remark       string     `gorm:"size:500" json:"remark"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (StocktakeTask) TableName() string { return "t_stocktake_task" }

// StocktakeItem 盘点明细
// Result: 0 未核对 1 在账相符 2 盘亏(账面有实物无) 3 盘盈(实物有账面无) 4 位置不符
type StocktakeItem struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TaskID       uint       `gorm:"index;not null" json:"task_id"`
	AssetID      uint       `gorm:"index" json:"asset_id"` // 盘盈项可为 0
	AssetCode    string     `gorm:"size:50" json:"asset_code"`
	AssetName    string     `gorm:"size:100" json:"asset_name"`
	CabinetID    uint       `json:"cabinet_id"`
	CabinetCode  string     `gorm:"size:50" json:"cabinet_code"`
	StartU       int        `json:"start_u"`
	BookStatus   int        `json:"book_status"` // 账面状态快照（1在架2在库3维修4报废）
	Result       int        `gorm:"default:0;index" json:"result"`
	ActualCabinetID uint    `json:"actual_cabinet_id"` // 实际位置（位置不符时记录）
	ActualStartU    int     `json:"actual_start_u"`
	Remark       string     `gorm:"size:255" json:"remark"`
	CheckedBy    uint       `json:"checked_by"`
	CheckedAt    *time.Time `json:"checked_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (StocktakeItem) TableName() string { return "t_stocktake_item" }
