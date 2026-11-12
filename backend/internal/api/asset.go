package api

import (
	"fmt"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 资产 Asset（P1） ====================

var validCategories = map[string]bool{
	"server": true, "network": true, "storage": true,
	"power": true, "cooling": true, "security": true, "other": true,
}

type assetReq struct {
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	Brand        string    `json:"brand"`
	Model        string    `json:"model"`
	SN           string    `json:"sn"`
	SiteID       uint      `json:"site_id"`
	CabinetID    uint      `json:"cabinet_id"`
	RoomID       uint      `json:"room_id"` // V34: 非机架设备所在房间
	StartU       int       `json:"start_u"`
	HeightU      int       `json:"height_u"`
	PowerW       int       `json:"power_w"`
	WeightKG     float64   `json:"weight_kg"`
	IP           string    `json:"ip"`
	AssetNo      string    `json:"asset_no"`
	FixedAssetNo string    `json:"fixed_asset_no"`
	Vendor       string    `json:"vendor"`
	ContractNo   string    `json:"contract_no"`
	Department   string    `json:"department"`
	Dimensions   string    `json:"dimensions"`
	PurchaseAt   *flexDate `json:"purchase_at"` // V16: 兼容 date 输入框短日期串(旧 *time.Time 会导致 400)
	Owner        string    `json:"owner"`
	Status       int       `json:"status"`
	Remark       string    `json:"remark"`
}

// ListAssets 资产台账列表（支持站点/分类/状态/机柜/关键词筛选）
func (h *Handler) ListAssets(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Asset{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if sid := c.Query("site_id"); sid != "" {
		q = q.Where("site_id = ?", sid)
	}
	if v := c.Query("id"); v != "" && v != "0" { // M2 移动端资产速查精确查询
		q = q.Where("id = ?", v)
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if cid := c.Query("cabinet_id"); cid != "" {
		q = q.Where("cabinet_id = ?", cid)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR code LIKE ? OR sn LIKE ? OR brand LIKE ? OR model LIKE ?",
			like, like, like, like, like)
	}
	if hg := c.Query("health_grade"); hg != "" { // V50 健康等级过滤（Dashboard 穿透深链）
		q = q.Joins("LEFT JOIN t_asset_health ah ON ah.asset_id = t_asset.id").Where("ah.grade = ?", hg)
	}
	var total int64
	q.Count(&total)
	var items []model.Asset
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if c.Query("enrich") == "1" && len(items) > 0 {
		// M6: 附加可读位置（站点/房间/机柜），移动端资产列表用
		siteIDs, roomIDs, cabIDs := []uint{}, []uint{}, []uint{}
		for _, a := range items {
			if a.SiteID > 0 {
				siteIDs = append(siteIDs, a.SiteID)
			}
			if a.RoomID > 0 {
				roomIDs = append(roomIDs, a.RoomID)
			}
			if a.CabinetID > 0 {
				cabIDs = append(cabIDs, a.CabinetID)
			}
		}
		siteNames := map[uint]string{}
		roomNames := map[uint]string{}
		cabCodes := map[uint]string{}
		if len(siteIDs) > 0 {
			var ss []model.Site
			h.DB.Select("id, name").Where("id IN ?", siteIDs).Find(&ss)
			for _, s := range ss {
				siteNames[s.ID] = s.Name
			}
		}
		if len(roomIDs) > 0 {
			var rs []model.Room
			h.DB.Select("id, name").Where("id IN ?", roomIDs).Find(&rs)
			for _, r := range rs {
				roomNames[r.ID] = r.Name
			}
		}
		if len(cabIDs) > 0 {
			var cs []model.Cabinet
			h.DB.Select("id, code").Where("id IN ?", cabIDs).Find(&cs)
			for _, cb := range cs {
				cabCodes[cb.ID] = cb.Code
			}
		}
		rows := make([]gin.H, 0, len(items))
		for _, a := range items {
			rows = append(rows, gin.H{
				"asset": a, "site_name": siteNames[a.SiteID],
				"room_name": roomNames[a.RoomID], "cabinet_code": cabCodes[a.CabinetID],
			})
		}
		response.Paginated(c, rows, total, page, size)
		return
	}
	// V50: 附加健康分/等级（PC 资产列表健康列）
	hmap := h.assetHealthMap(items)
	type assetPlus struct {
		model.Asset
		HealthScore *int   `json:"health_score"`
		HealthGrade string `json:"health_grade"`
	}
	plus := make([]assetPlus, 0, len(items))
	for _, a := range items {
		ap := assetPlus{Asset: a}
		if hv, ok := hmap[a.ID]; ok {
			sc := hv.Score
			ap.HealthScore = &sc
			ap.HealthGrade = hv.Grade
		}
		plus = append(plus, ap)
	}
	response.Paginated(c, plus, total, page, size)
}

// assetHealthMap V50: 批量取资产健康记录（asset_id -> {score,grade}）
func (h *Handler) assetHealthMap(items []model.Asset) map[uint]model.AssetHealth {
	out := map[uint]model.AssetHealth{}
	if len(items) == 0 {
		return out
	}
	ids := make([]uint, 0, len(items))
	for _, a := range items {
		ids = append(ids, a.ID)
	}
	var rows []model.AssetHealth
	h.DB.Where("asset_id IN ?", ids).Find(&rows)
	for _, r := range rows {
		out[r.AssetID] = r
	}
	return out
}

// CreateAsset 新增资产（默认在库；如需直接上架请随后调用 mount）
func (h *Handler) CreateAsset(c *gin.Context) {
	var req assetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Code == "" || req.Name == "" {
		response.BadRequest(c, "资产编码和名称必填")
		return
	}
	if req.Category == "" {
		req.Category = "other"
	}
	if !validCategories[req.Category] {
		response.BadRequest(c, "分类无效")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权在该站点创建资产")
		return
	}
	var dup int64
	h.DB.Model(&model.Asset{}).Where("code = ?", req.Code).Count(&dup)
	if dup > 0 {
		response.BadRequest(c, "资产编码已存在")
		return
	}
	if req.HeightU < 1 {
		req.HeightU = 1
	}
	a := model.Asset{
		Code: req.Code, Name: req.Name, Category: req.Category,
		Brand: req.Brand, Model: req.Model, SN: req.SN, SiteID: req.SiteID,
		RoomID: req.RoomID,
		HeightU: req.HeightU, PowerW: req.PowerW, WeightKG: req.WeightKG,
		IP: req.IP, AssetNo: req.AssetNo, FixedAssetNo: req.FixedAssetNo,
		Vendor: req.Vendor, ContractNo: req.ContractNo,
		Department: req.Department, Dimensions: req.Dimensions,
		PurchaseAt: func() *time.Time {
			if req.PurchaseAt != nil {
				return req.PurchaseAt.toTimePtr()
			}
			return nil
		}(), Owner: req.Owner,
		Status: 2, Remark: req.Remark,
	}
	if err := h.DB.Create(&a).Error; err != nil {
		response.BadRequest(c, "创建失败："+err.Error())
		return
	}
	middleware.WriteAudit(h.DB, c, "create_asset", a.Code, a.Name)
	response.Created(c, a)
}

// UpdateAsset 更新资产基础信息（不含上下架，上下架走专用接口）
func (h *Handler) UpdateAsset(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	var req assetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Name != "" {
		a.Name = req.Name
	}
	if req.Category != "" {
		if !validCategories[req.Category] {
			response.BadRequest(c, "分类无效")
			return
		}
		a.Category = req.Category
	}
	a.Brand = req.Brand
	a.Model = req.Model
	a.SN = req.SN
	a.AssetNo = req.AssetNo
	a.FixedAssetNo = req.FixedAssetNo
	a.Vendor = req.Vendor
	a.ContractNo = req.ContractNo
	a.Department = req.Department
	a.Dimensions = req.Dimensions
	if req.HeightU >= 1 {
		a.HeightU = req.HeightU
	}
	a.PowerW = req.PowerW
	a.WeightKG = req.WeightKG
	a.IP = req.IP
	// V16: flexDate 在 req 里是指针, 转换时先判空
	if req.PurchaseAt != nil {
		a.PurchaseAt = req.PurchaseAt.toTimePtr()
	}
	a.Owner = req.Owner
	a.Remark = req.Remark
	a.RoomID = req.RoomID // V34: 非机架设备所在房间(局部PUT会清0, 与其他字段行为一致, 前端表单总是携带)
	if req.Status == 3 || req.Status == 4 || (req.Status == 2 && a.Status != 1) {
		// 允许切换到维修中/报废/在库（在架资产不能直接改在库，须先下架）
		a.Status = req.Status
	}
	if err := h.DB.Save(&a).Error; err != nil {
		response.BadRequest(c, "更新失败："+err.Error())
		return
	}
	middleware.WriteAudit(h.DB, c, "update_asset", a.Code, a.Name)
	response.Success(c, a)
}

// DeleteAsset 删除资产（在架资产须先下架；级联删除维保记录）
func (h *Handler) DeleteAsset(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	if a.Status == 1 && a.CabinetID > 0 {
		response.BadRequest(c, "资产仍在架，请先下架再删除")
		return
	}
	h.DB.Where("asset_id = ?", a.ID).Delete(&model.Maintenance{})
	h.DB.Delete(&a)
	middleware.WriteAudit(h.DB, c, "delete_asset", a.Code, a.Name)
	response.Success(c, gin.H{"ok": true})
}

// ==================== 上下架（U位管理） ====================

type mountReq struct {
	CabinetID uint `json:"cabinet_id"`
	StartU    int  `json:"start_u"`
	Force     int  `json:"force"` // V51: 1=四维校验超限时强制上架（写审计）
}

// MountAsset 上架：校验机柜归属/状态、U位边界、U位冲突、功率与承重
func (h *Handler) MountAsset(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	if a.Status == 1 {
		response.BadRequest(c, "资产已在架，请先下架")
		return
	}
	if a.Status == 4 {
		response.BadRequest(c, "已报废资产不可上架")
		return
	}
	var req mountReq
	if err := c.ShouldBindJSON(&req); err != nil || req.CabinetID == 0 || req.StartU < 1 {
		response.BadRequest(c, "须指定机柜和起始U位（start_u≥1）")
		return
	}
	var cab model.Cabinet
	if err := h.DB.First(&cab, req.CabinetID).Error; err != nil {
		response.NotFound(c, "机柜不存在")
		return
	}
	if cab.SiteID != a.SiteID {
		response.BadRequest(c, "机柜与资产不在同一站点")
		return
	}
	if !middleware.InSiteScope(c, cab.SiteID) {
		response.Forbidden(c, "无权操作该机柜")
		return
	}
	if cab.Status == 0 {
		response.BadRequest(c, "机柜已停用，不可上架")
		return
	}
	endU := req.StartU + a.HeightU - 1
	if endU > cab.TotalU {
		response.BadRequest(c, fmt.Sprintf("U位越界：机柜共 %dU，需要 %d~%dU", cab.TotalU, req.StartU, endU))
		return
	}
	// U位冲突检测
	var occupants []model.Asset
	h.DB.Where("cabinet_id = ? AND status = 1", cab.ID).Find(&occupants)
	for _, o := range occupants {
		oEnd := o.StartU + o.HeightU - 1
		if req.StartU <= oEnd && o.StartU <= endU {
			response.BadRequest(c, fmt.Sprintf("U位冲突：与「%s」（%d~%dU）重叠", o.Name, o.StartU, oEnd))
			return
		}
	}
	// V51: 四维容量 What-if 校验（U位/功率/承重/制冷）
	wi := whatIfCheck(h.DB, cab, a.HeightU, a.PowerW, a.WeightKG)
	forced := false
	if wi.Verdict == "over" {
		if req.Force != 1 {
			response.BadRequest(c, wi.Conclusion+"（如确需上架可勾选强制上架）")
			return
		}
		forced = true
	}
	a.CabinetID = cab.ID
	a.StartU = req.StartU
	a.Status = 1
	h.DB.Save(&a)
	detail := fmt.Sprintf("机柜=%s U位=%d~%d", cab.Code, req.StartU, endU)
	if forced {
		detail += " [强制上架,超限维度:" + strings.Join(wi.Over, ",") + "] " + wi.Conclusion
		go middleware.WriteAudit(h.DB, c, "mount_asset_forced", a.Code, detail)
	} else {
		middleware.WriteAudit(h.DB, c, "mount_asset", a.Code, detail)
	}
	response.Success(c, gin.H{"asset": a, "what_if": wi, "forced": forced})
}

// UnmountAsset 下架：资产回库（在库状态）
func (h *Handler) UnmountAsset(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产")
		return
	}
	if a.CabinetID == 0 {
		response.BadRequest(c, "资产未在架")
		return
	}
	var cabCode string
	h.DB.Model(&model.Cabinet{}).Where("id = ?", a.CabinetID).Pluck("code", &cabCode)
	a.CabinetID = 0
	a.StartU = 0
	a.Status = 2
	h.DB.Save(&a)
	middleware.WriteAudit(h.DB, c, "unmount_asset", a.Code, "原机柜="+cabCode)
	response.Success(c, a)
}

// CabinetCapacity 机柜容量视图：U位占用图 + 功率/承重统计
func (h *Handler) CabinetCapacity(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var cab model.Cabinet
	if err := h.DB.First(&cab, id).Error; err != nil {
		response.NotFound(c, "机柜不存在")
		return
	}
	if !middleware.InSiteScope(c, cab.SiteID) {
		response.Forbidden(c, "无权查看该机柜")
		return
	}
	var assets []model.Asset
	h.DB.Where("cabinet_id = ? AND status = 1", cab.ID).Order("start_u").Find(&assets)

	// U位占用图：1~total_u，0=空闲，否则为资产ID
	uMap := make([]uint, cab.TotalU+1)
	var usedU int
	var usedW int
	var usedKG float64
	for _, a := range assets {
		for u := a.StartU; u < a.StartU+a.HeightU && u <= cab.TotalU; u++ {
			if uMap[u] == 0 {
				uMap[u] = a.ID
				usedU++
			}
		}
		usedW += a.PowerW
		usedKG += a.WeightKG
	}
	response.Success(c, gin.H{
		"cabinet":        cab,
		"assets":         assets,
		"u_map":          uMap[1:],
		"total_u":        cab.TotalU,
		"used_u":         usedU,
		"free_u":         cab.TotalU - usedU,
		"used_power_w":   usedW,
		"used_weight_kg": usedKG,
	})
}

// ==================== 维保 Maintenance ====================

// flexDate 兼容 "2006-01-02"（date输入框）与 RFC3339 完整时间串
type flexDate time.Time

func (fd *flexDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			*fd = flexDate(t)
			return nil
		}
	}
	return fmt.Errorf("无法解析日期: %s", s)
}

func (fd flexDate) toTimePtr() *time.Time {
	t := time.Time(fd)
	if t.IsZero() {
		return nil
	}
	return &t
}

type maintenanceReq struct {
	AssetID  uint      `json:"asset_id"`
	Kind     string    `json:"kind"`
	Vendor   string    `json:"vendor"`
	Contract string    `json:"contract"`
	StartAt  *flexDate `json:"start_at"`
	EndAt    *flexDate `json:"end_at"`
	Cost     float64   `json:"cost"`
	Detail   string    `json:"detail"`
	Status   int       `json:"status"`
}

func (req *maintenanceReq) startPtr() *time.Time {
	if req.StartAt == nil {
		return nil
	}
	return req.StartAt.toTimePtr()
}
func (req *maintenanceReq) endPtr() *time.Time {
	if req.EndAt == nil {
		return nil
	}
	return req.EndAt.toTimePtr()
}

// ListMaintenances 维保记录列表（可按资产过滤）
func (h *Handler) ListMaintenances(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Maintenance{})
	if aid := c.Query("asset_id"); aid != "" {
		q = q.Where("asset_id = ?", aid)
	}
	if kind := c.Query("kind"); kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	// 站点范围过滤：通过资产关联
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("asset_id IN (?)",
			h.DB.Model(&model.Asset{}).Select("id").Where("site_id IN ?", scope))
	}
	var total int64
	q.Count(&total)
	var items []model.Maintenance
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

// CreateMaintenance 新增维保记录
func (h *Handler) CreateMaintenance(c *gin.Context) {
	var req maintenanceReq
	if err := c.ShouldBindJSON(&req); err != nil || req.AssetID == 0 {
		response.BadRequest(c, "须指定资产")
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, req.AssetID).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该资产的维保")
		return
	}
	if req.Kind == "" {
		req.Kind = "contract"
	}
	if req.Kind != "contract" && req.Kind != "repair" {
		response.BadRequest(c, "类型无效（contract/repair）")
		return
	}
	m := model.Maintenance{
		AssetID: req.AssetID, Kind: req.Kind, Vendor: req.Vendor,
		Contract: req.Contract, StartAt: req.startPtr(), EndAt: req.endPtr(),
		Cost: req.Cost, Detail: req.Detail, Status: 1,
	}
	if req.Status != 0 {
		m.Status = req.Status
	}
	h.DB.Create(&m)
	middleware.WriteAudit(h.DB, c, "create_maintenance", a.Code,
		fmt.Sprintf("类型=%s 厂商=%s", m.Kind, m.Vendor))
	response.Created(c, m)
}

// UpdateMaintenance 更新维保记录
func (h *Handler) UpdateMaintenance(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var m model.Maintenance
	if err := h.DB.First(&m, id).Error; err != nil {
		response.NotFound(c, "维保记录不存在")
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, m.AssetID).Error; err == nil && !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该维保记录")
		return
	}
	var req maintenanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	m.Vendor = req.Vendor
	m.Contract = req.Contract
	m.StartAt = req.startPtr()
	m.EndAt = req.endPtr()
	m.Cost = req.Cost
	m.Detail = req.Detail
	if req.Status != 0 || c.Query("disable") == "1" {
		m.Status = req.Status
	}
	h.DB.Save(&m)
	middleware.WriteAudit(h.DB, c, "update_maintenance", a.Code, "类型="+m.Kind)
	response.Success(c, m)
}

// DeleteMaintenance 删除维保记录
func (h *Handler) DeleteMaintenance(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var m model.Maintenance
	if err := h.DB.First(&m, id).Error; err != nil {
		response.NotFound(c, "维保记录不存在")
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, m.AssetID).Error; err == nil && !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该维保记录")
		return
	}
	h.DB.Delete(&m)
	middleware.WriteAudit(h.DB, c, "delete_maintenance", a.Code, "类型="+m.Kind)
	response.Success(c, gin.H{"ok": true})
}

// ScanExpiringMaintenances 维保到期扫描：30天内到期的维保合同
func (h *Handler) ScanExpiringMaintenances(c *gin.Context) {
	now := time.Now()
	deadline := now.AddDate(0, 0, 30)
	var items []model.Maintenance
	h.DB.Where("kind = 'contract' AND status = 1 AND end_at IS NOT NULL AND end_at <= ?", deadline).
		Order("end_at").Find(&items)
	type expiringItem struct {
		Maintenance model.Maintenance `json:"maintenance"`
		AssetCode   string            `json:"asset_code"`
		AssetName   string            `json:"asset_name"`
		DaysLeft    int               `json:"days_left"`
	}
	var result []expiringItem
	for _, m := range items {
		var a model.Asset
		h.DB.First(&a, m.AssetID)
		daysLeft := int(m.EndAt.Sub(now).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
		result = append(result, expiringItem{m, a.Code, a.Name, daysLeft})
	}
	if result == nil {
		result = []expiringItem{}
	}
	response.Success(c, gin.H{"items": result, "total": len(result)})
}

// PushNotifications 推送通知（维保到期+告警汇总）
func (h *Handler) PushNotifications(c *gin.Context) {
	now := time.Now()
	deadline := now.AddDate(0, 0, 30)
	var maints []model.Maintenance
	h.DB.Where("kind = 'contract' AND status = 1 AND end_at IS NOT NULL AND end_at <= ?", deadline).
		Order("end_at").Find(&maints)
	pushed := 0
	for _, m := range maints {
		daysLeft := int(m.EndAt.Sub(now).Hours() / 24)
		var a model.Asset
		h.DB.First(&a, m.AssetID)
		msg := fmt.Sprintf("维保到期提醒：%s（%s）合同将于 %d 天后到期", a.Name, a.Code, daysLeft)
		var exists int64
		h.DB.Model(&model.Notification{}).
			Where("content = ? AND created_at > ?", msg, now.Add(-24*time.Hour)).Count(&exists)
		if exists == 0 {
			n := model.Notification{
				UserID: 0, Kind: "maintenance_due",
				Title: "维保到期提醒", Content: msg,
				Read: false,
				RefType: "asset", RefID: a.ID,
			}
			h.DB.Create(&n)
			h.DispatchWebhook(model.EvMaintenanceDue, "维保到期提醒", msg, map[string]interface{}{"asset_id": a.ID, "asset_code": a.Code, "days_left": daysLeft})
			pushed++
		}
	}
	response.Success(c, gin.H{"pushed": pushed, "scanned": len(maints)})
}
