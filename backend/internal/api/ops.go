package api

import (
	"fmt"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 工单 WorkOrder（P2） ====================

type workOrderReq struct {
	SiteID      uint   `json:"site_id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
	AssetID     uint   `json:"asset_id"`
	CabinetID   uint   `json:"cabinet_id"`
	AssigneeID  uint   `json:"assignee_id"`
}

// genCode 生成单号 前缀-yyyyMMdd-NNN（按当日计数，按前缀选对应表）
func (h *Handler) genCode(prefix string) string {
	today := time.Now().Format("20060102")
	table := "t_work_order"
	switch prefix {
	case "CHG":
		table = "t_change_request"
	case "ST":
		table = "t_stocktake_task"
	case "AST":
		table = "t_asset"
	case "PR":
		table = "t_purchase_request"
	case "EP": // V59 应急预案
		table = "t_emergency_plan"
	case "RT": // V60 资产退役
		table = "t_asset_retirement"
	case "IR":
		return h.genReportCode()
	}
	var cnt int64
	h.DB.Table(table).Where("code LIKE ?", prefix+"-"+today+"%").Count(&cnt)
	return fmt.Sprintf("%s-%s-%03d", prefix, today, cnt+1)
}

// genReportCode V19 巡检报告编号 IR-yyyyMMdd-NNN（列名是 report_code 不是 code）
func (h *Handler) genReportCode() string {
	today := time.Now().Format("20060102")
	var cnt int64
	h.DB.Table("t_inspection_record").Where("report_code LIKE ?", "IR-"+today+"%").Count(&cnt)
	return fmt.Sprintf("IR-%s-%03d", today, cnt+1)
}

func (h *Handler) ListWorkOrders(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.WorkOrder{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if v := c.Query("id"); v != "" {
		q = q.Where("id = ?", v)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("asset_id"); v != "" && v != "0" {
		q = q.Where("asset_id = ?", v)
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("type = ?", v)
	}
	// V18 SLA 筛选: breach=已违约 warn=预警中(未违约且剩余<20%) ok=正常
	switch c.Query("sla") {
	case "breach":
		q = q.Where("(sla_respond_breach = ? OR sla_resolve_breach = ?)", true, true)
	case "warn":
		q = q.Where("sla_respond_breach = ? AND sla_resolve_breach = ? AND (sla_respond_warned = ? OR sla_resolve_warned = ?)", false, false, true, true)
	case "ok":
		q = q.Where("sla_respond_breach = ? AND sla_resolve_breach = ? AND sla_respond_warned = ? AND sla_resolve_warned = ?", false, false, false, false)
	}
	if v := c.Query("priority"); v != "" {
		q = q.Where("priority = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		kw := "%" + v + "%"
		q = q.Where("title LIKE ? OR code LIKE ? OR description LIKE ?", kw, kw, kw)
	}
	var total int64
	q.Count(&total)
	var items []model.WorkOrder
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateWorkOrder(c *gin.Context) {
	var req workOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	if req.Title == "" || req.SiteID == 0 {
		response.BadRequest(c, "标题与站点必填")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Type == "" {
		req.Type = "task"
	}
	if req.Priority < 1 || req.Priority > 4 {
		req.Priority = 2
	}
	uidN := uint64(middleware.GetUserID(c))
	duty := h.v74DutyBrief(req.SiteID) // V74/B4-1 未指定处理人时默认当班人
	assigneeID := req.AssigneeID
	autoAssigned := false
	if assigneeID == 0 && duty.OnDuty {
		assigneeID = duty.UserID
		autoAssigned = true
	}
	wo := model.WorkOrder{
		SiteID: req.SiteID, Code: h.genCode("WO"), Title: req.Title,
		Type: req.Type, Priority: req.Priority, Status: 1,
		Description: req.Description, AssetID: req.AssetID, CabinetID: req.CabinetID,
		RequesterID: uint(uidN), AssigneeID: assigneeID, CreatedBy: uint(uidN),
	}
	wo.CreatedAt = time.Now()
	h.applySLA(&wo) // V18 计算 SLA 时限
	if err := h.DB.Create(&wo).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: uint(uidN), Username: usernameOf(c), Action: "create", Content: func() string {
		if autoAssigned {
			return "创建工单；按值班表自动指派当班人 " + duty.UserName + "（" + duty.ShiftLabel + "）"
		}
		return "创建工单"
	}()})
	// V18 派单外发通知
	assigneeName := ""
	if wo.AssigneeID != 0 {
		var au model.User
		if h.DB.Select("display_name").First(&au, wo.AssigneeID).Error == nil {
			assigneeName = au.DisplayName
		}
	}
	h.DispatchWebhook(model.EvWorkOrder, "新工单 "+wo.Code, fmt.Sprintf("%s（优先级%s%s）", wo.Title, priName(wo.Priority), map[bool]string{true: "，处理人 " + assigneeName, false: ""}[assigneeName != ""]), map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "site_id": wo.SiteID})
	auditDetail := wo.Title
	if autoAssigned {
		auditDetail = fmt.Sprintf("%s；按值班表自动指派 %s（%s）", wo.Title, duty.UserName, duty.ShiftLabel)
	}
	middleware.WriteAudit(h.DB, c, "create_work_order", wo.Code, auditDetail)
	response.Created(c, wo)
}

func (h *Handler) UpdateWorkOrder(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var wo model.WorkOrder
	if err := h.DB.First(&wo, id).Error; err != nil {
		response.NotFound(c, "工单不存在")
		return
	}
	if !middleware.InSiteScope(c, wo.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	var req workOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	wo.Title = req.Title
	if req.Type != "" {
		wo.Type = req.Type
	}
	if req.Priority >= 1 && req.Priority <= 4 {
		if req.Priority != wo.Priority {
			wo.Priority = req.Priority
			// V18 优先级变化 → 按新优先级重算时限并复位预警/违约标记
			wo.SlaRespondWarned, wo.SlaResolveWarned = false, false
			wo.SlaRespondBreach, wo.SlaResolveBreach = false, false
			h.applySLA(&wo)
		}
	}
	wo.Description = req.Description
	wo.AssetID = req.AssetID
	wo.CabinetID = req.CabinetID
	if req.AssigneeID != 0 {
		wo.AssigneeID = req.AssigneeID
	}
	h.DB.Save(&wo)
	middleware.WriteAudit(h.DB, c, "update_work_order", wo.Code, wo.Title)
	response.Success(c, wo)
}

// transitionWorkOrder 工单状态流转（受理/完成/关闭/取消）
func (h *Handler) transitionWorkOrder(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var wo model.WorkOrder
	if err := h.DB.First(&wo, id).Error; err != nil {
		response.NotFound(c, "工单不存在")
		return
	}
	if !middleware.InSiteScope(c, wo.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	action := c.Param("action")
	now := time.Now()
	uidN := uint64(middleware.GetUserID(c))

	var req struct {
		Resolution string `json:"resolution"`
		AssigneeID uint   `json:"assignee_id"`
		Note       string `json:"note"`
	}
	c.ShouldBindJSON(&req)

	logEntry := model.WorkOrderLog{OrderID: wo.ID, UserID: uint(uidN), Username: usernameOf(c)}
	switch action {
	case "accept":
		if wo.Status != 1 {
			response.BadRequest(c, "仅待受理工单可受理")
			return
		}
		wo.Status = 2
		wo.AcceptedAt = &now
		if wo.AssigneeID == 0 {
			wo.AssigneeID = uint(uidN)
		}
		logEntry.Action, logEntry.Content = "accept", "受理工单"
	case "finish":
		if wo.Status != 2 {
			response.BadRequest(c, "仅处理中工单可完成")
			return
		}
		wo.Status = 3
		wo.FinishedAt = &now
		wo.Resolution = req.Resolution
		logEntry.Action, logEntry.Content = "finish", "完成工单："+req.Resolution
		// 告警联动：工单完成时自动结束其关联的活跃告警
		h.resolveAlertsByWorkOrder(wo.ID, "关联工单 "+wo.Code+" 已完成，告警自动结束")
	case "close":
		if wo.Status == 4 || wo.Status == 5 {
			response.BadRequest(c, "工单已关闭/取消")
			return
		}
		wo.Status = 4
		logEntry.Action, logEntry.Content = "close", "关闭工单"
		h.resolveAlertsByWorkOrder(wo.ID, "关联工单 "+wo.Code+" 已关闭，告警自动结束")
	case "cancel":
		if wo.Status >= 3 {
			response.BadRequest(c, "已完成/关闭的工单不可取消")
			return
		}
		wo.Status = 5
		logEntry.Action, logEntry.Content = "cancel", "取消工单"
	case "reassign":
		rr := req
		if wo.Status != 1 && wo.Status != 2 {
			response.BadRequest(c, "仅待受理/处理中工单可改派")
			return
		}
		autoDuty := false
		if rr.AssigneeID == 0 {
			if b := h.v74DutyBrief(wo.SiteID); b.OnDuty {
				rr.AssigneeID = b.UserID
				autoDuty = true
			}
		}
		var u model.User
		if rr.AssigneeID == 0 || h.DB.First(&u, rr.AssigneeID).Error != nil {
			response.BadRequest(c, "改派目标用户无效（当前站点无当班人，请手动选择）")
			return
		}
		old := wo.AssigneeID
		wo.AssigneeID = rr.AssigneeID
		logEntry.Action = "reassign"
		logEntry.Content = fmt.Sprintf("改派: #%d → %s", old, u.DisplayName)
		if autoDuty {
			logEntry.Content += "（未指定目标，按值班表取当班人）"
		}
		if rr.Note != "" {
			logEntry.Content += "（" + rr.Note + "）"
		}
		// 通知被改派人
		h.DB.Create(&model.Notification{UserID: rr.AssigneeID, Kind: "work_order", Title: "工单改派通知", Content: "工单 " + wo.Code + "「" + wo.Title + "」已改派给您", RefType: "work_order", RefID: wo.ID})
		h.DispatchWebhook(model.EvWorkOrder, "工单改派 "+wo.Code, fmt.Sprintf("%s 已改派给 %s", wo.Title, u.DisplayName), map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "assignee_id": rr.AssigneeID})
	default:
		response.BadRequest(c, "未知操作")
		return
	}
	h.DB.Save(&wo)
	h.DB.Create(&logEntry)
	middleware.WriteAudit(h.DB, c, action+"_work_order", wo.Code, wo.Title)
	response.Success(c, wo)
}

func (h *Handler) GetWorkOrderLogs(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var logs []model.WorkOrderLog
	h.DB.Where("order_id = ?", id).Order("id ASC").Find(&logs)
	response.Success(c, logs)
}

func (h *Handler) DeleteWorkOrder(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var wo model.WorkOrder
	if err := h.DB.First(&wo, id).Error; err != nil {
		response.NotFound(c, "工单不存在")
		return
	}
	if !middleware.InSiteScope(c, wo.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if wo.Status == 2 || wo.Status == 3 {
		response.BadRequest(c, "处理中/已完成的工单不可删除")
		return
	}
	h.DB.Delete(&wo)
	h.DB.Where("order_id = ?", id).Delete(&model.WorkOrderLog{})
	middleware.WriteAudit(h.DB, c, "delete_work_order", wo.Code, wo.Title)
	response.Success(c, gin.H{"id": id})
}

// usernameOf 从上下文取用户名
func usernameOf(c *gin.Context) string {
	// GetCaller 返回 (username, ip)；此前误取第二返回值导致申请人/审批人/操作员存成 IP（V77 修复）
	name, _ := middleware.GetCaller(c)
	return name
}

// ==================== 巡检 Inspection（P2） ====================

type inspectionPlanReq struct {
	SiteID     uint   `json:"site_id"`
	RoomID     *uint  `json:"room_id"` // V19 关联房间（0=全站计划）
	Name       string `json:"name"`
	Freq       string `json:"freq"`
	Items      string `json:"items"`
	AssigneeID uint   `json:"assignee_id"`
	Status     int    `json:"status"`
}

func (h *Handler) ListInspectionPlans(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.InspectionPlan{})
	if v := c.Query("room_id"); v != "" && v != "0" {
		q = q.Where("room_id = ?", v)
	}
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	var total int64
	q.Count(&total)
	var items []model.InspectionPlan
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateInspectionPlan(c *gin.Context) {
	var req inspectionPlanReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.SiteID == 0 {
		response.BadRequest(c, "名称与站点必填")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Freq == "" {
		req.Freq = "daily"
	}
	uidN := uint64(middleware.GetUserID(c))
	p := model.InspectionPlan{
		SiteID: req.SiteID, Name: req.Name, Freq: req.Freq, Items: req.Items,
		AssigneeID: req.AssigneeID, Status: 1, CreatedBy: uint(uidN),
	}
	if req.RoomID != nil {
		p.RoomID = *req.RoomID
	}
	if req.Status != 0 {
		p.Status = req.Status
	}
	if err := h.DB.Create(&p).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_inspection_plan", p.Name, req.Freq)
	response.Created(c, p)
}

func (h *Handler) UpdateInspectionPlan(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var p model.InspectionPlan
	if err := h.DB.First(&p, id).Error; err != nil {
		response.NotFound(c, "巡检计划不存在")
		return
	}
	if !middleware.InSiteScope(c, p.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	var req inspectionPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Freq != "" {
		p.Freq = req.Freq
	}
	p.Items = req.Items
	if req.RoomID != nil {
		p.RoomID = *req.RoomID
	}
	if req.AssigneeID != 0 {
		p.AssigneeID = req.AssigneeID
	}
	if req.Status != 0 {
		p.Status = req.Status
	}
	h.DB.Save(&p)
	middleware.WriteAudit(h.DB, c, "update_inspection_plan", p.Name, "")
	response.Success(c, p)
}

func (h *Handler) DeleteInspectionPlan(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var p model.InspectionPlan
	if err := h.DB.First(&p, id).Error; err != nil {
		response.NotFound(c, "巡检计划不存在")
		return
	}
	if !middleware.InSiteScope(c, p.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	h.DB.Delete(&p)
	middleware.WriteAudit(h.DB, c, "delete_inspection_plan", p.Name, "")
	response.Success(c, gin.H{"id": id})
}

// SubmitInspection 提交巡检结果（可附带异常转工单）
func (h *Handler) SubmitInspection(c *gin.Context) {
	var req struct {
		PlanID   uint   `json:"plan_id"`
		Status   int    `json:"status"` // 1 正常 2 异常
		Result   string `json:"result"`
		Abnormal string `json:"abnormal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID == 0 {
		response.BadRequest(c, "巡检计划必填")
		return
	}
	var plan model.InspectionPlan
	if err := h.DB.First(&plan, req.PlanID).Error; err != nil {
		response.NotFound(c, "巡检计划不存在")
		return
	}
	if !middleware.InSiteScope(c, plan.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	uidN := uint64(middleware.GetUserID(c))
	rec := model.InspectionRecord{
		PlanID: plan.ID, SiteID: plan.SiteID, InspectorID: uint(uidN),
		Status: req.Status, Result: req.Result, Abnormal: req.Abnormal,
		InspectedAt: time.Now(),
	}
	// 异常自动转工单
	if req.Status == 2 && req.Abnormal != "" {
		wo := model.WorkOrder{
			SiteID: plan.SiteID, Code: h.genCode("WO"),
			Title:    "巡检异常：" + plan.Name,
			Type:     "fault", Priority: 3, Status: 1,
			Description: req.Abnormal, RequesterID: uint(uidN), CreatedBy: uint(uidN),
		}
		h.DB.Create(&wo)
		rec.WorkOrderID = wo.ID
	}
	h.DB.Create(&rec)
	h.LinkInspectionTask(&rec) // V62 核销当期巡检任务
	now := time.Now()
	plan.LastRunAt = &now
	h.DB.Save(&plan)
	middleware.WriteAudit(h.DB, c, "submit_inspection", plan.Name, fmt.Sprintf("status=%d", req.Status))
	response.Created(c, rec)
}

func (h *Handler) ListInspectionRecords(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.InspectionRecord{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	if v := c.Query("plan_id"); v != "" && v != "0" {
		q = q.Where("plan_id = ?", v)
	}
	var total int64
	q.Count(&total)
	var items []model.InspectionRecord
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}
