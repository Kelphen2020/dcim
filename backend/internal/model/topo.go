package model

import "time"

// TopoDiagram 拓扑图持久化（X6 画布布局）
// Kind: power_sld 供电单线图 / net_topo 网络拓扑
// Data: JSON，存储自定义节点位置 { nodes: {ref_key: {x,y}}, meta: {...} }
type TopoDiagram struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"uniqueIndex:uk_site_kind;not null" json:"site_id"`
	Kind      string    `gorm:"size:30;uniqueIndex:uk_site_kind;not null" json:"kind"`
	Data      string    `gorm:"type:text" json:"data"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TopoDiagram) TableName() string { return "t_topo_diagram" }
