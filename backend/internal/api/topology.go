package api

import (
	"fmt"
	"strconv"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) paging(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 500 {
		size = 20
	}
	return page, size
}

func parseIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "ID 无效")
		return 0, false
	}
	return uint(id), true
}

// ==================== 站点 Site ====================

type siteReq struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Level    int     `json:"level"`
	Address  string  `json:"address"`
	Province string  `json:"province"`
	Lng     float64 `json:"lng"`
	Lat     float64 `json:"lat"`
	Manager string  `json:"manager"`
	Phone   string  `json:"phone"`
	Status  int     `json:"status"`
	Remark  string  `json:"remark"`
}

// ListSites 站点列表（站点管理员只见授权站点）
func (h *Handler) ListSites(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Site{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("id IN ?", scope)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR code LIKE ? OR address LIKE ?", like, like, like)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	var total int64
	q.Count(&total)
	var items []model.Site
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

// CreateSite 新增站点
func (h *Handler) CreateSite(c *gin.Context) {
	var req siteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Code == "" || req.Name == "" {
		response.BadRequest(c, "站点编码和名称不能为空")
		return
	}
	var cnt int64
	h.DB.Model(&model.Site{}).Where("code = ?", req.Code).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "站点编码已存在")
		return
	}
	s := model.Site{
		Code: req.Code, Name: req.Name, Level: req.Level, Address: req.Address,
		Province: req.Province,
		Lng: req.Lng, Lat: req.Lat, Manager: req.Manager, Phone: req.Phone,
		Status: 1, Remark: req.Remark,
	}
	if req.Status != 0 {
		s.Status = req.Status
	}
	if err := h.DB.Create(&s).Error; err != nil {
		response.ServerError(c, "创建失败: "+err.Error())
		return
	}
	middleware.WriteAudit(h.DB, c, "create_site", s.Code, s.Name)
	response.Created(c, s)
}

// UpdateSite 更新站点
func (h *Handler) UpdateSite(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if scope := middleware.GetSiteScope(c); scope != nil {
		allowed := false
		for _, sid := range scope {
			if sid == id {
				allowed = true
				break
			}
		}
		if !allowed {
			response.Forbidden(c, "无权访问该站点")
			return
		}
	}
	var s model.Site
	if err := h.DB.First(&s, id).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var req siteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Name != "" {
		s.Name = req.Name
	}
	s.Level = req.Level
	s.Address = req.Address
	s.Province = req.Province
	s.Lng = req.Lng
	s.Lat = req.Lat
	s.Manager = req.Manager
	s.Phone = req.Phone
	s.Remark = req.Remark
	if req.Status != 0 || c.Query("disable") == "1" {
		s.Status = req.Status
	}
	h.DB.Save(&s)
	middleware.WriteAudit(h.DB, c, "update_site", s.Code, s.Name)
	response.Success(c, s)
}

// DeleteSite 删除站点（需无下级数据）
func (h *Handler) DeleteSite(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var s model.Site
	if err := h.DB.First(&s, id).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var rooms, cabinets, gws int64
	h.DB.Model(&model.Room{}).Where("site_id = ?", id).Count(&rooms)
	h.DB.Model(&model.Cabinet{}).Where("site_id = ?", id).Count(&cabinets)
	h.DB.Model(&model.Gateway{}).Where("site_id = ?", id).Count(&gws)
	if rooms > 0 || cabinets > 0 || gws > 0 {
		response.BadRequest(c, "站点下仍有房间/机柜/网关数据，无法删除")
		return
	}
	// V16: 资产/测点/网络设备/供电链路残留校验
	var sAssets, sPoints, sDevs, sPCs int64
	h.DB.Model(&model.Asset{}).Where("site_id = ?", id).Count(&sAssets)
	h.DB.Model(&model.MonitoringPoint{}).Where("site_id = ?", id).Count(&sPoints)
	h.DB.Model(&model.NetworkDevice{}).Where("site_id = ?", id).Count(&sDevs)
	h.DB.Model(&model.PowerChain{}).Where("site_id = ?", id).Count(&sPCs)
	if sAssets+sPoints+sDevs+sPCs > 0 {
		response.BadRequest(c, fmt.Sprintf("站点下仍有业务数据(资产%d/测点%d/网络设备%d/供电节点%d)，请先清理", sAssets, sPoints, sDevs, sPCs))
		return
	}
	h.DB.Delete(&s)
	middleware.WriteAudit(h.DB, c, "delete_site", s.Code, s.Name)
	response.Success(c, gin.H{"ok": true})
}

// ==================== 房间 Room ====================

type roomReq struct {
	SiteID uint    `json:"site_id"`
	Name   string  `json:"name"`
	Floor  string  `json:"floor"`
	Area   float64 `json:"area"`
	Grade  string  `json:"grade"`
	Remark string  `json:"remark"`
}

func (h *Handler) ListRooms(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Room{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if sid := c.Query("site_id"); sid != "" {
		q = q.Where("site_id = ?", sid)
	}
	var total int64
	q.Count(&total)
	var items []model.Room
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var req roomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.SiteID == 0 || req.Name == "" {
		response.BadRequest(c, "站点和房间名称不能为空")
		return
	}
	var site model.Site
	if err := h.DB.First(&site, req.SiteID).Error; err != nil {
		response.BadRequest(c, "所属站点不存在")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权在该站点创建房间")
		return
	}
	r := model.Room{SiteID: req.SiteID, Name: req.Name, Floor: req.Floor, Area: req.Area, Grade: req.Grade, Remark: req.Remark}
	if err := h.DB.Create(&r).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_room", site.Code+"/"+req.Name, "")
	response.Created(c, r)
}

func (h *Handler) UpdateRoom(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var r model.Room
	if err := h.DB.First(&r, id).Error; err != nil {
		response.NotFound(c, "房间不存在")
		return
	}
	if !middleware.InSiteScope(c, r.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var req roomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Name != "" {
		r.Name = req.Name
	}
	r.Floor = req.Floor
	r.Area = req.Area
	r.Grade = req.Grade
	r.Remark = req.Remark
	h.DB.Save(&r)
	middleware.WriteAudit(h.DB, c, "update_room", r.Name, "")
	response.Success(c, r)
}

func (h *Handler) DeleteRoom(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var r model.Room
	if err := h.DB.First(&r, id).Error; err != nil {
		response.NotFound(c, "房间不存在")
		return
	}
	if !middleware.InSiteScope(c, r.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var cabs int64
	h.DB.Model(&model.Cabinet{}).Where("room_id = ?", id).Count(&cabs)
	if cabs > 0 {
		response.BadRequest(c, "房间下仍有机柜，无法删除")
		return
	}
	// V16: 房间级测点也须先处理
	var rPts int64
	h.DB.Model(&model.MonitoringPoint{}).Where("room_id = ?", id).Count(&rPts)
	if rPts > 0 {
		response.BadRequest(c, fmt.Sprintf("房间仍关联 %d 个测点，请先迁移或删除", rPts))
		return
	}
	var rRows int64
	h.DB.Model(&model.Row{}).Where("room_id = ?", id).Count(&rRows)
	if rRows > 0 {
		response.BadRequest(c, fmt.Sprintf("房间下仍有 %d 个机柜列，请先删除", rRows))
		return
	}
	h.DB.Delete(&r)
	middleware.WriteAudit(h.DB, c, "delete_room", r.Name, "")
	response.Success(c, gin.H{"ok": true})
}

// ==================== 机柜列 Row ====================

type rowReq struct {
	SiteID uint   `json:"site_id"`
	RoomID uint   `json:"room_id"`
	Name   string `json:"name"`
	Channel string `json:"channel"`
	Remark string `json:"remark"`
}

func (h *Handler) ListRows(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Row{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if rid := c.Query("room_id"); rid != "" {
		q = q.Where("room_id = ?", rid)
	}
	var total int64
	q.Count(&total)
	var items []model.Row
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateRow(c *gin.Context) {
	var req rowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.RoomID == 0 || req.Name == "" {
		response.BadRequest(c, "房间和列名称不能为空")
		return
	}
	var room model.Room
	if err := h.DB.First(&room, req.RoomID).Error; err != nil {
		response.BadRequest(c, "所属房间不存在")
		return
	}
	r := model.Row{SiteID: room.SiteID, RoomID: req.RoomID, Name: req.Name, Channel: req.Channel, Remark: req.Remark}
	if err := h.DB.Create(&r).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_row", room.Name+"/"+req.Name, "")
	response.Created(c, r)
}

func (h *Handler) UpdateRow(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var r model.Row
	if err := h.DB.First(&r, id).Error; err != nil {
		response.NotFound(c, "机柜列不存在")
		return
	}
	if !middleware.InSiteScope(c, r.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var req rowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Name != "" {
		r.Name = req.Name
	}
	r.Channel = req.Channel
	r.Remark = req.Remark
	h.DB.Save(&r)
	middleware.WriteAudit(h.DB, c, "update_row", r.Name, "")
	response.Success(c, r)
}

func (h *Handler) DeleteRow(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var r model.Row
	if err := h.DB.First(&r, id).Error; err != nil {
		response.NotFound(c, "机柜列不存在")
		return
	}
	if !middleware.InSiteScope(c, r.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var cabs int64
	h.DB.Model(&model.Cabinet{}).Where("row_id = ?", id).Count(&cabs)
	if cabs > 0 {
		response.BadRequest(c, "列下仍有机柜，无法删除")
		return
	}
	h.DB.Delete(&r)
	middleware.WriteAudit(h.DB, c, "delete_row", r.Name, "")
	response.Success(c, gin.H{"ok": true})
}

// ==================== 机柜 Cabinet ====================

type cabinetReq struct {
	SiteID       uint    `json:"site_id"`
	RoomID       uint    `json:"room_id"`
	RowID        uint    `json:"row_id"`
	Code         string  `json:"code"`
	Model        string  `json:"model"`
	TotalU       int     `json:"total_u"`
	RatedPowerKW float64 `json:"rated_power_kw"`
	MaxWeightKG  int     `json:"max_weight_kg"`
	Status       int     `json:"status"`
	Remark       string  `json:"remark"`
}

func (h *Handler) ListCabinets(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Cabinet{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	for _, f := range []string{"site_id", "room_id", "row_id"} {
		if v := c.Query(f); v != "" {
			q = q.Where(f+" = ?", v)
		}
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("code LIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	var items []model.Cabinet
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

func (h *Handler) CreateCabinet(c *gin.Context) {
	var req cabinetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.RoomID == 0 || req.Code == "" {
		response.BadRequest(c, "房间和机柜编号不能为空")
		return
	}
	var room model.Room
	if err := h.DB.First(&room, req.RoomID).Error; err != nil {
		response.BadRequest(c, "所属房间不存在")
		return
	}
	if !middleware.InSiteScope(c, room.SiteID) {
		response.Forbidden(c, "无权在该站点创建机柜")
		return
	}
	var cnt int64
	h.DB.Model(&model.Cabinet{}).Where("room_id = ? AND code = ?", req.RoomID, req.Code).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "该房间下机柜编号已存在")
		return
	}
	cab := model.Cabinet{
		SiteID: room.SiteID, RoomID: req.RoomID, RowID: req.RowID, Code: req.Code,
		Model: req.Model, TotalU: req.TotalU, RatedPowerKW: req.RatedPowerKW,
		MaxWeightKG: req.MaxWeightKG, Status: 1, Remark: req.Remark,
	}
	if cab.TotalU <= 0 {
		cab.TotalU = 42
	}
	if req.Status != 0 {
		cab.Status = req.Status
	}
	if err := h.DB.Create(&cab).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_cabinet", room.Name+"/"+req.Code, "")
	response.Created(c, cab)
}

func (h *Handler) UpdateCabinet(c *gin.Context) {
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
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var req cabinetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Code != "" {
		cab.Code = req.Code
	}
	cab.Model = req.Model
	if req.TotalU > 0 {
		// V16: total_u 不能小于已用U（防止在架资产U位越界）
		if req.TotalU < cab.TotalU {
			var usedU int64
			h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", cab.ID).
				Select("COALESCE(MAX(start_u + height_u - 1),0)").Scan(&usedU)
			if int(req.TotalU) < int(usedU) {
				response.BadRequest(c, fmt.Sprintf("U位数不能小于已用U位（当前最高占用 %dU）", usedU))
				return
			}
		}
		cab.TotalU = req.TotalU
	}
	cab.RatedPowerKW = req.RatedPowerKW
	cab.MaxWeightKG = req.MaxWeightKG
	cab.Remark = req.Remark
	if req.RowID != 0 {
		cab.RowID = req.RowID
	}
	if req.Status != 0 || c.Query("disable") == "1" {
		cab.Status = req.Status
	}
	h.DB.Save(&cab)
	middleware.WriteAudit(h.DB, c, "update_cabinet", cab.Code, "")
	response.Success(c, cab)
}

func (h *Handler) DeleteCabinet(c *gin.Context) {
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
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	// V16: 在架资产/关联测点/供电绑定校验，防止孤儿数据
	var assetCnt int64
	h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", id).Count(&assetCnt)
	if assetCnt > 0 {
		response.BadRequest(c, fmt.Sprintf("机柜内仍有 %d 台在架资产，请先下架后再删除", assetCnt))
		return
	}
	var ptCnt int64
	h.DB.Model(&model.MonitoringPoint{}).Where("cabinet_id = ?", id).Count(&ptCnt)
	if ptCnt > 0 {
		response.BadRequest(c, fmt.Sprintf("机柜仍关联 %d 个测点，请先迁移或删除测点", ptCnt))
		return
	}
	var pcCnt int64
	h.DB.Model(&model.PowerChain{}).Where("cabinet_id = ?", id).Count(&pcCnt)
	if pcCnt > 0 {
		response.BadRequest(c, fmt.Sprintf("机柜仍被 %d 个供电链路节点引用，请先解除绑定", pcCnt))
		return
	}
	var devCnt int64
	h.DB.Model(&model.NetworkDevice{}).Where("cabinet_id = ?", id).Count(&devCnt)
	if devCnt > 0 {
		response.BadRequest(c, fmt.Sprintf("机柜内仍有 %d 台网络设备，请先迁移", devCnt))
		return
	}
	h.DB.Delete(&cab)
	middleware.WriteAudit(h.DB, c, "delete_cabinet", cab.Code, "")
	response.Success(c, gin.H{"ok": true})
}
