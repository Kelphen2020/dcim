package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V27 3D模型库 ====================

// ListModelAssets GET /model-assets 已上传模型库
func (h *Handler) ListModelAssets(c *gin.Context) {
	var list []model.ModelAsset
	h.DB.Order("id DESC").Find(&list)
	response.Success(c, gin.H{"list": list})
}

// UploadModelAsset POST /model-assets (multipart file=*.glb/*.gltf) 上传模型入库(可复用)
func (h *Handler) UploadModelAsset(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少 file 字段")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".glb" && ext != ".gltf" {
		response.BadRequest(c, "仅支持 .glb/.gltf(CAD图纸请先经Blender等工具转GLB)")
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
	displayName := strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename))
	if n := c.PostForm("name"); n != "" {
		displayName = n
	}
	ma := model.ModelAsset{
		Name: displayName,
		Src:  "/" + filepath.ToSlash(dst),
		Ext:  ext,
		Size: fh.Size,
	}
	if err := h.DB.Create(&ma).Error; err != nil {
		response.ServerError(c, "入库失败")
		return
	}
	response.Created(c, ma)
}

// DeleteModelAsset DELETE /model-assets/:id 删除模型库项(同时删磁盘文件)
func (h *Handler) DeleteModelAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ma model.ModelAsset
	if h.DB.First(&ma, id).Error != nil {
		response.NotFound(c, "模型不存在")
		return
	}
	h.DB.Delete(&ma)
	// 删磁盘文件(相对路径, 防越界)
	if strings.HasPrefix(ma.Src, "/uploads/models/") {
		rel := strings.TrimPrefix(ma.Src, "/")
		if filepath.Base(rel) == filepath.Base(ma.Src) {
			_ = os.Remove(rel)
		}
	}
	response.Success(c, gin.H{"deleted": id})
}
