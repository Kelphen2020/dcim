package api

import (
	"fmt"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V30-① 维保关联备件清单 ====================

// ListMaintenanceSpares GET /maintenances/:id/spares
func (h *Handler) ListMaintenanceSpares(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	type msRow struct {
		ID         uint   `json:"id"`
		SpareID    uint   `json:"spare_id"`
		Qty        int    `json:"qty"`
		Remark     string `json:"remark"`
		SpareName  string `json:"spare_name"`
		SpareCode  string `json:"spare_code"`
		SpareUnit  string `json:"spare_unit"`
		SpareStock int    `json:"spare_stock"`
	}
	items := []msRow{}
	h.DB.Raw(`SELECT ms.id, ms.spare_id, ms.qty, ms.remark, s.name as spare_name, s.code as spare_code, s.unit as spare_unit, s.stock as spare_stock
		FROM t_maintenance_spare ms JOIN t_spare_part s ON s.id = ms.spare_id
		WHERE ms.maintenance_id = ? ORDER BY ms.id`, id).Scan(&items)
	response.Success(c, gin.H{"list": items})
}

// SaveMaintenanceSpares PUT /maintenances/:id/spares 批量设置清单(整单替换)
func (h *Handler) SaveMaintenanceSpares(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var m model.Maintenance
	if h.DB.First(&m, id).Error != nil {
		response.NotFound(c, "维保记录不存在")
		return
	}
	var req struct {
		Items []struct {
			SpareID uint   `json:"spare_id"`
			Qty     int    `json:"qty"`
			Remark  string `json:"remark"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	h.DB.Where("maintenance_id = ?", id).Delete(&model.MaintenanceSpare{})
	n := 0
	for _, it := range req.Items {
		if it.SpareID == 0 || it.Qty <= 0 {
			continue
		}
		var sp model.SparePart
		if h.DB.First(&sp, it.SpareID).Error != nil {
			continue
		}
		h.DB.Create(&model.MaintenanceSpare{MaintenanceID: id, SpareID: it.SpareID, Qty: it.Qty, Remark: it.Remark})
		n++
	}
	middleware.WriteAudit(h.DB, c, "update_maint_spares", fmt.Sprint(id), fmt.Sprintf("清单%d项", n))
	response.Success(c, gin.H{"saved": n})
}

// ConsumeMaintenanceSpares POST /maintenances/:id/consume {work_order_id}
// 按清单一键领用: 逐项出库(库存不足的跳过并报告), 关联工单, 生成领用汇总日志
func (h *Handler) ConsumeMaintenanceSpares(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var m model.Maintenance
	if h.DB.First(&m, id).Error != nil {
		response.NotFound(c, "维保记录不存在")
		return
	}
	var req struct {
		WorkOrderID uint `json:"work_order_id"`
	}
	c.ShouldBindJSON(&req)

	var items []model.MaintenanceSpare
	h.DB.Where("maintenance_id = ?", id).Find(&items)
	if len(items) == 0 {
		response.BadRequest(c, "该维保记录没有配置备件清单")
		return
	}
	type result struct {
		SpareName string `json:"spare_name"`
		Qty       int    `json:"qty"`
		OK        bool   `json:"ok"`
		Stock     int    `json:"stock"`
	}
	results := []result{}
	consumed, skipped := 0, 0
	uid := uint(middleware.GetUserID(c))
	uname := usernameOf(c)
	now := time.Now()
	for _, it := range items {
		var sp model.SparePart
		if h.DB.First(&sp, it.SpareID).Error != nil {
			continue
		}
		r := result{SpareName: sp.Name, Qty: it.Qty, Stock: sp.Stock}
		if sp.Stock >= it.Qty {
			sp.Stock -= it.Qty
			h.DB.Save(&sp)
			h.DB.Create(&model.SpareMovement{
				SpareID: sp.ID, Type: "out", Qty: it.Qty, StockAfter: sp.Stock,
				WorkOrderID: req.WorkOrderID, OperatorID: uid, Operator: uname,
				Reason: fmt.Sprintf("维保领用(维保#%d)", id),
			})
			h.checkSpareLow(&sp)
			r.OK = true
			r.Stock = sp.Stock
			consumed++
		} else {
			skipped++
		}
		results = append(results, r)
	}
	// 工单日志汇总
	if req.WorkOrderID > 0 && consumed > 0 {
		var wo model.WorkOrder
		if h.DB.First(&wo, req.WorkOrderID).Error == nil {
			h.DB.Create(&model.WorkOrderLog{
				OrderID: wo.ID, UserID: uid, Username: uname,
				Action:  "spare_out",
				Content: fmt.Sprintf("维保#%d 按清单领用备件: %d 项成功, %d 项库存不足跳过", id, consumed, skipped),
			})
		}
	}
	_ = now
	middleware.WriteAudit(h.DB, c, "consume_maint_spares", fmt.Sprint(id), fmt.Sprintf("领用%d项/跳过%d项", consumed, skipped))
	response.Success(c, gin.H{"consumed": consumed, "skipped": skipped, "detail": results})
}

// ==================== V30-② 采购申请流 ====================

// ListPurchaseRequests GET /purchase-requests?status=&spare_id=
func (h *Handler) ListPurchaseRequests(c *gin.Context) {
	type prRow struct {
		ID          uint       `json:"id"`
		Code        string     `json:"code"`
		SpareID     uint       `json:"spare_id"`
		Qty         int        `json:"qty"`
		Reason      string     `json:"reason"`
		Supplier    string     `json:"supplier"`
		BudgetYuan  float64    `json:"budget_yuan"`
		Status      int        `json:"status"`
		Applicant   string     `json:"applicant"`
		Approver    string     `json:"approver"`
		ApproveNote string     `json:"approve_note"`
		ApprovedAt  *time.Time `json:"approved_at"`
		StockedAt   *time.Time `json:"stocked_at"`
		CreatedAt   time.Time  `json:"created_at"`
		SpareName   string     `json:"spare_name"`
		SpareCode   string     `json:"spare_code"`
		SpareUnit   string     `json:"spare_unit"`
		StockNow    int        `json:"stock_now"`
		SafetyStock int        `json:"safety_stock"`
		Price       float64    `json:"price"`
	}
	q := h.DB.Table("t_purchase_request pr").
		Select(`pr.id, pr.code, pr.spare_id, pr.qty, pr.reason, pr.supplier, pr.budget_yuan, pr.status, pr.applicant, pr.approver,
			pr.approve_note, pr.approved_at, pr.stocked_at, pr.created_at,
			s.name as spare_name, s.code as spare_code, s.unit as spare_unit,
			s.stock as stock_now, s.safety_stock, s.price`).
		Joins("LEFT JOIN t_spare_part s ON s.id = pr.spare_id")
	if st := c.Query("status"); st != "" {
		q = q.Where("pr.status = ?", st)
	}
	if sid := c.Query("spare_id"); sid != "" {
		q = q.Where("pr.spare_id = ?", sid)
	}
	list := []prRow{}
	q.Order("pr.id DESC").Limit(300).Scan(&list)
	response.Success(c, gin.H{"list": list})
}

// CreatePurchaseRequest POST /purchase-requests {spare_id, qty, reason}
// 任何登录用户可发起; 同备件已有待审批申请则拒绝(防重复)
func (h *Handler) CreatePurchaseRequest(c *gin.Context) {
	var req struct {
		SpareID    uint    `json:"spare_id"`
		Qty        int     `json:"qty"`
		Reason     string  `json:"reason"`
		Supplier   string  `json:"supplier"`
		BudgetYuan float64 `json:"budget_yuan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpareID == 0 || req.Qty <= 0 {
		response.BadRequest(c, "spare_id/qty 必填")
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, req.SpareID).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	var pending int64
	h.DB.Model(&model.PurchaseRequest{}).Where("spare_id = ? AND status = 1", req.SpareID).Count(&pending)
	if pending > 0 {
		response.BadRequest(c, "该备件已有待审批的采购申请")
		return
	}
	if req.Reason == "" {
		req.Reason = fmt.Sprintf("库存 %d%s 低于安全水位 %d，申请补货", sp.Stock, sp.Unit, sp.SafetyStock)
	}
	if req.Supplier == "" {
		req.Supplier = sp.Supplier // 默认带出备件台账供应商
	}
	if req.BudgetYuan <= 0 && sp.Price > 0 {
		req.BudgetYuan = float64(req.Qty) * sp.Price // 按单价估算
	}
	pr := model.PurchaseRequest{
		Code: h.genCode("PR"), SpareID: req.SpareID, Qty: req.Qty, Reason: req.Reason,
		Supplier: req.Supplier, BudgetYuan: req.BudgetYuan,
		Status: 1, ApplicantID: uint(middleware.GetUserID(c)), Applicant: usernameOf(c),
	}
	if err := h.DB.Create(&pr).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	// 通知管理员审批
	h.DB.Create(&model.Notification{
		Kind: "purchase_request", Title: "采购申请待审批: " + sp.Name,
		Content: fmt.Sprintf("%s 申请采购 %s(%s) ×%d%s%s%s。事由: %s", pr.Applicant, sp.Name, sp.Code, pr.Qty, sp.Unit,
			map[bool]string{true: "", false: fmt.Sprintf("，供应商: %s", pr.Supplier)}[pr.Supplier == ""],
			map[bool]string{true: "", false: fmt.Sprintf("，预算 ¥%.2f", pr.BudgetYuan)}[pr.BudgetYuan <= 0], pr.Reason),
		RefType: "purchase_request", RefID: pr.ID,
	})
	middleware.WriteAudit(h.DB, c, "create_purchase_request", pr.Code, sp.Name)
	response.Created(c, pr)
}

// ReviewPurchaseRequest POST /purchase-requests/:id/review {approve: bool, note}
func (h *Handler) ReviewPurchaseRequest(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var pr model.PurchaseRequest
	if h.DB.First(&pr, id).Error != nil {
		response.NotFound(c, "申请不存在")
		return
	}
	if pr.Status != 1 {
		response.BadRequest(c, "仅待审批申请可审批")
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	now := time.Now()
	pr.ApproverID = uint(middleware.GetUserID(c))
	pr.Approver = usernameOf(c)
	pr.ApproveNote = req.Note
	pr.ApprovedAt = &now
	if req.Approve {
		pr.Status = 2
	} else {
		pr.Status = 3
	}
	h.DB.Save(&pr)
	var sp model.SparePart
	h.DB.First(&sp, pr.SpareID)
	verb := "已批准"
	if !req.Approve {
		verb = "已驳回"
	}
	h.DB.Create(&model.Notification{
		Kind: "purchase_request", Title: fmt.Sprintf("采购申请%s: %s", verb, sp.Name),
		Content: fmt.Sprintf("%s 审批%s %s ×%d。%s", pr.Approver, verb, sp.Name, pr.Qty, req.Note),
		RefType: "purchase_request", RefID: pr.ID,
	})
	middleware.WriteAudit(h.DB, c, "review_purchase_request", pr.Code, verb)
	response.Success(c, pr)
}

// StockPurchaseRequest POST /purchase-requests/:id/stock
// 已批准申请→执行入库(建流水+库存增加), 状态置4
func (h *Handler) StockPurchaseRequest(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var pr model.PurchaseRequest
	if h.DB.First(&pr, id).Error != nil {
		response.NotFound(c, "申请不存在")
		return
	}
	if pr.Status != 2 {
		response.BadRequest(c, "仅已批准申请可入库")
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, pr.SpareID).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	sp.Stock += pr.Qty
	h.DB.Save(&sp)
	now := time.Now()
	pr.Status = 4
	pr.StockedAt = &now
	h.DB.Save(&pr)
	h.DB.Create(&model.SpareMovement{
		SpareID: sp.ID, Type: "in", Qty: pr.Qty, StockAfter: sp.Stock,
		OperatorID: uint(middleware.GetUserID(c)), Operator: usernameOf(c),
		Reason: "采购入库(" + pr.Code + ")",
	})
	h.checkSpareLow(&sp) // 入库回升自动复位 low_warned
	middleware.WriteAudit(h.DB, c, "stock_purchase_request", pr.Code, fmt.Sprintf("入库%d→库存%d", pr.Qty, sp.Stock))
	response.Success(c, gin.H{"stock": sp.Stock, "request": pr})
}
