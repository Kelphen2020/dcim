package model

import "time"

// LayoutTemplate V28 布局模板：整房间布局另存/套用
// Items 为 JSON 数组: [{ref_kind, ref_key, code, x, z, rot_y, scale, size_x, size_y, size_z, model_src}]
// code 仅 cabinet 项有值(套用时按编号在目标房间重映射 ref_id)
type LayoutTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Remark    string    `gorm:"size:255" json:"remark"`
	Items     string    `gorm:"type:text" json:"items"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LayoutTemplate) TableName() string { return "t_layout_template" }
