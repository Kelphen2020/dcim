package model

import "time"

// RoomLayoutItem V26 房间3D自定义布局项：拖拽后的位置/旋转持久化
// RefKind: cabinet 机柜 / decor 装饰设备 / model 导入模型
type RoomLayoutItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoomID    uint      `gorm:"uniqueIndex:uk_room_ref,priority:1;not null" json:"room_id"`
	RefKind   string    `gorm:"size:20;uniqueIndex:uk_room_ref,priority:2;not null" json:"ref_kind"`
	RefID     uint      `gorm:"uniqueIndex:uk_room_ref,priority:3;not null;default:0" json:"ref_id"`
	RefKey    string    `gorm:"size:60;uniqueIndex:uk_room_ref,priority:4;not null;default:''" json:"ref_key"` // decor无ID时用键名
	X         float64   `json:"x"`
	Z         float64   `json:"z"`
	RotY      float64   `json:"rot_y"`
	ModelSrc  string    `gorm:"size:255" json:"model_src"` // GLB 文件路径(导入模型)
	ModelName string    `gorm:"size:100" json:"model_name"`
	Scale     float64   `gorm:"default:1" json:"scale"`
	// V28 构件尺寸参数化(米): 0=用默认尺寸
	SizeX     float64   `json:"size_x"`
	SizeY     float64   `json:"size_y"`
	SizeZ     float64   `json:"size_z"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (RoomLayoutItem) TableName() string { return "t_room_layout" }
