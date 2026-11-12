package api

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V26 3D 自定义布局 ====================

// GetRoomLayout GET /room3d/layout?room_id= 房间的自定义布局项
func (h *Handler) GetRoomLayout(c *gin.Context) {
	roomID, _ := strconv.Atoi(c.Query("room_id"))
	var items []model.RoomLayoutItem
	q := h.DB.Order("id")
	if roomID > 0 {
		q = q.Where("room_id = ?", roomID)
	}
	q.Find(&items)
	response.Success(c, gin.H{"list": items})
}

// SaveRoomLayout PUT /room3d/layout 批量 upsert 布局项(拖拽保存)
func (h *Handler) SaveRoomLayout(c *gin.Context) {
	var req struct {
		RoomID uint   `json:"room_id"`
		Items  []struct {
			RefKind  string  `json:"ref_kind"`
			RefID    uint    `json:"ref_id"`
			RefKey   string  `json:"ref_key"`
			X        float64 `json:"x"`
			Z        float64 `json:"z"`
			RotY     float64 `json:"rot_y"`
			ModelSrc string  `json:"model_src"`
			Scale    float64 `json:"scale"`
			SizeX    float64 `json:"size_x"`
			SizeY    float64 `json:"size_y"`
			SizeZ    float64 `json:"size_z"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RoomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	saved := 0
	for _, it := range req.Items {
		if it.RefKind == "" {
			continue
		}
		var item model.RoomLayoutItem
		res := h.DB.Where("room_id = ? AND ref_kind = ? AND ref_id = ? AND ref_key = ?",
			req.RoomID, it.RefKind, it.RefID, it.RefKey).First(&item)
		if res.Error != nil {
			item = model.RoomLayoutItem{RoomID: req.RoomID, RefKind: it.RefKind, RefID: it.RefID, RefKey: it.RefKey}
		}
		item.X, item.Z, item.RotY = it.X, it.Z, it.RotY
		if it.ModelSrc != "" {
			item.ModelSrc = it.ModelSrc
		}
		if it.Scale > 0 {
			item.Scale = it.Scale
		}
		item.SizeX, item.SizeY, item.SizeZ = it.SizeX, it.SizeY, it.SizeZ
		h.DB.Save(&item)
		saved++
	}
	response.Success(c, gin.H{"saved": saved})
}

// DeleteRoomLayout DELETE /room3d/layout/:id 删除单个自定义位置(恢复自动布局)
func (h *Handler) DeleteRoomLayout(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.RoomLayoutItem{}, id)
	response.Success(c, gin.H{"deleted": id})
}

// ResetRoomLayout DELETE /room3d/layout?room_id= 清空房间全部自定义布局
func (h *Handler) ResetRoomLayout(c *gin.Context) {
	roomID, _ := strconv.Atoi(c.Query("room_id"))
	if roomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	h.DB.Where("room_id = ?", roomID).Delete(&model.RoomLayoutItem{})
	response.Success(c, gin.H{"reset": roomID})
}

// PlanHint GET /room3d/plan-hint?room_id= 按 GB 50174 面积推算建议机柜数
// 主机房面积 = 房间面积 × R_IT(GB比例)；机柜数 = 主机房面积 / 密度(默认4.5㎡/台, 含通道)
func (h *Handler) PlanHint(c *gin.Context) {
	roomID, _ := strconv.Atoi(c.Query("room_id"))
	density, _ := strconv.ParseFloat(c.DefaultQuery("density", "4.5"), 64)
	if density <= 0 {
		density = 4.5
	}
	var room model.Room
	if h.DB.First(&room, roomID).Error != nil {
		response.NotFound(c, "房间不存在")
		return
	}
	ratioAux := 1.5
	if room.Area >= 300 {
		ratioAux = 2.0
	}
	rIT := 1 / (1 + ratioAux)
	itArea := room.Area * rIT
	suggest := int(math.Floor(itArea / density))
	var current int64
	h.DB.Model(&model.Cabinet{}).Where("room_id = ?", roomID).Count(&current)
	response.Success(c, gin.H{
		"room_area":    room.Area,
		"it_area":      math.Round(itArea*10) / 10,
		"density":      density,
		"suggest_cabs": suggest,
		"current_cabs": current,
		"gap":          suggest - int(current),
	})
}

// PlanCabinets POST /room3d/plan-cabinets {room_id, density} 按面积一键补足规划机柜(status=2预留)
func (h *Handler) PlanCabinets(c *gin.Context) {
	var req struct {
		RoomID  uint    `json:"room_id"`
		Density float64 `json:"density"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RoomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	if req.Density <= 0 {
		req.Density = 4.5
	}
	var room model.Room
	if h.DB.First(&room, req.RoomID).Error != nil {
		response.NotFound(c, "房间不存在")
		return
	}
	ratioAux := 1.5
	if room.Area >= 300 {
		ratioAux = 2.0
	}
	itArea := room.Area / (1 + ratioAux)
	suggest := int(math.Floor(itArea / req.Density))
	var current int64
	h.DB.Model(&model.Cabinet{}).Where("room_id = ?", req.RoomID).Count(&current)
	gap := suggest - int(current)
	if gap <= 0 {
		response.Success(c, gin.H{"created": 0, "suggest": suggest, "current": current, "msg": "机柜数已达规划值"})
		return
	}
	// 现有无列机柜(孤儿)也纳入排列
	var orphans []model.Cabinet
	h.DB.Where("room_id = ? AND (row_id = 0 OR row_id IS NULL)", req.RoomID).Find(&orphans)
	total := gap + len(orphans)
	const perRow = 8 // 每列 8 台(0.66m 间距 ≈ 5.3m 列长, 适配主流机房柱距)
	nNewRows := (total + perRow - 1) / perRow

	// 已有列的最大字母
	var existRows []model.Row
	h.DB.Where("room_id = ?", req.RoomID).Order("id").Find(&existRows)
	maxLetter := byte('A') - 1
	for _, r := range existRows {
		if len(r.Name) > 0 && r.Name[0] >= 'A' && r.Name[0] <= 'Z' && r.Name[0] > maxLetter {
			maxLetter = r.Name[0]
		}
	}
	// 创建新列(冷热通道交替, 首列接现有列的通道极性)
	lastCh := "hot"
	if len(existRows) > 0 && existRows[len(existRows)-1].Channel != "" {
		lastCh = existRows[len(existRows)-1].Channel
	}
	var newRows []model.Row
	for i := 0; i < nNewRows; i++ {
		ch := "cold"
		if lastCh == "cold" {
			ch = "hot"
		}
		lastCh = ch
		row := model.Row{
			SiteID: room.SiteID, RoomID: req.RoomID,
			Name:    string(rune(maxLetter+1+byte(i))) + "列",
			Channel: ch,
			Remark:  "V26 面积规划",
		}
		if h.DB.Create(&row).Error == nil {
			newRows = append(newRows, row)
		}
	}
	// 分配槽位: 先孤儿后新建
	placed, created := 0, 0
	now := time.Now()
	assign := func(cab *model.Cabinet) {
		if len(newRows) == 0 {
			return
		}
		ri := placed / perRow
		if ri >= len(newRows) {
			ri = len(newRows) - 1
		}
		cab.RowID = newRows[ri].ID
		cab.Code = fmt.Sprintf("%c%02d", newRows[ri].Name[0], placed%perRow+1)
		placed++
	}
	for i := range orphans {
		o := orphans[i]
		assign(&o)
		h.DB.Model(&o).Updates(map[string]interface{}{"row_id": o.RowID, "code": o.Code})
	}
	for i := 0; i < gap; i++ {
		cab := model.Cabinet{
			SiteID:       room.SiteID,
			RoomID:       req.RoomID,
			TotalU:       42,
			RatedPowerKW: 6,
			MaxWeightKG:  1000,
			Status:       2, // 预留(规划位)
			Remark:       fmt.Sprintf("V26 面积规划自动生成(%.1f㎡/台)", req.Density),
			CreatedAt:    now,
		}
		assign(&cab)
		if h.DB.Create(&cab).Error == nil {
			created++
		}
	}
	response.Success(c, gin.H{"created": created, "rows_created": len(newRows), "reassigned": len(orphans), "suggest": suggest, "current": current + int64(created)})
}

// UploadModel POST /room3d/model-upload (multipart, file=*.glb/*.gltf) 导入3D模型文件
// 存 uploads/models/，返回可访问 URL —— 拖入场景后由布局项 model_src 引用
func (h *Handler) UploadModel(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少 file 字段")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".glb" && ext != ".gltf" {
		response.BadRequest(c, "仅支持 .glb/.gltf 模型(CAD图纸请先经Blender等工具转GLB)")
		return
	}
	if fh.Size > 64<<20 {
		response.BadRequest(c, "模型文件超过64MB限制")
		return
	}
	dir := "uploads/models"
	_ = os.MkdirAll(dir, 0o755)
	name := fmt.Sprintf("m%d%s", time.Now().UnixMilli(), ext)
	dst := filepath.Join(dir, name)
	src, err := fh.Open()
	if err != nil {
		response.ServerError(c, "读取失败")
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		response.ServerError(c, "写入失败")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		response.ServerError(c, "保存失败")
		return
	}
	response.Success(c, gin.H{
		"url":  "/" + filepath.ToSlash(dst),
		"name": fh.Filename,
		"size": fh.Size,
	})
}
