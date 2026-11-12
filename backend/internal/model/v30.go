package model

import "time"

// MaintenanceSpare V30-① 维保关联常用备件清单
// 一条维保记录(资产级)可挂多个备件项: 维保时按清单备料/一键领用
type MaintenanceSpare struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	MaintenanceID uint   `gorm:"uniqueIndex:uk_maint_spare,priority:1;not null" json:"maintenance_id"`
	SpareID       uint   `gorm:"uniqueIndex:uk_maint_spare,priority:2;not null" json:"spare_id"`
	Qty           int    `gorm:"not null;default:1" json:"qty"`
	Remark        string `gorm:"size:100" json:"remark"`
	// 冗余展示字段(查询时JOIN填充)
	SpareName string `gorm:"-" json:"spare_name"`
	SpareCode string `gorm:"-" json:"spare_code"`
	SpareUnit string `gorm:"-" json:"spare_unit"`
	SpareStock int   `gorm:"-" json:"spare_stock"`
}

func (MaintenanceSpare) TableName() string { return "t_maintenance_spare" }

// PurchaseRequest V30-② 备件采购申请流: 低库存→申请→审批→入库闭环
// Status: 1待审批 2已批准 3已驳回 4已入库
type PurchaseRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:50;uniqueIndex;not null" json:"code"` // PR-yyyyMMdd-NNN
	SpareID    uint       `gorm:"index;not null" json:"spare_id"`
	Qty        int        `gorm:"not null" json:"qty"`
	Reason     string     `gorm:"size:255" json:"reason"`
	Supplier   string     `gorm:"size:100" json:"supplier"`     // V31 供应商(默认带出备件供应商)
	BudgetYuan float64    `json:"budget_yuan"`                  // V31 预算金额(元)
	Status     int        `gorm:"default:1;index" json:"status"`
	ApplicantID uint      `json:"applicant_id"`
	Applicant  string     `gorm:"size:50" json:"applicant"`
	ApproverID uint       `json:"approver_id"`
	Approver   string     `gorm:"size:50" json:"approver"`
	ApproveNote string    `gorm:"size:255" json:"approve_note"`
	ApprovedAt *time.Time `json:"approved_at"`
	StockedAt  *time.Time `json:"stocked_at"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// 冗余展示
	SpareName  string `gorm:"-" json:"spare_name"`
	SpareCode  string `gorm:"-" json:"spare_code"`
	SpareUnit  string `gorm:"-" json:"spare_unit"`
	StockNow   int    `gorm:"-" json:"stock_now"`
	SafetyStock int   `gorm:"-" json:"safety_stock"`
	Price      float64 `gorm:"-" json:"price"`
}

func (PurchaseRequest) TableName() string { return "t_purchase_request" }
