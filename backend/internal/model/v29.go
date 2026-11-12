package model

import "time"

// SparePart V29-① 备件台账
type SparePart struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Category    string    `gorm:"size:50;index" json:"category"` // 电气/暖通/IT/安防/耗材
	Spec        string    `gorm:"size:200" json:"spec"`          // 规格型号
	Unit        string    `gorm:"size:10;default:'个'" json:"unit"`
	SiteID      uint      `gorm:"index" json:"site_id"`
	Stock       int       `gorm:"not null;default:0" json:"stock"`        // 当前库存
	SafetyStock int       `gorm:"not null;default:0" json:"safety_stock"` // 安全水位
	Location    string    `gorm:"size:100" json:"location"`               // 存放位置
	Price       float64   `json:"price"`                                  // 单价(元)
	Supplier    string    `gorm:"size:100" json:"supplier"`
	LowWarned   bool      `gorm:"default:false" json:"low_warned"` // 低库存已通知(防重复)
	Remark      string    `gorm:"type:text" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SparePart) TableName() string { return "t_spare_part" }

// SpareMovement V29-① 出入库流水(②领用关联工单)
type SpareMovement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SpareID     uint      `gorm:"index;not null" json:"spare_id"`
	Type        string    `gorm:"size:10;not null" json:"type"` // in 入库 / out 出库
	Qty         int       `gorm:"not null" json:"qty"`          // 正数
	StockAfter  int       `json:"stock_after"`                  // 变动后库存
	WorkOrderID uint      `gorm:"index" json:"work_order_id"`   // ②关联工单(领用出库)
	OperatorID  uint      `json:"operator_id"`
	Operator    string    `gorm:"size:50" json:"operator"`
	Reason      string    `gorm:"size:200" json:"reason"` // 采购入库/维修领用/盘盈盘亏/退库
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (SpareMovement) TableName() string { return "t_spare_movement" }

// TrendForecast V29-③ 测点趋势预测(线性回归, 计算态不落库)
type TrendForecast struct {
	PointID      uint    `json:"point_id"`
	PointCode    string  `json:"point_code"`
	PointName    string  `json:"point_name"`
	Unit         string  `json:"unit"`
	Current      float64 `json:"current"`
	Slope        float64 `json:"slope"`          // 每小时变化量(最小二乘斜率)
	Intercept    float64 `json:"intercept"`      // 截距
	Threshold    float64 `json:"threshold"`      // 参照阈值(取最近会越过的 max/high)
	HoursToLimit float64 `json:"hours_to_limit"` // 预计越限小时数(-1=不会越限/远离)
	Predicted24  float64 `json:"predicted_24h"`  // 24h 后预测值
	WillBreach   bool    `json:"will_breach"`    // 24h 内将越限
	Direction    string  `json:"direction"`      // rising/falling/flat
	Confidence   string  `json:"confidence"`     // high/medium/low
	R2           float64 `json:"r2"`             // 拟合优度
	Samples      int     `json:"samples"`
}
