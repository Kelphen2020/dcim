package api

import (
	"encoding/json"
	"io"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 变更 ChangeRequest（P2） ====================

type changeReq struct {
	SiteID       uint   `json:"site_id"`
	Title        string `json:"title"`
	Risk         int    `json:"risk"`
	Description  string `json:"description"`
	Impact       string `json:"impact"`
	Plan         string `json:"plan"`
	RollbackPlan string `json:"rollback_plan"`
	WindowStart  string `json:"window_start"`
	WindowEnd    string `json:"window_end"`
	CabinetIDs   string `json:"cabinet_ids"`    // V17 关联机柜(逗号分隔)
	PowerNodeIDs string `json:"power_node_ids"` // V17 关联供电节点(逗号分隔)
	NetworkDeviceIDs string `json:"network_device_ids"` // B2-3 关联网络设备(逗号分隔)
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func (h *Handler) ListChanges(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.ChangeRequest{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		kw := "%" + v + "%"
		q = q.Where("title LIKE ? OR code LIKE ?", kw, kw)
	}
	var total int64
	q.Count(&total)
	var items []model.ChangeRequest
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateChange(c *gin.Context) {
	var req changeReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" || req.SiteID == 0 {
		response.BadRequest(c, "标题与站点必填")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Risk < 1 || req.Risk > 3 {
		req.Risk = 1
	}
	uidN := uint64(middleware.GetUserID(c))
	chg := model.ChangeRequest{
		SiteID: req.SiteID, Code: h.genCode("CHG"), Title: req.Title,
		Risk: req.Risk, Status: 1, // 草稿
		Description: req.Description, Impact: req.Impact, Plan: req.Plan,
		RollbackPlan: req.RollbackPlan,
		WindowStart:  parseTimePtr(req.WindowStart), WindowEnd: parseTimePtr(req.WindowEnd),
		CabinetIDs:   req.CabinetIDs, PowerNodeIDs: req.PowerNodeIDs,
		NetworkDeviceIDs: req.NetworkDeviceIDs,
		ApplicantID: uint(uidN), CreatedBy: uint(uidN),
	}
	if err := h.DB.Create(&chg).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_change", chg.Code, chg.Title)
	response.Created(c, chg)
}

func (h *Handler) UpdateChange(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var chg model.ChangeRequest
	if err := h.DB.First(&chg, id).Error; err != nil {
		response.NotFound(c, "变更不存在")
		return
	}
	if !middleware.InSiteScope(c, chg.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if chg.Status >= 3 && chg.Status != 4 {
		response.BadRequest(c, "已批准的变更不可编辑（驳回后可重新修改）")
		return
	}
	var req changeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	if req.Title != "" {
		chg.Title = req.Title
	}
	if req.Risk >= 1 && req.Risk <= 3 {
		chg.Risk = req.Risk
	}
	chg.Description = req.Description
	chg.Impact = req.Impact
	chg.Plan = req.Plan
	chg.RollbackPlan = req.RollbackPlan
	chg.CabinetIDs = req.CabinetIDs
	chg.PowerNodeIDs = req.PowerNodeIDs
	chg.NetworkDeviceIDs = req.NetworkDeviceIDs
	if req.WindowStart != "" {
		chg.WindowStart = parseTimePtr(req.WindowStart)
	}
	if req.WindowEnd != "" {
		chg.WindowEnd = parseTimePtr(req.WindowEnd)
	}
	h.DB.Save(&chg)
	middleware.WriteAudit(h.DB, c, "update_change", chg.Code, chg.Title)
	response.Success(c, chg)
}

// TransitionChange 变更流转：submit(提审) approve(批准) reject(驳回) execute(开始执行) finish(完成) rollback(回滚)
// 审批权限：站点管理员以上；执行/完成：值班员以上
func (h *Handler) TransitionChange(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var chg model.ChangeRequest
	if err := h.DB.First(&chg, id).Error; err != nil {
		response.NotFound(c, "变更不存在")
		return
	}
	if !middleware.InSiteScope(c, chg.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	action := c.Param("action")
	uidN := uint64(middleware.GetUserID(c))
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "global_admin" || roleStr == "site_admin"

	var req struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&req)
	now := time.Now()

	switch action {
	case "submit":
		if chg.Status != 1 && chg.Status != 4 {
			response.BadRequest(c, "仅草稿/已驳回变更可提审")
			return
		}
		chg.Status = 2
	case "approve", "reject":
		if !isAdmin {
			response.Forbidden(c, "仅管理员可审批变更")
			return
		}
		if chg.Status != 2 {
			response.BadRequest(c, "仅待审批变更可审批")
			return
		}
		chg.ApproverID = uint(uidN)
		chg.ApprovedAt = &now
		chg.ApproveNote = req.Note
		if action == "approve" {
			chg.Status = 3
		} else {
			chg.Status = 4
		}
	case "execute":
		if chg.Status != 3 {
			response.BadRequest(c, "仅已批准变更可执行")
			return
		}
		chg.Status = 5
		chg.ExecutorID = uint(uidN)
		// B2-3: 关联网络设备 → 执行动作自动触发配置备份（占位记录，真实配置内容留待 SNMP/SSH 采集通道）
		h.backupNetdevsForChange(&chg, uint(uidN))
	case "finish":
		if chg.Status != 5 {
			response.BadRequest(c, "仅执行中变更可完成")
			return
		}
		chg.Status = 6
		chg.FinishedAt = &now
		chg.FinishNote = req.Note
	case "rollback":
		if chg.Status != 5 && chg.Status != 6 {
			response.BadRequest(c, "仅执行中/已完成变更可回滚")
			return
		}
		chg.Status = 7
		chg.FinishedAt = &now
		chg.FinishNote = "回滚：" + req.Note
	default:
		response.BadRequest(c, "未知操作")
		return
	}
	h.DB.Save(&chg)
	middleware.WriteAudit(h.DB, c, action+"_change", chg.Code, chg.Title)
	response.Success(c, chg)
}

// parseCSVUint B2-3: 解析逗号分隔的 ID 串（容错：跳过非数字段）
func parseCSVUint(s string) []uint {
	ids := []uint{}
	cur := uint(0)
	has := false
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			cur = cur*10 + uint(ch-'0')
			has = true
		} else {
			if has {
				ids = append(ids, cur)
			}
			cur, has = 0, false
		}
	}
	if has {
		ids = append(ids, cur)
	}
	return ids
}

// backupNetdevsForChange B2-3: 变更执行时对关联网络设备自动生成配置备份（占位记录，真实配置内容留待 SNMP/SSH 采集通道接入）
func (h *Handler) backupNetdevsForChange(chg *model.ChangeRequest, executorID uint) int {
	ids := parseCSVUint(chg.NetworkDeviceIDs)
	if len(ids) == 0 {
		return 0
	}
	now := time.Now()
	var devs []model.NetworkDevice
	h.DB.Where("id IN ? AND site_id = ?", ids, chg.SiteID).Find(&devs)
	n := 0
	for _, d := range devs {
		content := "变更 " + chg.Code + "「" + chg.Title + "」执行时自动触发的配置备份（占位记录）。\n执行时间: " + now.Format("2006-01-02 15:04:05") + "\n设备: " + d.Name + "（" + d.Code + "）管理IP: " + d.MgmtIP + "\n执行人ID: " + strconv.Itoa(int(executorID)) + "\n说明: 变更前后应留存设备配置快照用于回退比对；实际配置内容待 SNMP/SSH 采集通道接入后自动填充。"
		bk := model.ConfigBackup{
			DeviceID:   d.ID,
			Filename:   "config_" + strconv.Itoa(int(d.ID)) + "_" + now.Format("20060102_150405") + "_change.txt",
			Content:    content,
			Size:       int64(len(content)),
			Version:    chg.Code,
			BackupType: "change",
			Status:     1,
			BackedUpAt: now,
		}
		if h.DB.Create(&bk).Error == nil {
			n++
		}
	}
	return n
}

// ChangeBackups B2-3: 变更详情——关联网络设备与配置备份记录
func (h *Handler) ChangeBackups(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var chg model.ChangeRequest
	if err := h.DB.First(&chg, id).Error; err != nil {
		response.NotFound(c, "变更不存在")
		return
	}
	if !middleware.InSiteScope(c, chg.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	devIDs := parseCSVUint(chg.NetworkDeviceIDs)
	devs := []model.NetworkDevice{}
	backups := []model.ConfigBackup{}
	if len(devIDs) > 0 {
		h.DB.Where("id IN ?", devIDs).Find(&devs)
		h.DB.Where("device_id IN ?", devIDs).Order("backed_up_at DESC").Limit(20).Find(&backups)
	}
	response.Success(c, gin.H{"change": chg, "network_devices": devs, "config_backups": backups})
}

func (h *Handler) DeleteChange(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var chg model.ChangeRequest
	if err := h.DB.First(&chg, id).Error; err != nil {
		response.NotFound(c, "变更不存在")
		return
	}
	if !middleware.InSiteScope(c, chg.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if chg.Status != 1 && chg.Status != 4 {
		response.BadRequest(c, "仅草稿/已驳回变更可删除")
		return
	}
	h.DB.Delete(&chg)
	middleware.WriteAudit(h.DB, c, "delete_change", chg.Code, chg.Title)
	response.Success(c, gin.H{"id": id})
}

// ==================== 供电链路 PowerChain（P2） ====================

type powerChainReq struct {
	SiteID    uint    `json:"site_id"`
	ParentID  uint    `json:"parent_id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Model     string  `json:"model"`
	RatedKVA  float64 `json:"rated_kva"`
	LoadKW    float64 `json:"load_kw"`
	Status    int     `json:"status"`
	AssetID   uint    `json:"asset_id"`
	PointID   uint    `json:"point_id"`
	CabinetID uint    `json:"cabinet_id"`
	Remark    string  `json:"remark"`
}

// ListPowerChains 返回整棵树（按站点过滤）
func (h *Handler) ListPowerChains(c *gin.Context) {
	q := h.DB.Model(&model.PowerChain{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	var items []model.PowerChain
	q.Order("id ASC").Find(&items)
	response.Success(c, items)
}

func (h *Handler) CreatePowerChain(c *gin.Context) {
	var req powerChainReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.SiteID == 0 || req.Kind == "" {
		response.BadRequest(c, "名称/站点/类型必填")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	// 父节点校验：必须存在且同站点
	if req.ParentID != 0 {
		var parent model.PowerChain
		if err := h.DB.First(&parent, req.ParentID).Error; err != nil || parent.SiteID != req.SiteID {
			response.BadRequest(c, "上级节点不存在或跨站点")
			return
		}
	}
	uidN := uint64(middleware.GetUserID(c))
	pc := model.PowerChain{
		SiteID: req.SiteID, ParentID: req.ParentID, Name: req.Name, Kind: req.Kind,
		Model: req.Model, RatedKVA: req.RatedKVA, LoadKW: req.LoadKW,
		Status: 1, AssetID: req.AssetID, PointID: req.PointID, CabinetID: req.CabinetID,
		Remark: req.Remark, CreatedBy: uint(uidN),
	}
	if req.Status != 0 {
		pc.Status = req.Status
	}
	if err := h.DB.Create(&pc).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_power_chain", pc.Name, pc.Kind)
	response.Created(c, pc)
}

func (h *Handler) UpdatePowerChain(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var pc model.PowerChain
	if err := h.DB.First(&pc, id).Error; err != nil {
		response.NotFound(c, "供电节点不存在")
		return
	}
	if !middleware.InSiteScope(c, pc.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	// V54 修复：局部 PUT 键探测（陷阱34族）——io.ReadAll 必须在 ShouldBindJSON 之前，
	// 只有请求体显式携带的键才更新，未传字段保持原值（旧实现把 rated_kva/parent_id 等清零）
	raw, _ := io.ReadAll(c.Request.Body)
	var req powerChainReq
	if err := json.Unmarshal(raw, &req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	var rawMap map[string]interface{}
	_ = json.Unmarshal(raw, &rawMap)
	has := func(k string) bool { _, ok := rawMap[k]; return ok }
	if req.Name != "" {
		pc.Name = req.Name
	}
	if req.Kind != "" {
		pc.Kind = req.Kind
	}
	if has("model") {
		pc.Model = req.Model
	}
	if has("rated_kva") {
		pc.RatedKVA = req.RatedKVA
	}
	if has("load_kw") {
		pc.LoadKW = req.LoadKW
	}
	if req.Status != 0 {
		pc.Status = req.Status
	}
	if has("asset_id") {
		pc.AssetID = req.AssetID
	}
	if has("point_id") {
		pc.PointID = req.PointID
	}
	if has("cabinet_id") {
		pc.CabinetID = req.CabinetID
	}
	if has("remark") {
		pc.Remark = req.Remark
	}
	// 移动父节点：防环检查（显式携带 parent_id 才处理）
	if has("parent_id") && req.ParentID != pc.ParentID {
		if req.ParentID != 0 {
			// 不能挂到自己或自己的子孙下
			if req.ParentID == pc.ID || h.isDescendant(req.ParentID, pc.ID) {
				response.BadRequest(c, "不能挂载到自身或下级节点")
				return
			}
			var parent model.PowerChain
			if err := h.DB.First(&parent, req.ParentID).Error; err != nil || parent.SiteID != pc.SiteID {
				response.BadRequest(c, "上级节点不存在或跨站点")
				return
			}
		}
		pc.ParentID = req.ParentID
	}
	h.DB.Save(&pc)
	middleware.WriteAudit(h.DB, c, "update_power_chain", pc.Name, pc.Kind)
	response.Success(c, pc)
}

// isDescendant nodeID 是否是 ancestorID 的子孙
func (h *Handler) isDescendant(nodeID, ancestorID uint) bool {
	cur := nodeID
	for i := 0; i < 50 && cur != 0; i++ {
		var n model.PowerChain
		if err := h.DB.First(&n, cur).Error; err != nil {
			return false
		}
		if n.ParentID == ancestorID {
			return true
		}
		cur = n.ParentID
	}
	return false
}

func (h *Handler) DeletePowerChain(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var pc model.PowerChain
	if err := h.DB.First(&pc, id).Error; err != nil {
		response.NotFound(c, "供电节点不存在")
		return
	}
	if !middleware.InSiteScope(c, pc.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	var childCnt int64
	h.DB.Model(&model.PowerChain{}).Where("parent_id = ?", id).Count(&childCnt)
	if childCnt > 0 {
		response.BadRequest(c, "存在下级节点，请先删除下级")
		return
	}
	h.DB.Delete(&pc)
	middleware.WriteAudit(h.DB, c, "delete_power_chain", pc.Name, pc.Kind)
	response.Success(c, gin.H{"id": id})
}

// ==================== 通知中心 Notification（P2） ====================

// ListNotifications 我的通知（个人 + 全员广播）
func (h *Handler) ListNotifications(c *gin.Context) {
	page, size := h.paging(c)
	uidN := uint64(middleware.GetUserID(c))
	q := h.DB.Model(&model.Notification{}).Where("user_id = ? OR user_id = 0", uidN)
	if c.Query("unread") == "1" {
		q = q.Where("read = false")
	}
	var total int64
	q.Count(&total)
	var items []model.Notification
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) UnreadCount(c *gin.Context) {
	uidN := uint64(middleware.GetUserID(c))
	var n int64
	h.DB.Model(&model.Notification{}).Where("(user_id = ? OR user_id = 0) AND read = false", uidN).Count(&n)
	response.Success(c, gin.H{"unread": n})
}

// ReadNotifications 标记已读（id=0 表示全部）
func (h *Handler) ReadNotifications(c *gin.Context) {
	uidN := uint64(middleware.GetUserID(c))
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	q := h.DB.Model(&model.Notification{}).Where("(user_id = ? OR user_id = 0) AND read = false", uidN)
	if id != 0 {
		q = q.Where("id = ?", id)
	}
	q.Update("read", true)
	response.Success(c, gin.H{"ok": true})
}

// BroadcastNotification 管理员发全员通知
func (h *Handler) BroadcastNotification(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		response.BadRequest(c, "标题必填")
		return
	}
	n := model.Notification{UserID: 0, Kind: "system", Title: req.Title, Content: req.Content}
	h.DB.Create(&n)
	middleware.WriteAudit(h.DB, c, "broadcast_notification", req.Title, "")
	response.Created(c, n)
}

// CheckMaintenanceDue 维保到期检查（启动时与每日调用；30 天内到期生成提醒）
// 幂等：同一资产同一合同只提醒一次
func (h *Handler) CheckMaintenanceDue() int {
	var dues []model.Maintenance
	deadline := time.Now().AddDate(0, 0, 30)
	h.DB.Where("kind = 'contract' AND end_at IS NOT NULL AND end_at <= ? AND end_at >= ?", deadline, time.Now().AddDate(0, 0, -7)).Find(&dues)
	created := 0
	for _, m := range dues {
		var cnt int64
		h.DB.Model(&model.Notification{}).Where("kind = 'maintenance_due' AND ref_type = 'maintenance' AND ref_id = ?", m.ID).Count(&cnt)
		if cnt > 0 {
			continue
		}
		var asset model.Asset
		h.DB.First(&asset, m.AssetID)
		days := int(time.Until(*m.EndAt).Hours() / 24)
		title := "维保即将到期：" + asset.Name
		h.DB.Create(&model.Notification{
			UserID: 0, Kind: "maintenance_due", Title: title,
			Content: m.Vendor + " 维保合同 " + m.Contract + " 将于 " + m.EndAt.Format("2006-01-02") + " 到期（剩余 " + strconv.Itoa(days) + " 天），请及时续签。",
			RefType: "maintenance", RefID: m.ID,
		})
		created++
	}
	return created
}
