package model

import "time"

// ==================== 用户与权限 ====================

// User 用户（角色：global_admin 全局管理员 / site_admin 站点管理员 / operator 值班员 / viewer 只读）
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:100" json:"-"`
	DisplayName  string    `gorm:"size:50" json:"display_name"`
	Role         string    `gorm:"size:20;not null;default:'viewer'" json:"role"`
	SiteIDs      string    `gorm:"size:255" json:"-"` // 逗号分隔的站点ID；空=全部（全局管理员）
	Phone        string    `gorm:"size:20" json:"phone"`
	Email        string    `gorm:"size:100" json:"email"`
	Status       int       `gorm:"default:1" json:"status"` // 1 正常 0 停用
	AllowedGroups string   `gorm:"size:255;default:'*'" json:"allowed_groups"` // 菜单权限：*=全部，或逗号分隔分组 key（dashboard,infra,asset,ops,monitor,energy,visual,network,report,system）
	LastLoginAt  *time.Time `json:"last_login_at"`
	WxOpenID     string     `gorm:"size:64;index" json:"-"` // 微信小程序 openid（一键登录绑定，不外泄）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "t_user" }

// Audit 审计日志（复用教学系统模式：action/target/ip/detail）
type Audit struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	Username  string    `gorm:"size:50" json:"username"`
	Action    string    `gorm:"size:50;index" json:"action"`
	Target    string    `gorm:"size:100" json:"target"`
	IP        string    `gorm:"size:45" json:"ip"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Audit) TableName() string { return "t_audit" }

// ==================== 站点与物理空间（四级层级） ====================

// Site 数据中心站点
type Site struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"size:32;uniqueIndex;not null" json:"code"` // 站点编码，资产编码前缀
	Name      string    `gorm:"size:100;not null" json:"name"`
	Level     int       `gorm:"default:0" json:"level"`                  // 0 未定级 1~4 => T1~T4
	Address   string    `gorm:"size:255" json:"address"`
	Province  string    `gorm:"size:32" json:"province"` // V49 所属省份（能效合规政策匹配）
	Lng       float64   `json:"lng"`
	Lat       float64   `json:"lat"`
	Manager   string    `gorm:"size:50" json:"manager"`
	Phone     string    `gorm:"size:20" json:"phone"`
	Status    int       `gorm:"default:1" json:"status"` // 1 正常 2 建设中 0 停用
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Site) TableName() string { return "t_site" }

// Room 机房房间
type Room struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"index;not null" json:"site_id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Floor     string    `gorm:"size:20" json:"floor"`
	Area      float64   `json:"area"`                 // 面积 ㎡
	Grade     string    `gorm:"size:20" json:"grade"` // A/B/C 级
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Room) TableName() string { return "t_room" }

// Row 机柜列（冷热通道）
type Row struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SiteID    uint      `gorm:"index;not null" json:"site_id"`
	RoomID    uint      `gorm:"index;not null" json:"room_id"`
	Name      string    `gorm:"size:50;not null" json:"name"`      // 列编号 如 A列
	Channel   string    `gorm:"size:20" json:"channel"`            // cold 冷通道 / hot 热通道 / none
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Row) TableName() string { return "t_row" }

// Cabinet 机柜
type Cabinet struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"index;not null" json:"site_id"`
	RoomID       uint      `gorm:"index;not null" json:"room_id"`
	RowID        uint      `gorm:"index" json:"row_id"`
	Code         string    `gorm:"size:50;not null" json:"code"` // 机柜编号 如 A01
	Model        string    `gorm:"size:100" json:"model"`
	TotalU       int       `gorm:"default:42" json:"total_u"`
	RatedPowerKW float64   `json:"rated_power_kw"`
	MaxWeightKG  int       `json:"max_weight_kg"`
	Status       int       `gorm:"default:1" json:"status"` // 1 在用 2 预留 0 停用
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Cabinet) TableName() string { return "t_cabinet" }

// ==================== 资产与设备（P1） ====================

// Asset 设备资产台账
// Status: 1 在架（在机柜内） 2 在库（未上架） 3 维修中 4 已报废
// Category: server 服务器 / network 网络设备 / storage 存储 / power 配电 / cooling 制冷 / security 安防 / other 其他
type Asset struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:50;uniqueIndex;not null" json:"code"` // 资产编码（全局唯一）
	Name       string     `gorm:"size:100;not null" json:"name"`
	Category   string     `gorm:"size:20;index;default:'other'" json:"category"`
	Brand      string     `gorm:"size:50" json:"brand"`
	Model      string     `gorm:"size:100" json:"model"`
	SN         string     `gorm:"size:100;index" json:"sn"` // 序列号
	SiteID     uint       `gorm:"index;not null" json:"site_id"`
	CabinetID  uint       `gorm:"index" json:"cabinet_id"` // 0=未上架（在库）
	RoomID     uint       `gorm:"index" json:"room_id"`    // V34: 非机架设备所在房间(配电柜/变压器/电池组/冷机等), 0=不适用
	StartU     int        `json:"start_u"`                 // 起始U位（自下而上 1 起）
	HeightU    int        `gorm:"default:1" json:"height_u"`
	PowerW     int        `json:"power_w"`     // 额定功耗（瓦）
	WeightKG   float64    `json:"weight_kg"`   // 重量
	IP         string     `gorm:"size:45" json:"ip"`       // 管理IP
	AssetNo    string     `gorm:"size:100" json:"asset_no"`    // 资产编号（财务）
	FixedAssetNo string  `gorm:"size:100" json:"fixed_asset_no"` // 固资编号
	Vendor     string     `gorm:"size:100" json:"vendor"`       // 供应商
	ContractNo string     `gorm:"size:100" json:"contract_no"`  // 合同编号
	Department string     `gorm:"size:50" json:"department"`    // 所属部门
	Dimensions string     `gorm:"size:50" json:"dimensions"`    // 尺寸（宽×深×高 mm）
	PurchaseAt *time.Time `json:"purchase_at"`             // 采购日期
	Owner      string     `gorm:"size:50" json:"owner"`    // 责任人/部门
	Status     int        `gorm:"default:2;index" json:"status"`
	Remark     string     `gorm:"size:255" json:"remark"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (Asset) TableName() string { return "t_asset" }

// Maintenance 维保记录（合同级维保 + 单次维修工单共用，Kind 区分）
// Kind: contract 维保合同 / repair 维修记录
// Status: 1 生效中 2 已过期 3 已完成（维修） 0 已作废
type Maintenance struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	AssetID   uint       `gorm:"index;not null" json:"asset_id"`
	Kind      string     `gorm:"size:20;default:'contract'" json:"kind"`
	Vendor    string     `gorm:"size:100" json:"vendor"`  // 维保厂商
	Contract  string     `gorm:"size:100" json:"contract"` // 合同编号
	StartAt   *time.Time `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Cost      float64    `json:"cost"`     // 维保费用
	Detail    string     `gorm:"type:text" json:"detail"` // 维修内容/合同说明
	Status    int        `gorm:"default:1" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Maintenance) TableName() string { return "t_maintenance" }

// ==================== 站点网关（边缘节点） ====================

// Gateway 站点边缘网关注册表
type Gateway struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	SiteID     uint       `gorm:"index;not null" json:"site_id"`
	Code       string     `gorm:"size:50;uniqueIndex;not null" json:"code"` // 网关编码
	Name       string     `gorm:"size:100" json:"name"`
	Secret     string     `gorm:"size:100" json:"-"` // 接入密钥
	IP         string     `gorm:"size:45" json:"ip"`
	Version    string     `gorm:"size:30" json:"version"`
	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	Status     int        `gorm:"default:1" json:"status"` // 1 启用 0 停用
	Remark     string     `gorm:"size:255" json:"remark"`
	// V47: 边缘运行时状态 JSON（di_state/do_state 位图、edge_points、push_fail、uptime、serials）
	RuntimeState string `gorm:"type:text" json:"runtime_state"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// GatewayCommand V47: 网关 DO 联动命令队列（平台入队 → 边缘拉取执行 → 回报结果）
type GatewayCommand struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	GatewayID    uint       `gorm:"index" json:"gateway_id"`
	Type         string     `gorm:"size:30" json:"type"`   // do_set
	Payload      string     `gorm:"size:500" json:"payload"`
	Status       string     `gorm:"size:20;index" json:"status"` // pending/dispatched/done/failed
	Result       string     `gorm:"size:500" json:"result"`
	CreatedBy    string     `gorm:"size:60" json:"created_by"`
	DispatchedAt *time.Time `json:"dispatched_at"`
	DoneAt       *time.Time `json:"done_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Gateway) TableName() string { return "t_gateway" }
func (GatewayCommand) TableName() string { return "t_gateway_command" }
