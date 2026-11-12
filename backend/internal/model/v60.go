package model

import "time"

// V60 资产退役与处置流程
// 闭环: 退役申请(人工/盘点盘亏转办) → 管理员审批(批准自动下架+报废) → 处置登记(方式/残值/凭证/数据擦除确认) → 留痕统计
// 业务背景: 资产处置留痕是军队/等保机房检查硬性材料; 此前"报废"只是台账上一个状态位, 无审批无处置记录

// AssetRetirement 资产退役申请单
type AssetRetirement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"` // RT-yyyyMMdd-NNN
	AssetID     uint      `gorm:"index;not null" json:"asset_id"`
	SiteID      uint      `gorm:"index" json:"site_id"`
	Reason      string    `gorm:"size:20;not null" json:"reason"` // age 服役到期 / fault 故障不可修 / loss 盘点盘亏 / upgrade 技术更新 / other 其他
	Description string    `gorm:"type:text" json:"description"`
	SourceType  string    `gorm:"size:20" json:"source_type"` // stocktake=盘点转办 / 空=人工发起
	SourceID    uint      `json:"source_id"`
	Status      int       `gorm:"default:1;index" json:"status"` // 1待审批 2已批准待处置 3已驳回 4处置完成 5已撤销
	ApplicantID uint      `gorm:"index" json:"applicant_id"`
	AppliedAt   time.Time `json:"applied_at"`

	ApproverID  uint       `json:"approver_id"`
	ApprovedAt  *time.Time `json:"approved_at"`
	ApproveNote string     `gorm:"size:255" json:"approve_note"`

	DisposalMethod string     `gorm:"size:20" json:"disposal_method"` // resale 转售 / scrap 报废销毁 / takeback 厂商回收 / donate 捐赠 / other 其他
	DisposalDate   *time.Time `json:"disposal_date"`
	DisposalAmount float64    `json:"disposal_amount"` // 残值回收金额（元）
	CertificateNo  string     `gorm:"size:100" json:"certificate_no"` // 处置凭证编号
	DataWiped      bool       `json:"data_wiped"`                     // 数据擦除/介质销毁确认（合规必填）
	Witness        string     `gorm:"size:50" json:"witness"`         // 见证人
	DisposalNote   string     `gorm:"type:text" json:"disposal_note"`
	DisposalBy     uint       `json:"disposal_by"`
	DisposalAt     *time.Time `json:"disposal_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AssetRetirement) TableName() string { return "t_asset_retirement" }

// EvAssetRetire Webhook 事件：资产退役（申请/批准/驳回/处置完成）
const EvAssetRetire = "asset_retire"
