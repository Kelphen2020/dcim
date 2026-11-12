package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 网络设备 CRUD ====================

func (h *Handler) ListNetworkDevices(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	q := h.DB.Model(&model.NetworkDevice{})
	if sid := c.Query("site_id"); sid != "" {
		if id, _ := strconv.Atoi(sid); id > 0 { q = q.Where("site_id = ?", id) }
	}
	if typ := c.Query("type"); typ != "" { q = q.Where("type = ?", typ) }
	if kw := c.Query("keyword"); kw != "" { q = q.Where("name ILIKE ? OR code ILIKE ?", "%"+kw+"%", "%"+kw+"%") }
	if role != "global_admin" {
		siteIDs := h.userSiteIDs(userID)
		if len(siteIDs) == 0 { response.Success(c, gin.H{"items": []model.NetworkDevice{}, "total": 0}); return }
		q = q.Where("site_id IN ?", siteIDs)
	}
	var total int64; q.Count(&total)
	page, size := pagination(c)
	var list []model.NetworkDevice
	q.Order("site_id, type, name").Offset((page-1)*size).Limit(size).Find(&list)
	// V69/B2-4: 关联资产信息映射（前端列表/详情免二次查询）
	amap := map[uint]gin.H{}
	aids := []uint{}
	for _, d := range list { if d.AssetID != nil && *d.AssetID > 0 { aids = append(aids, *d.AssetID) } }
	if len(aids) > 0 {
		var assets []model.Asset
		h.DB.Select("id, code, name, status, ip").Where("id IN ?", aids).Find(&assets)
		for _, a := range assets { amap[a.ID] = gin.H{"code": a.Code, "name": a.Name, "status": a.Status, "ip": a.IP} }
	}
	response.Success(c, gin.H{"items": list, "total": total, "page": page, "size": size, "asset_map": amap})
}

func (h *Handler) CreateNetworkDevice(c *gin.Context) {
	var req model.NetworkDevice
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.SiteID == 0 || req.Code == "" || req.Name == "" || req.Type == "" { response.Error(c, 400, "站点/编码/名称/类型不能为空"); return }
	// V69/B2-4: 关联资产校验
	if req.AssetID != nil && *req.AssetID > 0 {
		var ac int64
		h.DB.Model(&model.Asset{}).Where("id = ?", *req.AssetID).Count(&ac)
		if ac == 0 { response.Error(c, 400, "关联资产不存在"); return }
	}
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败: "+err.Error()); return }
	go middleware.WriteAudit(h.DB, c, "create", "network_device", fmt.Sprintf("%d %s", req.ID, req.Name))
	response.Success(c, req)
}

func (h *Handler) UpdateNetworkDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var dev model.NetworkDevice
	if err := h.DB.First(&dev, id).Error; err != nil { response.Error(c, 404, "设备不存在"); return }
	// V67/B2-2: io.ReadAll 键探测（陷阱34族）——旧实现 `req.Status > 0` 导致无法把设备置为离线(0)
	raw, _ := io.ReadAll(c.Request.Body)
	var req model.NetworkDevice
	if err := json.Unmarshal(raw, &req); err != nil { response.Error(c, 400, "参数错误: "+err.Error()); return }
	var rawKeys map[string]interface{}
	_ = json.Unmarshal(raw, &rawKeys)
	has := func(k string) bool { _, ok := rawKeys[k]; return ok }
	updates := map[string]interface{}{}
	if req.Name != "" { updates["name"] = req.Name }
	if req.Type != "" { updates["type"] = req.Type }
	if req.Vendor != "" { updates["vendor"] = req.Vendor }
	if req.Model != "" { updates["model"] = req.Model }
	if req.MgmtIP != "" { updates["mgmt_ip"] = req.MgmtIP }
	if req.Firmware != "" { updates["firmware"] = req.Firmware }
	if req.Description != "" { updates["description"] = req.Description }
	if has("status") && req.Status >= 0 && req.Status <= 3 { updates["status"] = req.Status }
	// V69/B2-4: asset_id 显式携带才更新（null=解绑，局部 PUT 不清零）
	if has("asset_id") {
		if req.AssetID != nil && *req.AssetID > 0 {
			var ac int64
			h.DB.Model(&model.Asset{}).Where("id = ?", *req.AssetID).Count(&ac)
			if ac == 0 { response.Error(c, 400, "关联资产不存在"); return }
		}
		updates["asset_id"] = req.AssetID
	}
	if h.DB.Model(&dev).Updates(updates).Error != nil { response.Error(c, 500, "更新失败"); return }
	response.Success(c, nil)
}

func (h *Handler) DeleteNetworkDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Where("device_id = ?", id).Delete(&model.NetworkPort{})
	h.DB.Where("device_id = ?", id).Delete(&model.ConfigBackup{})
	h.DB.Delete(&model.NetworkDevice{}, id)
	response.Success(c, nil)
}

// ==================== 网络端口 ====================

func (h *Handler) ListNetworkPorts(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Query("device_id"))
	q := h.DB.Model(&model.NetworkPort{})
	if deviceID > 0 { q = q.Where("device_id = ?", deviceID) }
	var ports []model.NetworkPort
	q.Order("index").Find(&ports)
	response.Success(c, gin.H{"items": ports, "total": len(ports)})
}

func (h *Handler) CreateNetworkPort(c *gin.Context) {
	var req model.NetworkPort
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.DeviceID == 0 || req.Name == "" { response.Error(c, 400, "设备和端口名不能为空"); return }
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败"); return }
	// 更新设备端口数
	var pc int64
	h.DB.Model(&model.NetworkPort{}).Where("device_id = ?", req.DeviceID).Count(&pc)
	h.DB.Model(&model.NetworkDevice{}).Where("id = ?", req.DeviceID).UpdateColumn("port_count", pc)
	response.Success(c, req)
}

func (h *Handler) UpdateNetworkPort(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var port model.NetworkPort
	if err := h.DB.First(&port, id).Error; err != nil { response.Error(c, 404, "端口不存在"); return }
	var req model.NetworkPort
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	updates := map[string]interface{}{}
	if req.Name != "" { updates["name"] = req.Name }
	if req.Status > 0 { updates["status"] = req.Status }
	if req.MAC != "" { updates["mac"] = req.MAC }
	if req.VLANMode != "" { updates["vlan_mode"] = req.VLANMode }
	if req.Description != "" { updates["description"] = req.Description }
	if req.NativeVLAN != nil { updates["native_vlan"] = req.NativeVLAN }
	if req.AllowedVLANs != "" { updates["allowed_vlans"] = req.AllowedVLANs }
	h.DB.Model(&port).Updates(updates)
	response.Success(c, nil)
}

func (h *Handler) DeleteNetworkPort(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Where("port_a_id = ? OR port_b_id = ?", id, id).Delete(&model.NetworkLink{})
	h.DB.Delete(&model.NetworkPort{}, id)
	response.Success(c, nil)
}

// ==================== 链路拓扑 ====================

func (h *Handler) ListNetworkLinks(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	q := h.DB.Model(&model.NetworkLink{})
	if siteID > 0 { q = q.Where("site_id = ?", siteID) }
	var links []model.NetworkLink
	q.Order("id").Find(&links)
	response.Success(c, gin.H{"items": links, "total": len(links)})
}

func (h *Handler) CreateNetworkLink(c *gin.Context) {
	var req model.NetworkLink
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.PortAID == 0 || req.PortBID == 0 { response.Error(c, 400, "两端端口不能为空"); return }
	if req.PortAID == req.PortBID { response.Error(c, 400, "不能连接同一端口"); return }
	// 检查链路上是否已有连接
	var existing int64
	h.DB.Model(&model.NetworkLink{}).Where("(port_a_id = ? AND port_b_id = ?) OR (port_a_id = ? AND port_b_id = ?)",
		req.PortAID, req.PortBID, req.PortBID, req.PortAID).Count(&existing)
	if existing > 0 { response.Error(c, 400, "链路已存在"); return }
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败"); return }
	response.Success(c, req)
}

func (h *Handler) DeleteNetworkLink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.NetworkLink{}, id)
	response.Success(c, nil)
}

// UpdateNetworkLink V13 链路编辑（带宽/类型/状态/标签）
func (h *Handler) UpdateNetworkLink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var link model.NetworkLink
	if err := h.DB.First(&link, id).Error; err != nil {
		response.Error(c, 404, "链路不存在")
		return
	}
	body, _ := io.ReadAll(c.Request.Body)
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if v, ok := raw["bandwidth"].(string); ok && v != "" {
		updates["bandwidth"] = v
	}
	if v, ok := raw["link_type"].(string); ok && v != "" {
		updates["link_type"] = v
	}
	if v, ok := raw["label"].(string); ok {
		updates["label"] = v
	}
	if v, ok := raw["status"].(float64); ok {
		updates["status"] = int(v)
	}
	// 换端口（两端任一变更都做查重）
	newA, newB := link.PortAID, link.PortBID
	if v, ok := raw["port_a_id"].(float64); ok && uint(v) > 0 {
		newA = uint(v)
	}
	if v, ok := raw["port_b_id"].(float64); ok && uint(v) > 0 {
		newB = uint(v)
	}
	if newA != link.PortAID || newB != link.PortBID {
		if newA == newB {
			response.Error(c, 400, "不能连接同一端口")
			return
		}
		for _, pid := range []uint{newA, newB} {
			var pc int64
			h.DB.Model(&model.NetworkPort{}).Where("id = ?", pid).Count(&pc)
			if pc == 0 {
				response.Error(c, 400, "端口不存在")
				return
			}
		}
		var dup int64
		h.DB.Model(&model.NetworkLink{}).
			Where("id <> ? AND ((port_a_id = ? AND port_b_id = ?) OR (port_a_id = ? AND port_b_id = ?))",
				link.ID, newA, newB, newB, newA).Count(&dup)
		if dup > 0 {
			response.Error(c, 400, "该端口组合的链路已存在")
			return
		}
		updates["port_a_id"] = newA
		updates["port_b_id"] = newB
	}
	h.DB.Model(&link).Updates(updates)
	response.Success(c, nil)
}

// NetworkTopology 网络拓扑（设备+链路+端口）
func (h *Handler) NetworkTopology(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	var devices []model.NetworkDevice
	q := h.DB.Model(&model.NetworkDevice{})
	if siteID > 0 { q = q.Where("site_id = ?", siteID) }
	q.Find(&devices)

	deviceIDs := make([]uint, len(devices))
	for i, d := range devices { deviceIDs[i] = d.ID }

	var ports []model.NetworkPort
	if len(deviceIDs) > 0 {
		h.DB.Where("device_id IN ?", deviceIDs).Order("device_id, index").Find(&ports)
	}

	var links []model.NetworkLink
	if siteID > 0 { h.DB.Where("site_id = ?", siteID).Find(&links) }

	response.Success(c, gin.H{
		"devices": devices,
		"ports":   ports,
		"links":   links,
	})
}

// ==================== IP 地址池 ====================

func (h *Handler) ListIPPools(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	q := h.DB.Model(&model.IPPool{})
	if siteID > 0 { q = q.Where("site_id = ?", siteID) }
	var pools []model.IPPool
	q.Order("id").Find(&pools)
	// 更新已用数
	for i := range pools {
		var used int64
		h.DB.Model(&model.IPAssignment{}).Where("pool_id = ? AND status = 1", pools[i].ID).Count(&used)
		pools[i].UsedCount = int(used)
	}
	response.Success(c, gin.H{"items": pools, "total": len(pools)})
}

func (h *Handler) CreateIPPool(c *gin.Context) {
	var req model.IPPool
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.SiteID == 0 || req.Name == "" || req.CIDR == "" { response.Error(c, 400, "站点/名称/CIDR不能为空"); return }
	if _, _, err := net.ParseCIDR(req.CIDR); err != nil { response.Error(c, 400, "CIDR 格式无效"); return }
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败"); return }
	response.Success(c, req)
}

func (h *Handler) DeleteIPPool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Where("pool_id = ?", id).Delete(&model.IPAssignment{})
	h.DB.Delete(&model.IPPool{}, id)
	response.Success(c, nil)
}

func (h *Handler) UpdateIPPool(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var pool model.IPPool
	if err := h.DB.First(&pool, id).Error; err != nil { response.Error(c, 404, "IP池不存在"); return }
	var req struct {
		Name    *string `json:"name"`
		CIDR    *string `json:"cidr"`
		Gateway *string `json:"gateway"`
		DNS     *string `json:"dns"`
		Type    *string `json:"type"`
		Status  *int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	updates := map[string]interface{}{}
	if req.Name != nil && *req.Name != "" { updates["name"] = *req.Name }
	if req.CIDR != nil && *req.CIDR != "" {
		if _, _, err := net.ParseCIDR(*req.CIDR); err != nil { response.Error(c, 400, "CIDR 格式无效"); return }
		updates["cidr"] = *req.CIDR
	}
	if req.Gateway != nil { updates["gateway"] = *req.Gateway }
	if req.DNS != nil { updates["dns"] = *req.DNS }
	if req.Type != nil && *req.Type != "" { updates["type"] = *req.Type }
	if req.Status != nil && (*req.Status == 0 || *req.Status == 1) { updates["status"] = *req.Status }
	if len(updates) == 0 { response.Error(c, 400, "无可更新字段"); return }
	if err := h.DB.Model(&pool).Updates(updates).Error; err != nil { response.Error(c, 500, "更新失败"); return }
	h.DB.First(&pool, id)
	response.Success(c, pool)
}

// ==================== IP 分配 ====================

func (h *Handler) ListIPAssignments(c *gin.Context) {
	poolID, _ := strconv.Atoi(c.Query("pool_id"))
	q := h.DB.Model(&model.IPAssignment{})
	if poolID > 0 { q = q.Where("pool_id = ?", poolID) }
	if st := c.Query("status"); st != "" { q = q.Where("status = ?", st) }
	if t := c.Query("type"); t != "" { q = q.Where("type = ?", t) }
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("ip ILIKE ? OR hostname ILIKE ? OR mac ILIKE ?", like, like, like)
	}
	var items []model.IPAssignment
	q.Order("string_to_array(ip,'.')::int[]").Limit(2000).Find(&items)
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) CreateIPAssignment(c *gin.Context) {
	var req model.IPAssignment
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.PoolID == 0 || req.IP == "" { response.Error(c, 400, "池和IP不能为空"); return }
	// 检查IP是否已存在
	var existing int64
	h.DB.Model(&model.IPAssignment{}).Where("pool_id = ? AND ip = ?", req.PoolID, req.IP).Count(&existing)
	if existing > 0 { response.Error(c, 400, "IP已被分配"); return }
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败"); return }
	response.Success(c, req)
}

func (h *Handler) UpdateIPAssignment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.IPAssignment
	if err := h.DB.First(&item, id).Error; err != nil { response.Error(c, 404, "记录不存在"); return }
	var req struct {
		IP          string `json:"ip"`
		PoolID      uint   `json:"pool_id"`
		Hostname    string `json:"hostname"`
		MAC         string `json:"mac"`
		Type        string `json:"type"`
		Status      *int   `json:"status"`
		AssetID     *uint  `json:"asset_id"`
		DeviceID    *uint  `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	updates := map[string]interface{}{}
	if req.IP != "" && req.IP != item.IP {
		if net.ParseIP(req.IP) == nil { response.Error(c, 400, "IP 格式无效"); return }
		pid := item.PoolID
		if req.PoolID != 0 { pid = req.PoolID }
		var existing int64
		h.DB.Model(&model.IPAssignment{}).Where("pool_id = ? AND ip = ? AND id <> ?", pid, req.IP, id).Count(&existing)
		if existing > 0 { response.Error(c, 400, "IP已被分配"); return }
		updates["ip"] = req.IP
	}
	if req.PoolID != 0 && req.PoolID != item.PoolID {
		var existing int64
		ip := item.IP
		if req.IP != "" { ip = req.IP }
		h.DB.Model(&model.IPAssignment{}).Where("pool_id = ? AND ip = ?", req.PoolID, ip).Count(&existing)
		if existing > 0 { response.Error(c, 400, "目标池中该IP已被分配"); return }
		updates["pool_id"] = req.PoolID
	}
	if req.Hostname != "" { updates["hostname"] = req.Hostname }
	if req.MAC != "" { updates["mac"] = req.MAC }
	if req.Type != "" { updates["type"] = req.Type }
	if req.Status != nil && *req.Status >= 0 && *req.Status <= 2 { updates["status"] = *req.Status }
	if req.AssetID != nil { updates["asset_id"] = *req.AssetID }
	if req.DeviceID != nil { updates["device_id"] = *req.DeviceID }
	if len(updates) == 0 { response.Error(c, 400, "无可更新字段"); return }
	h.DB.Model(&item).Updates(updates)
	h.DB.First(&item, id)
	response.Success(c, item)
}

func (h *Handler) DeleteIPAssignment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.IPAssignment{}, id)
	response.Success(c, nil)
}

// ==================== VLAN 管理 ====================

func (h *Handler) ListVLANs(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	q := h.DB.Model(&model.VLAN{})
	if siteID > 0 { q = q.Where("site_id = ?", siteID) }
	var vlans []model.VLAN
	q.Order("vid").Find(&vlans)
	if vlans == nil { vlans = []model.VLAN{} }
	response.Success(c, gin.H{"items": vlans, "total": len(vlans)})
}

func (h *Handler) CreateVLAN(c *gin.Context) {
	var req model.VLAN
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.SiteID == 0 || req.VID == 0 || req.Name == "" { response.Error(c, 400, "站点/VID/名称不能为空"); return }
	if req.VID < 1 || req.VID > 4094 { response.Error(c, 400, "VID范围1-4094"); return }
	// 检查VID是否重复
	var existing int64
	h.DB.Model(&model.VLAN{}).Where("site_id = ? AND vid = ?", req.SiteID, req.VID).Count(&existing)
	if existing > 0 { response.Error(c, 400, "该站点VID已存在"); return }
	if err := h.DB.Create(&req).Error; err != nil { response.Error(c, 500, "创建失败"); return }
	response.Success(c, req)
}

func (h *Handler) UpdateVLAN(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var vlan model.VLAN
	if err := h.DB.First(&vlan, id).Error; err != nil { response.Error(c, 404, "VLAN不存在"); return }
	var req model.VLAN
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	updates := map[string]interface{}{}
	if req.Name != "" { updates["name"] = req.Name }
	if req.Description != "" { updates["description"] = req.Description }
	if req.Color != "" { updates["color"] = req.Color }
	if req.Subnet != "" { updates["subnet"] = req.Subnet }
	if req.Gateway != "" { updates["gateway"] = req.Gateway }
	if req.Status > 0 { updates["status"] = req.Status }
	h.DB.Model(&vlan).Updates(updates)
	response.Success(c, nil)
}

func (h *Handler) DeleteVLAN(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.VLAN{}, id)
	response.Success(c, nil)
}

// ==================== 配置备份 ====================

func (h *Handler) ListConfigBackups(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Query("device_id"))
	q := h.DB.Model(&model.ConfigBackup{})
	if deviceID > 0 { q = q.Where("device_id = ?", deviceID) }
	var items []model.ConfigBackup
	q.Order("id DESC").Limit(50).Find(&items)
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) CreateConfigBackup(c *gin.Context) {
	var req struct {
		DeviceID uint   `json:"device_id"`
		Content  string `json:"content"`
		Version  string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.DeviceID == 0 { response.Error(c, 400, "设备ID不能为空"); return }
	backup := model.ConfigBackup{
		DeviceID:   req.DeviceID,
		Filename:   fmt.Sprintf("config_%d_%s.txt", req.DeviceID, time.Now().Format("20060102_150405")),
		Content:    req.Content,
		Size:       int64(len(req.Content)),
		Version:    req.Version,
		BackupType: "manual",
		Status:     1,
		BackedUpAt: time.Now(),
	}
	if err := h.DB.Create(&backup).Error; err != nil { response.Error(c, 500, "备份失败"); return }
	// V67/B2-2: 备份成功 = 设备可达，顺带 touch 心跳（离线判定数据源）
	h.DB.Model(&model.NetworkDevice{}).Where("id = ?", req.DeviceID).Update("last_seen_at", time.Now())
	response.Success(c, backup)
}

func (h *Handler) GetConfigBackup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var backup model.ConfigBackup
	if err := h.DB.First(&backup, id).Error; err != nil { response.Error(c, 404, "备份不存在"); return }
	response.Success(c, backup)
}

func (h *Handler) DeleteConfigBackup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.DB.Delete(&model.ConfigBackup{}, id)
	response.Success(c, nil)
}

// ==================== 网络摘要 ====================

func (h *Handler) NetworkSummary(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	q := h.DB.Model(&model.NetworkDevice{})
	if role != "global_admin" {
		siteIDs := h.userSiteIDs(userID)
		if len(siteIDs) > 0 { q = q.Where("site_id IN ?", siteIDs) }
	}
	var totalDevices, online, offline int64
	q.Count(&totalDevices)
	q.Where("status = 1").Count(&online)
	q.Where("status = 0").Count(&offline)

	var totalLinks int64
	h.DB.Model(&model.NetworkLink{}).Count(&totalLinks)

	var totalPools int64
	h.DB.Model(&model.IPPool{}).Count(&totalPools)

	var totalVLANs int64
	h.DB.Model(&model.VLAN{}).Count(&totalVLANs)

	response.Success(c, gin.H{
		"devices_total": totalDevices, "devices_online": online, "devices_offline": offline,
		"links_total": totalLinks, "pools_total": totalPools, "vlans_total": totalVLANs,
	})
}

// DevicePoints V66 B2-1: 网络设备绑定的测点与实时读数（设备详情抽屉用）
func (h *Handler) DevicePoints(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var dev model.NetworkDevice
	if err := h.DB.First(&dev, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "设备不存在")
		return
	}
	var points []model.MonitoringPoint
	h.DB.Where("network_device_id = ?", id).Order("type, name").Find(&points)
	if points == nil {
		points = []model.MonitoringPoint{}
	}
	var alarming, offline int
	for _, p := range points {
		if p.Status == 2 || p.Status == 3 {
			alarming++
		}
		if p.Status == 0 {
			offline++
		}
	}
	response.Success(c, gin.H{
		"device":   dev,
		"points":   points,
		"total":    len(points),
		"alarming": alarming,
		"offline":  offline,
	})
}