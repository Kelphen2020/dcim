package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V60 资产退役与处置流程 ============
// 闭环: 退役申请(人工发起/盘点盘亏一键转办) → 管理员审批(批准→资产自动下架+置报废) →
//       处置登记(方式/残值/凭证编号/数据擦除确认/见证人, 未确认数据擦除不得完成处置) → 留痕与统计
// 与既有体系联动: 通知中心(kind=asset_retirement) + Webhook(EvAssetRetire) + 审计 + 盘点(盘亏转办) + 资产台账(状态机)

var retireReasons = map[string]string{
	"age": "服役到期", "fault": "故障不可修复", "loss": "盘点盘亏", "upgrade": "技术更新淘汰", "other": "其他",
}

var disposalMethods = map[string]string{
	"resale": "转售", "scrap": "报废销毁", "takeback": "厂商回收", "donate": "捐赠", "other": "其他",
}

func retireReasonLabel(r string) string {
	if v, ok := retireReasons[r]; ok {
		return v
	}
	return r
}

func disposalMethodLabel(m string) string {
	if v, ok := disposalMethods[m]; ok {
		return v
	}
	return m
}

func retireStatusLabel(s int) string {
	switch s {
	case 1:
		return "待审批"
	case 2:
		return "已批准待处置"
	case 3:
		return "已驳回"
	case 4:
		return "处置完成"
	case 5:
		return "已撤销"
	}
	return "未知"
}

// v60AssetInfo 批量取资产摘要
func (h *Handler) v60AssetInfo(ids []uint) map[uint]model.Asset {
	out := map[uint]model.Asset{}
	if len(ids) == 0 {
		return out
	}
	var assets []model.Asset
	h.DB.Where("id IN ?", ids).Find(&assets)
	for _, a := range assets {
		out[a.ID] = a
	}
	return out
}

// ---------- 列表 ----------

// ListAssetRetirements GET /asset-retirements?status=&reason=&site_id=&keyword=&page=&size=
func (h *Handler) ListAssetRetirements(c *gin.Context) {
	q := h.DB.Model(&model.AssetRetirement{})
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("reason"); v != "" {
		q = q.Where("reason = ?", v)
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	if v := c.Query("asset_id"); v != "" && v != "0" {
		q = q.Where("asset_id = ?", v)
	}
	var total int64
	q.Count(&total)
	page, size := h.paging(c)
	var items []model.AssetRetirement
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.AssetRetirement{}
	}

	ids := make([]uint, 0, len(items))
	for _, r := range items {
		ids = append(ids, r.AssetID)
	}
	assets := h.v60AssetInfo(ids)
	names := h.v59UserNames()
	// V69/B2-4: 关联网络设备提示（审批前展示联动范围）
	ndByAsset := map[uint][]string{}
	if len(ids) > 0 {
		var nds []model.NetworkDevice
		h.DB.Where("asset_id IN ?", ids).Find(&nds)
		for _, nd := range nds {
			if nd.AssetID != nil { ndByAsset[*nd.AssetID] = append(ndByAsset[*nd.AssetID], nd.Name) }
		}
	}

	type row struct {
		model.AssetRetirement
		AssetCode     string `json:"asset_code"`
		AssetName     string `json:"asset_name"`
		AssetCategory string `json:"asset_category"`
		ApplicantName string `json:"applicant_name"`
		ApproverName  string `json:"approver_name"`
		DisposalByName string `json:"disposal_by_name"`
		NetdevNames   string `json:"netdev_names"`
	}
	out := make([]row, 0, len(items))
	for _, r := range items {
		a := assets[r.AssetID]
		out = append(out, row{AssetRetirement: r, AssetCode: a.Code, AssetName: a.Name,
			AssetCategory: a.Category, ApplicantName: names[r.ApplicantID],
			ApproverName: names[r.ApproverID], DisposalByName: names[r.DisposalBy],
			NetdevNames: strings.Join(ndByAsset[r.AssetID], "、")})
	}
	// keyword 过滤（资产编码/名称/单号/凭证号，列表页内存过滤，量级小）
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		filtered := out[:0]
		for _, r := range out {
			if strings.Contains(r.Code, kw) || strings.Contains(r.AssetCode, kw) ||
				strings.Contains(r.AssetName, kw) || strings.Contains(r.CertificateNo, kw) {
				filtered = append(filtered, r)
			}
		}
		out = filtered
	}

	// summary：待审批/待处置/本月完成处置/本月残值回收
	var pending, pendDisp int64
	h.DB.Model(&model.AssetRetirement{}).Where("status = 1").Count(&pending)
	h.DB.Model(&model.AssetRetirement{}).Where("status = 2").Count(&pendDisp)
	monthStart := time.Now().Format("2006-01") + "-01"
	var monthDone int64
	var monthAmount float64
	h.DB.Model(&model.AssetRetirement{}).Where("status = 4 AND disposal_at >= ?", monthStart).Count(&monthDone)
	h.DB.Model(&model.AssetRetirement{}).Where("status = 4 AND disposal_at >= ?", monthStart).
		Select("COALESCE(SUM(disposal_amount),0)").Scan(&monthAmount)
	// 按原因分布
	type reasonRow struct {
		Reason string `json:"reason"`
		N      int    `json:"n"`
	}
	var byReason []reasonRow
	h.DB.Model(&model.AssetRetirement{}).Select("reason, COUNT(*) as n").Group("reason").Scan(&byReason)

	response.Success(c, gin.H{
		"items": out, "total": total, "page": page, "size": size,
		"summary": gin.H{"pending": pending, "pending_disposal": pendDisp,
			"month_done": monthDone, "month_amount": monthAmount, "by_reason": byReason},
	})
}

// GetAssetRetirement GET /asset-retirements/:id 详情（资产快照+时间线）
func (h *Handler) GetAssetRetirement(c *gin.Context) {
	var r model.AssetRetirement
	if err := h.DB.First(&r, c.Param("id")).Error; err != nil {
		response.NotFound(c, "退役申请不存在")
		return
	}
	var a model.Asset
	h.DB.First(&a, r.AssetID)
	names := h.v59UserNames()
	siteName := ""
	if r.SiteID > 0 {
		var s model.Site
		if h.DB.Select("name").First(&s, r.SiteID).Error == nil {
			siteName = s.Name
		}
	}
	type tlItem struct {
		Label string `json:"label"`
		Time  string `json:"time"`
		Who   string `json:"who"`
		Note  string `json:"note"`
		Done  bool   `json:"done"`
	}
	tl := []tlItem{
		{Label: "提交申请", Time: r.AppliedAt.Format("2006-01-02 15:04"), Who: names[r.ApplicantID], Note: retireReasonLabel(r.Reason), Done: true},
	}
	if r.ApprovedAt != nil {
		tl = append(tl, tlItem{Label: map[bool]string{true: "审批通过", false: "审批驳回"}[r.Status != 3],
			Time: r.ApprovedAt.Format("2006-01-02 15:04"), Who: names[r.ApproverID], Note: r.ApproveNote, Done: true})
	} else if r.Status == 1 {
		tl = append(tl, tlItem{Label: "待审批", Done: false})
	}
	if r.DisposalAt != nil {
		tl = append(tl, tlItem{Label: "处置完成", Time: r.DisposalAt.Format("2006-01-02 15:04"),
			Who: names[r.DisposalBy], Note: disposalMethodLabel(r.DisposalMethod), Done: true})
	} else if r.Status == 2 {
		tl = append(tl, tlItem{Label: "待处置登记", Done: false})
	}
	response.Success(c, gin.H{
		"retirement": r, "asset": a, "site_name": siteName,
		"applicant_name": names[r.ApplicantID], "approver_name": names[r.ApproverID],
		"disposal_by_name": names[r.DisposalBy],
		"reason_label": retireReasonLabel(r.Reason), "status_label": retireStatusLabel(r.Status),
		"timeline": tl,
	})
}

// ---------- 申请 ----------

// CreateAssetRetirement POST /asset-retirements（ops+）
func (h *Handler) CreateAssetRetirement(c *gin.Context) {
	var req struct {
		AssetID     uint   `json:"asset_id"`
		Reason      string `json:"reason"`
		Description string `json:"description"`
		SourceType  string `json:"source_type"`
		SourceID    uint   `json:"source_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AssetID == 0 {
		response.BadRequest(c, "须指定资产")
		return
	}
	if _, ok := retireReasons[req.Reason]; !ok {
		response.BadRequest(c, "退役原因不合法（age/fault/loss/upgrade/other）")
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		response.BadRequest(c, "请填写退役说明（处置留痕是检查材料）")
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, req.AssetID).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	if a.Status == 4 {
		response.BadRequest(c, "资产已是报废状态，无需再走退役流程")
		return
	}
	r, err := h.createRetirement(c, &a, req.Reason, req.Description, req.SourceType, req.SourceID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, r)
}

// createRetirement 建单公共逻辑（人工/盘亏转办共用）
func (h *Handler) createRetirement(c *gin.Context, a *model.Asset, reason, desc, srcType string, srcID uint) (*model.AssetRetirement, error) {
	var openCnt int64
	h.DB.Model(&model.AssetRetirement{}).Where("asset_id = ? AND status IN (1,2)", a.ID).Count(&openCnt)
	if openCnt > 0 {
		return nil, fmt.Errorf("资产 %s 已有进行中的退役申请（待审批或待处置），不能重复发起", a.Code)
	}
	uid := middleware.GetUserID(c)
	r := model.AssetRetirement{
		Code: h.genCode("RT"), AssetID: a.ID, SiteID: a.SiteID,
		Reason: reason, Description: desc, SourceType: srcType, SourceID: srcID,
		Status: 1, ApplicantID: uid, AppliedAt: time.Now(),
	}
	if err := h.DB.Create(&r).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %v", err)
	}
	go middleware.WriteAudit(h.DB, c, "create", "asset_retirement", r.Code+" 资产="+a.Code+" 原因="+retireReasonLabel(reason))
	// 通知管理员审批（广播）
	msg := fmt.Sprintf("资产 %s（%s）申请退役，原因：%s。请管理员尽快审批。", a.Code, a.Name, retireReasonLabel(reason))
	h.DB.Create(&model.Notification{
		UserID: 0, Kind: "asset_retirement", Title: "资产退役待审批 " + r.Code,
		Content: msg, RefType: "asset_retirement", RefID: r.ID,
	})
	h.DispatchWebhook(model.EvAssetRetire, "资产退役申请 "+r.Code, msg,
		map[string]interface{}{"retirement_id": r.ID, "asset_code": a.Code, "reason": reason, "stage": "apply"})
	return &r, nil
}

// CreateRetirementFromStocktakeItem POST /asset-retirements/from-stocktake-item/:id（ops+）
// 盘点盘亏项一键转退役申请（reason=loss，来源留痕 stocktake_item）
func (h *Handler) CreateRetirementFromStocktakeItem(c *gin.Context) {
	var item model.StocktakeItem
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		response.NotFound(c, "盘点明细不存在")
		return
	}
	if item.Result != 2 {
		response.BadRequest(c, "仅「盘亏」明细可转退役申请")
		return
	}
	if item.AssetID == 0 {
		response.BadRequest(c, "该明细未关联资产")
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, item.AssetID).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	var task model.StocktakeTask
	h.DB.Select("code").First(&task, item.TaskID)
	desc := fmt.Sprintf("盘点任务 %s 核对发现盘亏（原位置 %s U%d），账实不符，申请退役核销。%s",
		task.Code, item.CabinetCode, item.StartU, item.Remark)
	r, err := h.createRetirement(c, &a, "loss", strings.TrimSpace(desc), "stocktake", item.ID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, r)
}

// ---------- 审批 ----------

// ApproveAssetRetirement POST /asset-retirements/:id/approve（admin）{approve: bool, note}
// 批准 → 资产自动下架（如在架）并置报废态（终态），释放 U 位/功率容量
func (h *Handler) ApproveAssetRetirement(c *gin.Context) {
	var r model.AssetRetirement
	if err := h.DB.First(&r, c.Param("id")).Error; err != nil {
		response.NotFound(c, "退役申请不存在")
		return
	}
	if r.Status != 1 {
		response.BadRequest(c, "当前状态（"+retireStatusLabel(r.Status)+"）不可审批，仅待审批单可操作")
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	now := time.Now()
	r.ApproverID = middleware.GetUserID(c)
	r.ApprovedAt = &now
	r.ApproveNote = strings.TrimSpace(req.Note)
	var a model.Asset
	h.DB.First(&a, r.AssetID)
	var netdevNames []string
	if req.Approve {
		r.Status = 2
		// 资产联动：在架先自动下架（释放容量），再置报废终态
		if a.CabinetID > 0 {
			var cabCode string
			h.DB.Model(&model.Cabinet{}).Where("id = ?", a.CabinetID).Pluck("code", &cabCode)
			a.CabinetID = 0
			a.StartU = 0
			go middleware.WriteAudit(h.DB, c, "unmount_asset", a.Code, "退役批准自动下架，原机柜="+cabCode)
		}
		a.Status = 4
		h.DB.Save(&a)
		// V69/B2-4: 资产退役联动——关联网络设备自动置离线并留痕
		netdevNames = h.netdevRetireLinkage(c, a.ID, a.Code)
	} else {
		r.Status = 3
	}
	if err := h.DB.Save(&r).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "审批失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, map[bool]string{true: "approve", false: "reject"}[req.Approve],
		"asset_retirement", r.Code+" 资产="+a.Code)
	// 通知申请人
	verb := map[bool]string{true: "已批准", false: "已驳回"}[req.Approve]
	msg := fmt.Sprintf("退役申请 %s（资产 %s %s）%s。", r.Code, a.Code, a.Name, verb)
	if req.Approve {
		msg += "资产已置为报废状态，请在处置完成后登记处置信息（方式/残值/凭证/数据擦除确认）。"
	}
	if len(netdevNames) > 0 {
		msg += " 关联网络设备已自动置离线：" + strings.Join(netdevNames, "、") + "，如需继续使用请在网络设备页恢复状态或解绑资产。"
	}
	if r.ApproveNote != "" {
		msg += "审批意见：" + r.ApproveNote
	}
	h.DB.Create(&model.Notification{
		UserID: r.ApplicantID, Kind: "asset_retirement", Title: "退役申请" + verb + " " + r.Code,
		Content: msg, RefType: "asset_retirement", RefID: r.ID,
	})
	h.DispatchWebhook(model.EvAssetRetire, "资产退役"+verb+" "+r.Code, msg,
		map[string]interface{}{"retirement_id": r.ID, "asset_code": a.Code,
			"stage": map[bool]string{true: "approve", false: "reject"}[req.Approve]})
	response.Success(c, r)
}

// ---------- 处置登记 ----------

// DisposeAssetRetirement POST /asset-retirements/:id/disposal（ops+）
func (h *Handler) DisposeAssetRetirement(c *gin.Context) {
	var r model.AssetRetirement
	if err := h.DB.First(&r, c.Param("id")).Error; err != nil {
		response.NotFound(c, "退役申请不存在")
		return
	}
	if r.Status != 2 {
		response.BadRequest(c, "当前状态（"+retireStatusLabel(r.Status)+"）不可登记处置，仅已批准待处置单可操作")
		return
	}
	var req struct {
		Method        string  `json:"method"`
		Date          string  `json:"date"` // yyyy-MM-dd，空=今天
		Amount        float64 `json:"amount"`
		CertificateNo string  `json:"certificate_no"`
		DataWiped     bool    `json:"data_wiped"`
		Witness       string  `json:"witness"`
		Note          string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if _, ok := disposalMethods[req.Method]; !ok {
		response.BadRequest(c, "处置方式不合法（resale/scrap/takeback/donate/other）")
		return
	}
	// 合规硬约束：存储介质数据擦除必须确认（军队/等保检查项）
	if !req.DataWiped {
		response.BadRequest(c, "必须确认数据已擦除/介质已销毁才能完成处置（保密合规要求）")
		return
	}
	if req.Amount < 0 {
		response.BadRequest(c, "残值回收金额不能为负")
		return
	}
	dispDate := time.Now()
	if req.Date != "" {
		if t, err := time.ParseInLocation("2006-01-02", req.Date, time.Local); err == nil {
			dispDate = t.Add(10*time.Hour + 30*time.Minute)
		} else {
			response.BadRequest(c, "处置日期格式应为 yyyy-MM-dd")
			return
		}
	}
	now := time.Now()
	r.Status = 4
	r.DisposalMethod = req.Method
	r.DisposalDate = &dispDate
	r.DisposalAmount = req.Amount
	r.CertificateNo = strings.TrimSpace(req.CertificateNo)
	r.DataWiped = true
	r.Witness = strings.TrimSpace(req.Witness)
	r.DisposalNote = strings.TrimSpace(req.Note)
	r.DisposalBy = middleware.GetUserID(c)
	r.DisposalAt = &now
	if err := h.DB.Save(&r).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "登记失败: "+err.Error())
		return
	}
	var a model.Asset
	h.DB.First(&a, r.AssetID)
	go middleware.WriteAudit(h.DB, c, "create", "asset_disposal",
		fmt.Sprintf("%s 资产=%s 方式=%s 残值=%.2f 凭证=%s", r.Code, a.Code, disposalMethodLabel(req.Method), req.Amount, r.CertificateNo))
	msg := fmt.Sprintf("退役申请 %s（资产 %s %s）处置完成：%s，残值回收 %.2f 元，数据擦除已确认。",
		r.Code, a.Code, a.Name, disposalMethodLabel(req.Method), req.Amount)
	h.DB.Create(&model.Notification{
		UserID: r.ApplicantID, Kind: "asset_retirement", Title: "退役处置完成 " + r.Code,
		Content: msg, RefType: "asset_retirement", RefID: r.ID,
	})
	h.DispatchWebhook(model.EvAssetRetire, "资产退役处置完成 "+r.Code, msg,
		map[string]interface{}{"retirement_id": r.ID, "asset_code": a.Code, "stage": "disposal",
			"method": req.Method, "amount": req.Amount})
	response.Success(c, r)
}

// ---------- 撤销 / 统计 ----------

// CancelAssetRetirement POST /asset-retirements/:id/cancel（申请人本人或管理员）
func (h *Handler) CancelAssetRetirement(c *gin.Context) {
	var r model.AssetRetirement
	if err := h.DB.First(&r, c.Param("id")).Error; err != nil {
		response.NotFound(c, "退役申请不存在")
		return
	}
	if r.Status != 1 {
		response.BadRequest(c, "仅待审批的申请可撤销")
		return
	}
	uid := middleware.GetUserID(c)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if uid != r.ApplicantID && roleStr != "global_admin" && roleStr != "site_admin" {
		response.Forbidden(c, "只有申请人本人或管理员可撤销")
		return
	}
	r.Status = 5
	h.DB.Model(&r).Update("status", 5)
	go middleware.WriteAudit(h.DB, c, "cancel", "asset_retirement", r.Code)
	response.Success(c, gin.H{"id": r.ID, "status": 5})
}

// RetirementStats GET /asset-retirements/stats 近6个月处置统计+原因分布+残值汇总
func (h *Handler) RetirementStats(c *gin.Context) {
	type monthRow struct {
		Month  string  `json:"month"`
		N      int     `json:"n"`
		Amount float64 `json:"amount"`
	}
	var months []monthRow
	h.DB.Model(&model.AssetRetirement{}).
		Select("to_char(date_trunc('month', disposal_at), 'YYYY-MM') as month, COUNT(*) as n, COALESCE(SUM(disposal_amount),0) as amount").
		Where("status = 4 AND disposal_at >= ?", time.Now().AddDate(0, -6, 0)).
		Group("month").Order("month").Scan(&months)
	// 补零
	byM := map[string]monthRow{}
	for _, m := range months {
		byM[m.Month] = m
	}
	full := make([]monthRow, 0, 7)
	cur := time.Now().AddDate(0, -6, 0)
	for i := 0; i < 7; i++ {
		k := cur.Format("2006-01")
		if m, ok := byM[k]; ok {
			full = append(full, m)
		} else {
			full = append(full, monthRow{Month: k})
		}
		cur = cur.AddDate(0, 1, 0)
	}
	type reasonRow struct {
		Reason string `json:"reason"`
		Label  string `json:"label"`
		N      int    `json:"n"`
	}
	var reasons []reasonRow
	h.DB.Model(&model.AssetRetirement{}).Select("reason, COUNT(*) as n").Group("reason").Scan(&reasons)
	for i := range reasons {
		reasons[i].Label = retireReasonLabel(reasons[i].Reason)
	}
	var totalAmount float64
	h.DB.Model(&model.AssetRetirement{}).Where("status = 4").Select("COALESCE(SUM(disposal_amount),0)").Scan(&totalAmount)
	var doneCnt int64
	h.DB.Model(&model.AssetRetirement{}).Where("status = 4").Count(&doneCnt)
	response.Success(c, gin.H{
		"months": full, "by_reason": reasons,
		"total_done": doneCnt, "total_amount": totalAmount,
	})
}
