package model

import "time"

// ==================== P2 基础运维 ====================

// WorkOrder 运维工单
// Type: fault 故障 / request 需求 / incident 事件 / task 任务
// Status: 1 待受理 2 处理中 3 已完成 4 已关闭 5 已取消
// Priority: 1 低 2 中 3 高 4 紧急
type WorkOrder struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SiteID      uint      `gorm:"index" json:"site_id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"` // WO-yyyyMMdd-NNN
	Title       string    `gorm:"size:200;not null" json:"title"`
	Type        string    `gorm:"size:20;not null;default:'task'" json:"type"`
	Priority    int       `gorm:"default:2" json:"priority"`
	Status      int       `gorm:"default:1" json:"status"`
	Description string    `gorm:"type:text" json:"description"`
	AssetID     uint      `json:"asset_id"` // 关联资产（可空）
	CabinetID   uint      `json:"cabinet_id"`
	RequesterID uint      `json:"requester_id"` // 报单人
	AssigneeID  uint      `json:"assignee_id"`  // 处理人
	AcceptedAt  *time.Time `json:"accepted_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	Resolution  string    `gorm:"type:text" json:"resolution"` // 处理结果
	CreatedBy   uint      `json:"created_by"`
	// V18 SLA 时限管理
	SlaRespondDue    *time.Time `json:"sla_respond_due"`    // 响应时限（受理截止）
	SlaResolveDue    *time.Time `json:"sla_resolve_due"`    // 解决时限（完成截止）
	SlaRespondWarned bool       `gorm:"default:false" json:"sla_respond_warned"`
	SlaResolveWarned bool       `gorm:"default:false" json:"sla_resolve_warned"`
	SlaRespondBreach bool       `gorm:"default:false" json:"sla_respond_breach"`
	SlaResolveBreach bool       `gorm:"default:false" json:"sla_resolve_breach"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (WorkOrder) TableName() string { return "t_work_order" }

// WorkOrderLog 工单流转记录
type WorkOrderLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"index" json:"order_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `gorm:"size:50" json:"username"`
	Action    string    `gorm:"size:30" json:"action"` // create/accept/progress/finish/close/cancel/reassign
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (WorkOrderLog) TableName() string { return "t_work_order_log" }

// InspectionPlan 巡检计划
// Freq: daily 每日 / weekly 每周 / monthly 每月 / once 一次性
// Status: 1 启用 0 停用
type InspectionPlan struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SiteID     uint      `gorm:"index" json:"site_id"`
	RoomID     uint      `gorm:"index" json:"room_id"` // V19 关联房间（扫码巡检定位计划，0=全站）
	Name       string    `gorm:"size:100;not null" json:"name"`
	Freq       string    `gorm:"size:20;default:'daily'" json:"freq"`
	Items      string    `gorm:"type:text" json:"items"` // 巡检项，每行一条
	AssigneeID uint      `json:"assignee_id"`
	Status     int       `gorm:"default:1" json:"status"`
	LastRunAt  *time.Time `json:"last_run_at"`
	CreatedBy  uint      `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (InspectionPlan) TableName() string { return "t_inspection_plan" }

// InspectionRecord 巡检记录（一次执行）
// Status: 1 正常 2 发现异常 3 逾期未巡
type InspectionRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PlanID     uint      `gorm:"index" json:"plan_id"`
	SiteID     uint      `gorm:"index" json:"site_id"`
	InspectorID uint     `json:"inspector_id"`
	Status     int       `gorm:"default:1" json:"status"`
	Result     string    `gorm:"type:text" json:"result"` // 逐项结果（JSON 或文本）
	Abnormal   string    `gorm:"type:text" json:"abnormal"` // 异常描述
	WorkOrderID uint     `json:"work_order_id"` // 异常转工单
	// V19 扫码巡检 + 报告签发
	Source      string `gorm:"size:20;default:'web'" json:"source"`       // web 桌面录入 / scan 扫码签到
	CheckinID   uint   `json:"checkin_id"`                                  // 关联签到记录（扫码防补录）
	TargetType  string `gorm:"size:20" json:"target_type"`                  // cabinet/room（扫码目标）
	TargetID    uint   `json:"target_id"`
	TargetName  string `gorm:"size:100" json:"target_name"`                 // 机柜编号/房间名（快照）
	PointSnap   string `gorm:"type:text" json:"point_snap"`                 // 测点实时值快照 JSON（报告归档用）
	ReviewerID  uint   `json:"reviewer_id"`                                 // 审核人
	ReviewedAt  *time.Time `json:"reviewed_at"`
	Signature   string `gorm:"size:100" json:"signature"`                   // 签字（审核人姓名文本）
	ReportCode  string `gorm:"size:50" json:"report_code"`                  // 报告编号 IR-yyyyMMdd-NNN
	InspectedAt time.Time `json:"inspected_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (InspectionRecord) TableName() string { return "t_inspection_record" }

// ChangeRequest 变更申请
// Risk: 1 低 2 中 3 高
// Status: 1 草稿 2 待审批 3 已批准 4 已驳回 5 执行中 6 已完成 7 已回滚
type ChangeRequest struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	SiteID       uint       `gorm:"index" json:"site_id"`
	Code         string     `gorm:"size:50;uniqueIndex;not null" json:"code"` // CHG-yyyyMMdd-NNN
	Title        string     `gorm:"size:200;not null" json:"title"`
	Risk         int        `gorm:"default:1" json:"risk"`
	Status       int        `gorm:"default:1" json:"status"`
	Description  string     `gorm:"type:text" json:"description"`
	Impact       string     `gorm:"type:text" json:"impact"`       // 影响范围
	Plan         string     `gorm:"type:text" json:"plan"`         // 实施方案
	RollbackPlan string     `gorm:"type:text" json:"rollback_plan"` // 回退方案
	WindowStart  *time.Time `json:"window_start"` // 变更窗口
	WindowEnd    *time.Time `json:"window_end"`
	ApplicantID  uint       `json:"applicant_id"`
	ApproverID   uint       `json:"approver_id"`
	ApprovedAt   *time.Time `json:"approved_at"`
	ApproveNote  string     `gorm:"type:text" json:"approve_note"`
	ExecutorID   uint       `json:"executor_id"`
	FinishedAt   *time.Time `json:"finished_at"`
	FinishNote   string     `gorm:"type:text" json:"finish_note"`
	CabinetIDs   string     `gorm:"size:500" json:"cabinet_ids"`    // V17 关联机柜(逗号分隔)
	PowerNodeIDs string     `gorm:"size:500" json:"power_node_ids"` // V17 关联供电节点(逗号分隔)
	NetworkDeviceIDs string `gorm:"size:500" json:"network_device_ids"` // B2-3 关联网络设备(逗号分隔,执行时触发配置备份)
	CreatedBy    uint       `json:"created_by"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (ChangeRequest) TableName() string { return "t_change_request" }

// PowerChain 供电链路（层级树：市电/UPS/列头柜/PDU → 末端设备）
// Kind: utility 市电 / transformer 变压器 / ups UPS / ats ATS / rack_pdu 列头柜 / pdu PDU / other
// Status: 1 正常 2 告警 3 故障 0 停用
type PowerChain struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"index" json:"site_id"`
	ParentID     uint      `gorm:"index;default:0" json:"parent_id"` // 0=根节点
	Name         string    `gorm:"size:100;not null" json:"name"`
	Kind         string    `gorm:"size:20;not null" json:"kind"`
	Model        string    `gorm:"size:100" json:"model"`
	RatedKVA     float64   `json:"rated_kva"`  // 额定容量
	LoadKW       float64   `json:"load_kw"`    // 当前负载（手工/采集）
	Status       int       `gorm:"default:1" json:"status"`
	AssetID      uint      `json:"asset_id"`   // 关联资产台账（可空）
	PointID      uint      `gorm:"index;default:0" json:"point_id"`   // V14 绑定测点(实时负载来源)
	CabinetID    uint      `gorm:"index;default:0" json:"cabinet_id"` // V14 PDU/支路供电的机柜
	Remark       string    `gorm:"type:text" json:"remark"`
	CreatedBy    uint      `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (PowerChain) TableName() string { return "t_power_chain" }

// Notification 站内通知
// Kind: maintenance_due 维保到期 / work_order 工单 / change 变更 / inspection 巡检 / system 系统
// Status: 0 未读 1 已读
type Notification struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"` // 0=全员广播
	Kind      string    `gorm:"size:30;index" json:"kind"`
	Title     string    `gorm:"size:200;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	RefType   string    `gorm:"size:30" json:"ref_type"` // asset/work_order/change_request...
	RefID     uint      `json:"ref_id"`
	Read      bool      `gorm:"default:false" json:"read"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Notification) TableName() string { return "t_notification" }
