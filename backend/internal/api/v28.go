package api

import (
	"encoding/json"
	"strconv"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V28 布局模板 ====================

// layoutTplItem 模板内单项(与前端 saveLayout items 对齐, code 用于机柜跨房间重映射)
type layoutTplItem struct {
	RefKind  string  `json:"ref_kind"`
	RefKey   string  `json:"ref_key"`
	Code     string  `json:"code"`
	X        float64 `json:"x"`
	Z        float64 `json:"z"`
	RotY     float64 `json:"rot_y"`
	Scale    float64 `json:"scale"`
	SizeX    float64 `json:"size_x"`
	SizeY    float64 `json:"size_y"`
	SizeZ    float64 `json:"size_z"`
	ModelSrc string  `json:"model_src"`
}

// ListLayoutTemplates GET /layout-templates
func (h *Handler) ListLayoutTemplates(c *gin.Context) {
	var list []model.LayoutTemplate
	h.DB.Order("id DESC").Find(&list)
	response.Success(c, gin.H{"list": list})
}

// SaveLayoutTemplate POST /layout-templates {name, room_id, remark}
// 从房间当前布局(含机柜覆盖+自定义构件+模型)另存为模板
func (h *Handler) SaveLayoutTemplate(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		RoomID uint   `json:"room_id"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RoomID == 0 || req.Name == "" {
		response.BadRequest(c, "name/room_id 必填")
		return
	}
	var items []model.RoomLayoutItem
	h.DB.Where("room_id = ?", req.RoomID).Order("id").Find(&items)
	if len(items) == 0 {
		response.BadRequest(c, "该房间没有已保存的自定义布局，无法存为模板")
		return
	}
	// 机柜项带上 code(套用时按编号在目标房间重映射)
	codeByID := map[uint]string{}
	if len(items) > 0 {
		var cabs []model.Cabinet
		h.DB.Where("room_id = ?", req.RoomID).Find(&cabs)
		for _, cb := range cabs {
			codeByID[cb.ID] = cb.Code
		}
	}
	tplItems := make([]layoutTplItem, 0, len(items))
	for _, it := range items {
		ti := layoutTplItem{
			RefKind: it.RefKind, RefKey: it.RefKey,
			X: it.X, Z: it.Z, RotY: it.RotY, Scale: it.Scale,
			SizeX: it.SizeX, SizeY: it.SizeY, SizeZ: it.SizeZ, ModelSrc: it.ModelSrc,
		}
		if it.RefKind == "cabinet" {
			ti.Code = codeByID[it.RefID]
		}
		tplItems = append(tplItems, ti)
	}
	raw, _ := json.Marshal(tplItems)
	tpl := model.LayoutTemplate{
		Name: req.Name, Remark: req.Remark,
		Items: string(raw), ItemCount: len(tplItems),
	}
	if err := h.DB.Create(&tpl).Error; err != nil {
		response.ServerError(c, "保存失败")
		return
	}
	response.Created(c, tpl)
}

// ApplyLayoutTemplate POST /layout-templates/:id/apply {room_id}
// 套用模板到目标房间: 先清空目标房间布局, 机柜按 code 匹配目标房间机柜重映射 ref_id(匹配不上跳过), custom/model 原样复制
func (h *Handler) ApplyLayoutTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		RoomID uint `json:"room_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RoomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	var tpl model.LayoutTemplate
	if h.DB.First(&tpl, id).Error != nil {
		response.NotFound(c, "模板不存在")
		return
	}
	var tplItems []layoutTplItem
	if err := json.Unmarshal([]byte(tpl.Items), &tplItems); err != nil {
		response.ServerError(c, "模板数据损坏")
		return
	}
	// 目标房间机柜 code→id
	var cabs []model.Cabinet
	h.DB.Where("room_id = ?", req.RoomID).Find(&cabs)
	idByCode := map[string]uint{}
	for _, cb := range cabs {
		idByCode[cb.Code] = cb.ID
	}
	// 清空目标房间旧布局
	h.DB.Where("room_id = ?", req.RoomID).Delete(&model.RoomLayoutItem{})
	applied, skipped := 0, 0
	for _, ti := range tplItems {
		item := model.RoomLayoutItem{
			RoomID: req.RoomID, RefKind: ti.RefKind, RefKey: ti.RefKey,
			X: ti.X, Z: ti.Z, RotY: ti.RotY, ModelSrc: ti.ModelSrc, Scale: ti.Scale,
			SizeX: ti.SizeX, SizeY: ti.SizeY, SizeZ: ti.SizeZ,
		}
		if ti.RefKind == "cabinet" {
			cid, ok := idByCode[ti.Code]
			if !ok {
				skipped++
				continue
			}
			item.RefID = cid
		}
		if h.DB.Create(&item).Error == nil {
			applied++
		}
	}
	response.Success(c, gin.H{"applied": applied, "skipped": skipped, "template": tpl.Name})
}

// DeleteLayoutTemplate DELETE /layout-templates/:id
func (h *Handler) DeleteLayoutTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var tpl model.LayoutTemplate
	if h.DB.First(&tpl, id).Error != nil {
		response.NotFound(c, "模板不存在")
		return
	}
	h.DB.Delete(&tpl)
	response.Success(c, gin.H{"deleted": id})
}
