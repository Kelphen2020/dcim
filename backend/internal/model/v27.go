package model

import "time"

// ModelAsset V27 3D模型库：上传的GLB模型可复用（构件库面板拖入场景）
type ModelAsset struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"` // 原始文件名
	Src       string    `gorm:"size:255;not null" json:"src"`  // /uploads/models/xxx.glb
	Ext       string    `gorm:"size:10" json:"ext"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (ModelAsset) TableName() string { return "t_model_asset" }
