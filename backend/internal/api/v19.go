package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V19 扫码巡检 + 巡检报告签发 ====================
// 二维码内容: DCIM-SCAN|<target_type>|<target_id>|<exp>|<hmac>
//   hmac = HMAC-SHA256(JWTSecret, "DCIM-SCAN|type|id|exp")
// 流程: 手机扫码 → POST /inspection-scan/checkin {token} → 服务端验签+验过期
//   → 落 t_inspection_checkin(checked_in_at=服务端此刻, used=false) → 返回目标信息+匹配巡检计划+检查项实时值
//   → 执行提交 POST /inspection-scan/submit {checkin_id, ...} → 校验签到未使用/新鲜度(默认120分钟)
//   → 写 t_inspection_record(source=scan, inspected_at=签到时间(防补录), 测点快照, 报告编号)
// 报告: GET /inspection-reports/:id → 完整签发数据；POST /inspection-reports/:id/sign → 审核签字

// signScanToken 生成/校验二维码令牌
func (h *Handler) signScanToken(targetType string, targetID uint, exp int64) string {
	msg := fmt.Sprintf("DCIM-SCAN|%s|%d|%d", targetType, targetID, exp)
	mac := hmac.New(sha256.New, []byte(h.JWTSecret))
	mac.Write([]byte(msg))
	return msg + "|" + hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) verifyScanToken(token string) (targetType string, targetID uint, exp int64, err error) {
	parts := strings.Split(token, "|")
	if len(parts) != 5 || parts[0] != "DCIM-SCAN" {
		err = fmt.Errorf("令牌格式无效")
		return
	}
	msg := strings.Join(parts[:4], "|")
	mac := hmac.New(sha256.New, []byte(h.JWTSecret))
	mac.Write([]byte(msg))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(parts[4])) {
		err = fmt.Errorf("令牌签名无效")
		return
	}
	id64, e1 := strconv.ParseUint(parts[2], 10, 64)
	expV, e2 := strconv.ParseInt(parts[3], 10, 64)
	exp = expV
	if e1 != nil || e2 != nil {
		err = fmt.Errorf("令牌字段解析失败")
		return
	}
	targetType, targetID = parts[1], uint(id64)
	if exp < time.Now().Unix() {
		err = fmt.Errorf("二维码已过期，请重新生成")
		return
	}
	return
}

// GenScanQR 生成巡检二维码数据: GET /inspection-scan/qr?type=cabinet&id=12&days=365
// 返回 {token, url}（前端用 qrcode 库渲染成图；url 供直接跳转 H5）
func (h *Handler) GenScanQR(c *gin.Context) {
	ttype := c.DefaultQuery("type", "cabinet")
	if ttype != "cabinet" && ttype != "room" {
		response.BadRequest(c, "type 仅支持 cabinet/room")
		return
	}
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if id == 0 {
		response.BadRequest(c, "缺少目标 id")
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "365"))
	if days <= 0 || days > 3650 {
		days = 365
	}
	// 校验目标存在且取名称
	var name string
	var siteID uint
	if ttype == "cabinet" {
		var cab model.Cabinet
		if h.DB.First(&cab, id).Error != nil {
			response.NotFound(c, "机柜不存在")
			return
		}
		name, siteID = cab.Code, cab.SiteID
	} else {
		var room model.Room
		if h.DB.First(&room, id).Error != nil {
			response.NotFound(c, "房间不存在")
			return
		}
		name, siteID = room.Name, room.SiteID
	}
	if !middleware.InSiteScope(c, siteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	exp := time.Now().AddDate(0, 0, days).Unix()
	token := h.signScanToken(ttype, uint(id), exp)
	response.Success(c, gin.H{
		"token":      token,
		"url":        fmt.Sprintf("/#!/inspection-scan?%s=%s", "token", token),
		"target":     gin.H{"type": ttype, "id": id, "name": name, "site_id": siteID},
		"expires_at": time.Unix(exp, 0).Format("2006-01-02"),
	})
}

// BatchScanQR 批量生成（整排/整房间二维码打印）: GET /inspection-scan/qr-batch?room_id=3&type=cabinet&days=365
func (h *Handler) BatchScanQR(c *gin.Context) {
	ttype := c.DefaultQuery("type", "cabinet")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "365"))
	if days <= 0 || days > 3650 {
		days = 365
	}
	exp := time.Now().AddDate(0, 0, days).Unix()
	type qrItem struct {
		Type string `json:"type"`
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Token string `json:"token"`
	}
	out := make([]qrItem, 0)
	if ttype == "cabinet" {
		q := h.DB.Model(&model.Cabinet{})
		if v := c.Query("room_id"); v != "" && v != "0" {
			q = q.Where("room_id = ?", v)
		}
		if v := c.Query("site_id"); v != "" && v != "0" {
			q = q.Where("site_id = ?", v)
		}
		if scope := middleware.GetSiteScope(c); scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		var cabs []model.Cabinet
		q.Order("code").Limit(500).Find(&cabs)
		for _, cab := range cabs {
			out = append(out, qrItem{Type: "cabinet", ID: cab.ID, Name: cab.Code, Token: h.signScanToken("cabinet", cab.ID, exp)})
		}
	} else {
		q := h.DB.Model(&model.Room{})
		if v := c.Query("site_id"); v != "" && v != "0" {
			q = q.Where("site_id = ?", v)
		}
		if scope := middleware.GetSiteScope(c); scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		var rooms []model.Room
		q.Order("id").Limit(200).Find(&rooms)
		for _, room := range rooms {
			out = append(out, qrItem{Type: "room", ID: room.ID, Name: room.Name, Token: h.signScanToken("room", room.ID, exp)})
		}
	}
	response.Success(c, gin.H{"items": out, "total": len(out), "expires_at": time.Unix(exp, 0).Format("2006-01-02")})
}

// ScanCheckin 扫码签到: POST /inspection-scan/checkin {token}
// 验签→落签到记录（服务端时间）→返回目标+适用计划+检查项实时值
func (h *Handler) ScanCheckin(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		response.BadRequest(c, "缺少扫码令牌")
		return
	}
	ttype, tid, _, err := h.verifyScanToken(strings.TrimSpace(req.Token))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var siteID uint
	var name string
	var roomID uint
	if ttype == "cabinet" {
		var cab model.Cabinet
		if h.DB.First(&cab, tid).Error != nil {
			response.NotFound(c, "机柜不存在")
			return
		}
		siteID, name, roomID = cab.SiteID, cab.Code, cab.RoomID
	} else {
		var room model.Room
		if h.DB.First(&room, tid).Error != nil {
			response.NotFound(c, "房间不存在")
			return
		}
		siteID, name = room.SiteID, room.Name
	}
	if !middleware.InSiteScope(c, siteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	uidN := uint64(middleware.GetUserID(c))
	now := time.Now()
	ck := model.InspectionCheckin{
		RefHash:     scanRefHash(ttype, tid),
		TargetType:  ttype, TargetID: tid, SiteID: siteID,
		UserID:      uint(uidN), CheckedInAt: now,
	}
	if err := h.DB.Create(&ck).Error; err != nil {
		response.ServerError(c, "签到失败")
		return
	}
	// 匹配适用巡检计划：房间精确匹配 > 全站计划（room_id=0）
	var plans []model.InspectionPlan
	pq := h.DB.Where("status = 1 AND site_id = ?", siteID)
	if ttype == "cabinet" || ttype == "room" {
		if roomID == 0 && ttype == "room" {
			roomID = tid
		}
		pq = pq.Where("room_id = ? OR room_id = 0 OR room_id IS NULL", roomID)
	}
	pq.Order("room_id DESC, id").Find(&plans)
	// 该目标今天的签到/巡检历史
	var todayCount int64
	h.DB.Model(&model.InspectionRecord{}).
		Where("target_type = ? AND target_id = ? AND inspected_at >= ?", ttype, tid, time.Now().Truncate(24*time.Hour)).
		Count(&todayCount)
	response.Success(c, gin.H{
		"checkin":       ck,
		"target":        gin.H{"type": ttype, "id": tid, "name": name, "site_id": siteID, "room_id": roomID},
		"plans":         plans,
		"today_records": todayCount,
		"server_time":   now.Format("2006-01-02 15:04:05"),
	})
}

func scanRefHash(ttype string, tid uint) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", ttype, tid)))
	return hex.EncodeToString(sum[:16])
}

// checkin 新鲜度窗口（分钟）：超过视为过期签到，须重新扫码
func (h *Handler) checkinFreshMinutes() int {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", "scan_checkin_fresh_min").Error; err == nil {
		if v, err := strconv.Atoi(s.Value); err == nil && v > 0 {
			return v
		}
	}
	return 120
}

// ScanSubmit 扫码巡检提交: POST /inspection-scan/submit
// body: {checkin_id, plan_id, items_results:[{item,result,note}], status, abnormal, point_snap}
// 防补录: inspected_at 取签到时间；签到必须未使用且在新鲜窗口内
func (h *Handler) ScanSubmit(c *gin.Context) {
	var req struct {
		CheckinID uint   `json:"checkin_id"`
		PlanID    uint   `json:"plan_id"`
		Status    int    `json:"status"` // 1 正常 2 异常
		Abnormal  string `json:"abnormal"`
		Result    string `json:"result"`
		PointSnap string `json:"point_snap"` // JSON 串（检查项实时值快照）
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CheckinID == 0 {
		response.BadRequest(c, "缺少签到记录")
		return
	}
	var ck model.InspectionCheckin
	if err := h.DB.First(&ck, req.CheckinID).Error; err != nil {
		response.NotFound(c, "签到记录不存在")
		return
	}
	if ck.Used {
		response.BadRequest(c, "该签到已使用，请重新扫码")
		return
	}
	fresh := time.Duration(h.checkinFreshMinutes()) * time.Minute
	if time.Since(ck.CheckedInAt) > fresh {
		response.BadRequest(c, fmt.Sprintf("签到已超过 %d 分钟有效窗口，请重新扫码", h.checkinFreshMinutes()))
		return
	}
	var plan model.InspectionPlan
	if req.PlanID != 0 {
		if err := h.DB.First(&plan, req.PlanID).Error; err != nil {
			response.NotFound(c, "巡检计划不存在")
			return
		}
	} else {
		// 未指定计划 → 取该目标最匹配的一个启用计划；没有则允许「临时巡检」(plan_id=0)
		var ps []model.InspectionPlan
		h.DB.Where("status = 1 AND site_id = ? AND (room_id = 0 OR room_id IS NULL)", ck.SiteID).Order("id").Limit(1).Find(&ps)
		if len(ps) > 0 {
			plan = ps[0]
		}
	}
	if plan.ID != 0 && !middleware.InSiteScope(c, plan.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	uidN := uint64(middleware.GetUserID(c))
	// 目标名称快照
	targetName := ""
	if ck.TargetType == "cabinet" {
		var cab model.Cabinet
		if h.DB.First(&cab, ck.TargetID).Error == nil {
			targetName = cab.Code
		}
	} else {
		var room model.Room
		if h.DB.First(&room, ck.TargetID).Error == nil {
			targetName = room.Name
		}
	}
	rec := model.InspectionRecord{
		PlanID: plan.ID, SiteID: ck.SiteID, InspectorID: uint(uidN),
		Status: req.Status, Result: req.Result, Abnormal: req.Abnormal,
		Source: "scan", CheckinID: ck.ID,
		TargetType: ck.TargetType, TargetID: ck.TargetID, TargetName: targetName,
		PointSnap:  req.PointSnap,
		ReportCode: h.genCode("IR"),
		InspectedAt: ck.CheckedInAt, // 防补录：以扫码签到时间为准
	}
	// 异常自动转工单（与桌面录入一致）
	if req.Status == 2 && req.Abnormal != "" {
		wo := model.WorkOrder{
			SiteID: ck.SiteID, Code: h.genCode("WO"),
			Title:       "扫码巡检异常：" + targetName,
			Type:        "fault", Priority: 3, Status: 1,
			Description: req.Abnormal, RequesterID: uint(uidN), CreatedBy: uint(uidN),
		}
		wo.CreatedAt = time.Now()
		h.applySLA(&wo) // V18 SLA 时限
		if h.DB.Create(&wo).Error == nil {
			rec.WorkOrderID = wo.ID
			h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: uint(uidN), Action: "create", Content: "扫码巡检异常自动创建"})
			h.DispatchWebhook(model.EvWorkOrder, "巡检异常转工单 "+wo.Code, fmt.Sprintf("%s 扫码巡检发现异常：%s", targetName, req.Abnormal), map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code})
		}
	}
	if err := h.DB.Create(&rec).Error; err != nil {
		response.ServerError(c, "提交失败")
		return
	}
	// 签到标记已用
	usedAt := time.Now()
	h.DB.Model(&ck).Updates(map[string]interface{}{"used": true, "used_at": usedAt, "record_id": rec.ID})
	h.LinkInspectionTask(&rec) // V62 核销当期巡检任务
	if plan.ID != 0 {
		h.DB.Model(&model.InspectionPlan{}).Where("id = ?", plan.ID).Update("last_run_at", usedAt)
	}
	middleware.WriteAudit(h.DB, c, "scan_inspection", targetName, fmt.Sprintf("checkin=%d record=%d", ck.ID, rec.ID))
	response.Created(c, rec)
}

// InspectionReport 巡检报告: GET /inspection-reports/:id
// 聚合：计划信息+逐项结果+测点快照+异常+工单链接+审核签字状态
func (h *Handler) InspectionReport(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rec model.InspectionRecord
	if err := h.DB.First(&rec, id).Error; err != nil {
		response.NotFound(c, "巡检记录不存在")
		return
	}
	if !middleware.InSiteScope(c, rec.SiteID) {
		response.Forbidden(c, "无权查看该站点")
		return
	}
	out := gin.H{
		"record": rec,
	}
	// 计划
	var plan model.InspectionPlan
	if rec.PlanID != 0 && h.DB.First(&plan, rec.PlanID).Error == nil {
		out["plan"] = gin.H{"id": plan.ID, "name": plan.Name, "freq": plan.Freq, "items": plan.Items}
	}
	// 站点
	var site model.Site
	if h.DB.First(&site, rec.SiteID).Error == nil {
		out["site"] = gin.H{"id": site.ID, "name": site.Name, "address": site.Address}
	}
	// 巡检人/审核人
	nameOf := func(uid uint) string {
		if uid == 0 {
			return ""
		}
		var u model.User
		if h.DB.Select("display_name", "username").First(&u, uid).Error == nil {
			if u.DisplayName != "" {
				return u.DisplayName
			}
			return u.Username
		}
		return fmt.Sprintf("用户#%d", uid)
	}
	out["inspector_name"] = nameOf(rec.InspectorID)
	out["reviewer_name"] = nameOf(rec.ReviewerID)
	// 关联工单
	if rec.WorkOrderID != 0 {
		var wo model.WorkOrder
		if h.DB.First(&wo, rec.WorkOrderID).Error == nil {
			out["work_order"] = gin.H{"id": wo.ID, "code": wo.Code, "title": wo.Title, "status": wo.Status}
		}
	}
	// 测点快照（存的是 JSON 串，原样回传给前端解析）
	if rec.PointSnap != "" {
		var snap interface{}
		if err := json.Unmarshal([]byte(rec.PointSnap), &snap); err == nil {
			out["point_snapshot"] = snap
		}
	}
	// 签到信息（扫码巡检才有）
	if rec.CheckinID != 0 {
		var ck model.InspectionCheckin
		if h.DB.First(&ck, rec.CheckinID).Error == nil {
			out["checkin"] = gin.H{"checked_in_at": ck.CheckedInAt, "used_at": ck.UsedAt}
		}
	}
	// 逐项结果解析（result 文本按行）
	lines := []gin.H{}
	for _, ln := range strings.Split(rec.Result, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		abnormal := strings.Contains(ln, "异常")
		lines = append(lines, gin.H{"text": ln, "abnormal": abnormal})
	}
	out["result_lines"] = lines
	response.Success(c, out)
}

// SignInspectionReport 报告签发: POST /inspection-reports/:id/sign {signature}
// 站点管理员以上审核签字；签发后不可再改
func (h *Handler) SignInspectionReport(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var rec model.InspectionRecord
	if err := h.DB.First(&rec, id).Error; err != nil {
		response.NotFound(c, "巡检记录不存在")
		return
	}
	if !middleware.InSiteScope(c, rec.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if rec.ReviewedAt != nil {
		response.BadRequest(c, "该报告已签发，不可重复签字")
		return
	}
	var req struct {
		Signature string `json:"signature"`
	}
	c.ShouldBindJSON(&req)
	_, uname := middleware.GetCaller(c)
	uidN := uint64(middleware.GetUserID(c))
	sig := strings.TrimSpace(req.Signature)
	if sig == "" {
		sig = uname // 默认用登录名
	}
	now := time.Now()
	h.DB.Model(&rec).Updates(map[string]interface{}{
		"reviewer_id": uint(uidN), "reviewed_at": now, "signature": sig,
	})
	if rec.ReportCode == "" {
		h.DB.Model(&rec).Update("report_code", h.genCode("IR"))
	}
	middleware.WriteAudit(h.DB, c, "sign_inspection_report", rec.ReportCode, sig)
	h.DB.First(&rec, id)
	response.Success(c, rec)
}

// ListInspectionReports 报告列表: GET /inspection-reports?site_id=&days=&status=&source=&keyword=
func (h *Handler) ListInspectionReports(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.InspectionRecord{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("source"); v != "" {
		q = q.Where("source = ?", v)
	}
	if v := c.Query("signed"); v == "1" {
		q = q.Where("reviewed_at IS NOT NULL")
	} else if v == "0" {
		q = q.Where("reviewed_at IS NULL")
	}
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			q = q.Where("inspected_at >= ?", time.Now().AddDate(0, 0, -d))
		}
	}
	if v := c.Query("keyword"); v != "" {
		kw := "%" + v + "%"
		q = q.Where("target_name LIKE ? OR report_code LIKE ? OR abnormal LIKE ?", kw, kw, kw)
	}
	var total int64
	q.Count(&total)
	var items []model.InspectionRecord
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.InspectionRecord{}
	}
	// 富化：计划名+巡检人名
	planNames := map[uint]string{}
	var plans []model.InspectionPlan
	h.DB.Select("id", "name").Find(&plans)
	for _, p := range plans {
		planNames[p.ID] = p.Name
	}
	userNames := map[uint]string{}
	var users []model.User
	h.DB.Select("id", "display_name", "username").Find(&users)
	for _, u := range users {
		if u.DisplayName != "" {
			userNames[u.ID] = u.DisplayName
		} else {
			userNames[u.ID] = u.Username
		}
	}
	type row struct {
		model.InspectionRecord
		PlanName     string `json:"plan_name"`
		InspectorName string `json:"inspector_name"`
		ReviewerName string `json:"reviewer_name"`
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		rows = append(rows, row{
			InspectionRecord: it,
			PlanName:         planNames[it.PlanID],
			InspectorName:    userNames[it.InspectorID],
			ReviewerName:     userNames[it.ReviewerID],
		})
	}
	response.Paginated(c, rows, total, page, size)
}
