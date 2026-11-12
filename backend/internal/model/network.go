package model

import "time"

// ==================== P4 网络管理 ====================

// NetworkDevice 网络设备（交换机/路由器/防火墙/无线AP等）
type NetworkDevice struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"index;not null" json:"site_id"`
	RoomID       *uint     `json:"room_id"`
	CabinetID    *uint     `json:"cabinet_id"`
	Code         string    `gorm:"size:80;uniqueIndex;not null" json:"code"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Type         string    `gorm:"size:20;index;not null" json:"type"` // switch/router/firewall/ap/controller
	Vendor       string    `gorm:"size:50" json:"vendor"`             // 厂商
	Model        string    `gorm:"size:80" json:"model"`
	MgmtIP       string    `gorm:"size:45" json:"mgmt_ip"` // 管理IP
	Firmware     string    `gorm:"size:50" json:"firmware"`
	SerialNo     string    `gorm:"size:50" json:"serial_no"`
	PortCount    int       `gorm:"default:0" json:"port_count"` // 端口总数
	Status       int       `gorm:"default:1" json:"status"`     // 0 离线 1 在线 2 维护 3 故障
	AssetID      *uint     `gorm:"index" json:"asset_id"`       // V69/B2-4: 强关联资产台账（从资产创建时预填）
	Description  string    `gorm:"size:500" json:"description"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (NetworkDevice) TableName() string { return "t_network_device" }

// NetworkPort 网络端口
type NetworkPort struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	DeviceID   uint   `gorm:"index;not null" json:"device_id"`
	Name       string `gorm:"size:50;not null" json:"name"`     // 端口名（GigabitEthernet0/1）
	Index      int    `gorm:"default:0" json:"index"`           // 端口索引
	Type       string `gorm:"size:20" json:"type"`              // copper/fiber/sfp
	Speed      string `gorm:"size:20" json:"speed"`             // 100M/1G/10G/40G/100G
	Status     int    `gorm:"default:0" json:"status"`          // 0 down 1 up
	MAC        string `gorm:"size:20" json:"mac"`
	VLANMode   string `gorm:"size:20" json:"vlan_mode"`        // access/trunk
	NativeVLAN *uint  `json:"native_vlan"`
	AllowedVLANs string `gorm:"size:500" json:"allowed_vlans"` // trunk allowed vlan list
	Description string `gorm:"size:200" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (NetworkPort) TableName() string { return "t_network_port" }

// NetworkLink 链路（连接两个端口）
type NetworkLink struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SiteID    uint   `gorm:"index;not null" json:"site_id"`
	PortAID   uint   `gorm:"index;not null" json:"port_a_id"` // 本端端口
	PortBID   uint   `gorm:"index;not null" json:"port_b_id"` // 对端端口
	LinkType  string `gorm:"size:20;default:'ethernet'" json:"link_type"` // ethernet/fiber/trunk
	Bandwidth string `gorm:"size:20" json:"bandwidth"` // 带宽
	Status    int    `gorm:"default:1" json:"status"`   // 0 down 1 up
	Label     string `gorm:"size:200" json:"label"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (NetworkLink) TableName() string { return "t_network_link" }

// IPPool IP地址池/网段
type IPPool struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"index;not null" json:"site_id"`
	Name      string    `gorm:"size:100;not null" json:"name"`     // 池名称
	CIDR      string    `gorm:"size:45;not null" json:"cidr"`      // 192.168.1.0/24
	Gateway   string    `gorm:"size:45" json:"gateway"`
	DNS       string    `gorm:"size:200" json:"dns"`
	Type      string    `gorm:"size:20;default:'lan'" json:"type"` // lan/mgmt/wan/dmz
	UsedCount int       `gorm:"default:0" json:"used_count"`
	TotalCount int      `gorm:"default:0" json:"total_count"`
	Status    int       `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (IPPool) TableName() string { return "t_ip_pool" }

// IPAssignment IP分配记录
type IPAssignment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PoolID    uint      `gorm:"index;not null" json:"pool_id"`
	IP        string    `gorm:"size:45;not null" json:"ip"`
	MAC       string    `gorm:"size:20" json:"mac"`
	Hostname  string    `gorm:"size:100" json:"hostname"`
	Type      string    `gorm:"size:20;default:'static'" json:"type"` // static/dhcp/reserved
	Status    int       `gorm:"default:1" json:"status"` // 0 空闲 1 已用 2 保留
	AssetID   *uint     `json:"asset_id"` // 关联资产
	DeviceID  *uint     `json:"device_id"` // 关联网络设备
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (IPAssignment) TableName() string { return "t_ip_assignment" }

// VLAN 虚拟局域网
type VLAN struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"index;not null" json:"site_id"`
	VID       int       `gorm:"not null" json:"vid"`                     // VLAN ID (1-4094)
	Name      string    `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"size:500" json:"description"`
	Color     string    `gorm:"size:20" json:"color"`                    // 拓扑图着色
	Subnet    string    `gorm:"size:45" json:"subnet"`                   // 关联子网
	Gateway   string    `gorm:"size:45" json:"gateway"`
	Status    int       `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (VLAN) TableName() string { return "t_vlan" }

// ConfigBackup 配置备份
type ConfigBackup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DeviceID    uint      `gorm:"index;not null" json:"device_id"`
	Filename    string    `gorm:"size:200;not null" json:"filename"`
	Content     string    `gorm:"type:text" json:"content"`
	Size        int64     `gorm:"default:0" json:"size"` // 字节数
	Version     string    `gorm:"size:50" json:"version"`
	BackupType  string    `gorm:"size:20;default:'manual'" json:"backup_type"` // manual/scheduled
	Status      int       `gorm:"default:1" json:"status"` // 1 成功 0 失败
	ErrorMsg    string    `gorm:"size:500" json:"error_msg"`
	BackedUpAt  time.Time `json:"backed_up_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (ConfigBackup) TableName() string { return "t_config_backup" }