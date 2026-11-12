package model

import "time"

// ==================== P3 动环监控 ====================

// MonitoringPoint 监控测点（配电/UPS/空调/温湿度/漏水/烟感）
type MonitoringPoint struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"index;not null" json:"site_id"`
	RoomID       *uint     `json:"room_id"`
	CabinetID    *uint     `json:"cabinet_id"`
	NetworkDeviceID *uint  `gorm:"index" json:"network_device_id"` // V66 B2-1: 绑定网络设备（SNMP/心跳类测点）
	Code         string    `gorm:"size:80;not null" json:"code"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Type         string    `gorm:"size:20;index;not null" json:"type"`       // power_dist/ups/hvac/temp_humidity/water_leak/smoke/cdu/leak_rope
	Protocol     string    `gorm:"size:20;default:'manual'" json:"protocol"` // modbus/snmp/mqtt/manual
	Address      string    `gorm:"size:200" json:"address"`                  // 协议地址（寄存器/OID/Topic）
	DataType     string    `gorm:"size:20;default:'float'" json:"data_type"` // float/int/bool/string
	Unit         string    `gorm:"size:20" json:"unit"`                      // 单位：V/A/kW/°C/%RH/ppm
	CurrentValue *float64  `json:"current_value"`                            // 最新读数
	Status       int       `gorm:"default:0" json:"status"`                  // 0 离线 1 正常 2 告警 3 故障
	MinThreshold *float64  `json:"min_threshold"` // 下限阈值（过低线）
	MaxThreshold *float64  `json:"max_threshold"` // 上限阈值（超高线）
	HighThreshold *float64 `json:"high_threshold"` // V21 过高线（三级阈值: 超过=严重）
	ScaleFactor  float64   `gorm:"default:1" json:"scale_factor"`            // 缩放系数（Modbus 原始值→实际值）
	PollInterval int       `gorm:"default:30" json:"poll_interval"`          // 采集间隔（秒）
	AlarmDelay   int       `gorm:"default:0" json:"alarm_delay"`             // 告警延迟（秒，连续超阈值才告警）
	ProtocolConfig string  `gorm:"type:text" json:"protocol_config"`         // 协议特定配置 JSON
	Description  string    `gorm:"size:500" json:"description"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (MonitoringPoint) TableName() string { return "t_monitoring_point" }

// Reading 时序读数
type Reading struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	PointID   uint      `gorm:"index;not null" json:"point_id"`
	Value     float64   `gorm:"not null" json:"value"`
	Quality   int       `gorm:"default:1" json:"quality"` // 0 坏值 1 正常
	Timestamp time.Time `gorm:"index;not null" json:"timestamp"`
}

func (Reading) TableName() string { return "t_reading" }

// AlertRule 告警规则
type AlertRule struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	Name                string `gorm:"size:100;not null" json:"name"`
	PointID             *uint  `json:"point_id"`           // 关联测点，nil=全局规则
	PointType           string `gorm:"size:20" json:"point_type"` // 测点类型过滤（空=全部）
	Metric              string `gorm:"size:20;default:'value'" json:"metric"` // value/status
	Operator            string `gorm:"size:5;not null" json:"operator"`       // gt/lt/eq/ne
	Threshold           float64 `json:"threshold"`
	Severity            int    `gorm:"default:2" json:"severity"`             // 1 提示 2 警告 3 严重
	Enabled             bool   `gorm:"default:true" json:"enabled"`
	AutoCreateWorkOrder bool   `gorm:"default:false" json:"auto_create_work_order"`
	EscalateAfterMin    int    `gorm:"default:0" json:"escalate_after_min"` // V13 未确认超过N分钟自动升级(0=不升级)
	Message             string `gorm:"size:500" json:"message"` // 告警模板
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (AlertRule) TableName() string { return "t_alert_rule" }

// Alert 告警记录
type Alert struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	SiteID         uint       `gorm:"index;not null" json:"site_id"`
	PointID        *uint      `json:"point_id"`
	RuleID         *uint      `json:"rule_id"`
	Severity       int        `gorm:"default:2" json:"severity"` // 1 提示 2 警告 3 严重
	Message        string     `gorm:"size:500" json:"message"`
	Detail         string     `gorm:"size:2000" json:"detail"`
	Status         int        `gorm:"default:0;index" json:"status"`         // 0 活跃 1 已确认 2 已恢复
	AcknowledgedBy *uint      `json:"acknowledged_by"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	WorkOrderID    *uint      `json:"work_order_id"` // 自动生成的工单ID
	Watched        bool       `gorm:"default:false" json:"watched"` // 已关注
	Blocked        bool       `gorm:"default:false" json:"blocked"` // 已屏蔽
	StormID        uint       `gorm:"index" json:"storm_id"`      // V22-② 所属告警风暴(0=非风暴)
	Suppressed     bool       `gorm:"default:false" json:"suppressed"` // V22-② 被抑制(非根因)
	Escalated      bool       `gorm:"default:false" json:"escalated"`   // V13 已升级
	EscalatedAt    *time.Time `json:"escalated_at"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (Alert) TableName() string { return "t_alert" }