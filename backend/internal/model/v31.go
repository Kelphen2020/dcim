package model

import "time"

// SpareBOM V31-① 备件↔资产分类/型号 关联(BOM): 某类设备常用哪些备件
// 一个备件可绑多个资产分类; AssetModel 为空=该分类通用, 非空=仅该型号
type SpareBOM struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SpareID    uint      `gorm:"uniqueIndex:uk_bom,priority:1;not null" json:"spare_id"`
	AssetCat   string    `gorm:"size:30;uniqueIndex:uk_bom,priority:2;not null" json:"asset_cat"`   // server/network/storage/power/hvac/security/other
	AssetModel string    `gorm:"size:100;uniqueIndex:uk_bom,priority:3;not null;default:''" json:"asset_model"`
	Qty        int       `gorm:"not null;default:1" json:"qty"` // 单台设备建议备件数
	Remark     string    `gorm:"size:150" json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	// JOIN 展示
	SpareName  string  `gorm:"-" json:"spare_name"`
	SpareCode  string  `gorm:"-" json:"spare_code"`
	SpareUnit  string  `gorm:"-" json:"spare_unit"`
	SpareStock int     `gorm:"-" json:"spare_stock"`
	SparePrice float64 `gorm:"-" json:"spare_price"`
}

func (SpareBOM) TableName() string { return "t_spare_bom" }
