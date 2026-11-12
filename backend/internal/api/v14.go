package api

import (
	"fmt"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V14 业务关联深化 ====================

// ---------- 供电链路富化 ----------

type powerChainView struct {
	model.PowerChain
	AssetName    string   `json:"asset_name"`
	AssetCode    string   `json:"asset_code"`
	CabinetCode  string   `json:"cabinet_code"`
	PointName    string   `json:"point_name"`
	PointValue   *float64 `json:"point_value"`   // 绑定测点的实时读数
	PointStatus  int      `json:"point_status"`  // 测点状态(2=告警)
	LoadSource   string   `json:"load_source"`   // point=测点实时 / manual=手工填报
	EffectiveKW  float64  `json:"effective_kw"`  // 生效负载(测点优先)
	LoadPct      float64  `json:"load_pct"`      // 负载率%(按 0.9 功率因数)
	ChildrenKW   float64  `json:"children_kw"`   // 下级负载汇总(不含自身)
	ActiveAlerts int64    `json:"active_alerts"` // 绑定测点的活跃告警数
}

// powerChainSubtreeKW 递归汇总子孙节点的生效负载
func powerChainSubtreeKW(id uint, nodes []powerChainView) float64 {
	sum := 0.0
	for _, n := range nodes {
		if n.ParentID == id {
			sum += n.EffectiveKW + powerChainSubtreeKW(n.ID, nodes)
		}
	}
	return sum
}

// ListPowerChainsRich 富化供电链路（数组形态，兼容 PowerSld 直接消费 r.data）
func (h *Handler) ListPowerChainsRich(c *gin.Context) {
	q := h.DB.Model(&model.PowerChain{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	var items []model.PowerChain
	q.Order("id ASC").Find(&items)
	if items == nil {
		items = []model.PowerChain{}
	}

	// 批量取关联对象
	assetIDs, pointIDs, cabIDs := []uint{}, []uint{}, []uint{}
	for _, it := range items {
		if it.AssetID > 0 {
			assetIDs = append(assetIDs, it.AssetID)
		}
		if it.PointID > 0 {
			pointIDs = append(pointIDs, it.PointID)
		}
		if it.CabinetID > 0 {
			cabIDs = append(cabIDs, it.CabinetID)
		}
	}
	assets := map[uint]model.Asset{}
	if len(assetIDs) > 0 {
		var as []model.Asset
		h.DB.Where("id IN ?", assetIDs).Find(&as)
		for _, a := range as {
			assets[a.ID] = a
		}
	}
	points := map[uint]model.MonitoringPoint{}
	if len(pointIDs) > 0 {
		var ps []model.MonitoringPoint
		h.DB.Where("id IN ?", pointIDs).Find(&ps)
		for _, p := range ps {
			points[p.ID] = p
		}
	}
	cabs := map[uint]string{}
	if len(cabIDs) > 0 {
		var cs []model.Cabinet
		h.DB.Select("id", "code").Where("id IN ?", cabIDs).Find(&cs)
		for _, cb := range cs {
			cabs[cb.ID] = cb.Code
		}
	}

	views := make([]powerChainView, 0, len(items))
	for _, it := range items {
		v := powerChainView{PowerChain: it}
		if a, ok := assets[it.AssetID]; ok {
			v.AssetName, v.AssetCode = a.Name, a.Code
		}
		v.CabinetCode = cabs[it.CabinetID]
		v.LoadSource = "manual"
		v.EffectiveKW = it.LoadKW
		if p, ok := points[it.PointID]; ok && p.CurrentValue != nil {
			v.PointName = p.Name
			v.PointValue = p.CurrentValue
			v.PointStatus = p.Status
			// 功率/电流类测点读数视为 kW 负载来源（电流×220V×3/1000 由测点缩放层处理）
			v.EffectiveKW = *p.CurrentValue
			v.LoadSource = "point"
			var ac int64
			h.DB.Model(&model.Alert{}).Where("point_id = ? AND status = 0", p.ID).Count(&ac)
			v.ActiveAlerts = ac
		}
		if it.RatedKVA > 0 {
			v.LoadPct = v.EffectiveKW / (it.RatedKVA * 0.9) * 100
			if v.LoadPct > 100 {
				v.LoadPct = 100
			}
		}
		views = append(views, v)
	}
	for i := range views {
		views[i].ChildrenKW = powerChainSubtreeKW(views[i].ID, views)
	}
	response.Success(c, views)
}

// PowerChainImpact 停电影响分析：某节点停电波及的下级设备/机柜/资产/测点/告警
func (h *Handler) PowerChainImpact(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var root model.PowerChain
	if err := h.DB.First(&root, id).Error; err != nil {
		response.NotFound(c, "供电节点不存在")
		return
	}
	var all []model.PowerChain
	h.DB.Where("site_id = ?", root.SiteID).Find(&all)

	// 收集子树
	subtree := []model.PowerChain{root}
	var walk func(pid uint)
	walk = func(pid uint) {
		for _, n := range all {
			if n.ParentID == pid {
				subtree = append(subtree, n)
				walk(n.ID)
			}
		}
	}
	walk(root.ID)

	// 上游供电路径
	upstream := []gin.H{}
	cur := root
	for i := 0; i < 50 && cur.ParentID != 0; i++ {
		var p model.PowerChain
		if h.DB.First(&p, cur.ParentID).Error != nil {
			break
		}
		upstream = append([]gin.H{{"id": p.ID, "name": p.Name, "kind": p.Kind}}, upstream...)
		cur = p
	}

	// 波及机柜：子树节点直接绑定的机柜 + 子树节点关联资产所在机柜
	cabSet := map[uint]bool{}
	for _, n := range subtree {
		if n.CabinetID > 0 {
			cabSet[n.CabinetID] = true
		}
	}
	// PDU 类资产绑定 → 该资产所在机柜
	assetIDs := []uint{}
	for _, n := range subtree {
		if n.AssetID > 0 {
			assetIDs = append(assetIDs, n.AssetID)
		}
	}
	if len(assetIDs) > 0 {
		var as []model.Asset
		h.DB.Select("id", "cabinet_id").Where("id IN ? AND cabinet_id > 0", assetIDs).Find(&as)
		for _, a := range as {
			cabSet[a.CabinetID] = true
		}
	}
	cabIDs := make([]uint, 0, len(cabSet))
	for cid := range cabSet {
		cabIDs = append(cabIDs, cid)
	}

	// 波及资产
	type assetRow struct {
		ID        uint    `json:"id"`
		Code      string  `json:"code"`
		Name      string  `json:"name"`
		Category  string  `json:"category"`
		CabinetID uint    `json:"cabinet_id"`
		StartU    int     `json:"start_u"`
		PowerW    int     `json:"power_w"`
		IP        string  `json:"ip"`
		Status    int     `json:"status"`
	}
	assets := []assetRow{}
	totalW := 0
	if len(cabIDs) > 0 {
		h.DB.Model(&model.Asset{}).Select("id, code, name, category, cabinet_id, start_u, power_w, ip, status").
			Where("cabinet_id IN ? AND status = 1", cabIDs).Order("cabinet_id, start_u").Scan(&assets)
		if assets == nil {
			assets = []assetRow{}
		}
		for _, a := range assets {
			totalW += a.PowerW
		}
	}

	// 波及机柜明细
	type cabRow struct {
		ID        uint   `json:"id"`
		Code      string `json:"code"`
		RoomID    uint   `json:"room_id"`
		TotalU    int    `json:"total_u"`
		AssetCnt  int64  `json:"asset_cnt"`
		PowerKW   float64 `json:"power_kw"`
	}
	cabRows := []cabRow{}
	if len(cabIDs) > 0 {
		h.DB.Model(&model.Cabinet{}).Select("id, code, room_id, total_u").Where("id IN ?", cabIDs).Order("code").Scan(&cabRows)
		if cabRows == nil {
			cabRows = []cabRow{}
		}
		for i := range cabRows {
			h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", cabRows[i].ID).Count(&cabRows[i].AssetCnt)
			var w int64
			h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", cabRows[i].ID).
				Select("COALESCE(SUM(power_w),0)").Scan(&w)
			cabRows[i].PowerKW = float64(w) / 1000.0
		}
	}

	// 波及测点与活跃告警
	var ptCnt int64
	var alertCnt int64
	if len(cabIDs) > 0 {
		h.DB.Model(&model.MonitoringPoint{}).Where("cabinet_id IN ?", cabIDs).Count(&ptCnt)
		h.DB.Model(&model.Alert{}).Where("site_id = ? AND status < 3", root.SiteID).
			Where("point_id IN (SELECT id FROM t_monitoring_point WHERE cabinet_id IN ?)", cabIDs).Count(&alertCnt)
	}

	// 子树节点摘要
	nodes := make([]gin.H, 0, len(subtree))
	for _, n := range subtree {
		nodes = append(nodes, gin.H{"id": n.ID, "name": n.Name, "kind": n.Kind, "rated_kva": n.RatedKVA, "load_kw": n.LoadKW, "status": n.Status})
	}

	response.Success(c, gin.H{
		"root":          root,
		"upstream":      upstream,
		"nodes":         nodes,
		"node_count":    len(subtree),
		"cabinets":      cabRows,
		"cabinet_count": len(cabRows),
		"assets":        assets,
		"asset_count":   len(assets),
		"total_power_w": totalW,
		"point_count":   ptCnt,
		"active_alerts": alertCnt,
		"generated_at":  time.Now(),
	})
}

// ---------- 机柜容量深化 ----------

// CapacityOverview 全机柜容量总览（热力矩阵数据源）
func (h *Handler) CapacityOverview(c *gin.Context) {
	q := h.DB.Model(&model.Cabinet{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if s := c.Query("site_id"); s != "" && s != "0" {
		q = q.Where("site_id = ?", s)
	}
	var cabs []model.Cabinet
	q.Order("room_id, code").Find(&cabs)
	if cabs == nil {
		cabs = []model.Cabinet{}
	}
	roomNames := map[uint]string{}
	{
		var rooms []model.Room
		h.DB.Select("id", "name").Find(&rooms)
		for _, r := range rooms {
			roomNames[r.ID] = r.Name
		}
	}
	type cabStat struct {
		ID          uint    `json:"id"`
		SiteID      uint    `json:"site_id"`
		Code        string  `json:"code"`
		RoomID      uint    `json:"room_id"`
		RoomName    string  `json:"room_name"`
		TotalU      int     `json:"total_u"`
		UsedU       int     `json:"used_u"`
		UPct        float64 `json:"u_pct"`
		RatedKW     float64 `json:"rated_kw"`
		UsedKW      float64 `json:"used_kw"`
		PowerPct    float64 `json:"power_pct"`
		MaxWeight   float64 `json:"max_weight_kg"`
		UsedWeight  float64 `json:"used_weight_kg"`
		AssetCnt    int64   `json:"asset_cnt"`
		Alerts      int64   `json:"alerts"`
		TempValue   *float64 `json:"temp_value"`
	}
	out := make([]cabStat, 0, len(cabs))
	for _, cb := range cabs {
		st := cabStat{
			ID: cb.ID, SiteID: cb.SiteID, Code: cb.Code, RoomID: cb.RoomID, RoomName: roomNames[cb.RoomID],
			TotalU: cb.TotalU, RatedKW: cb.RatedPowerKW, MaxWeight: float64(cb.MaxWeightKG),
		}
		var rows []struct {
			U    int
			W    int
			KG   float64
			N    int64
		}
		h.DB.Model(&model.Asset{}).
			Select("COALESCE(SUM(height_u),0) as u, COALESCE(SUM(power_w),0) as w, COALESCE(COALESCE(SUM(weight_kg),0),0) as kg, COUNT(*) as n").
			Where("cabinet_id = ? AND status = 1", cb.ID).Scan(&rows)
		if len(rows) > 0 {
			st.UsedU = rows[0].U
			st.UsedKW = float64(rows[0].W) / 1000.0
			st.UsedWeight = rows[0].KG
			st.AssetCnt = rows[0].N
		}
		if st.TotalU > 0 {
			st.UPct = float64(st.UsedU) / float64(st.TotalU) * 100
		}
		if st.RatedKW > 0 {
			st.PowerPct = st.UsedKW / st.RatedKW * 100
		}
		h.DB.Model(&model.Alert{}).
			Where("status = 0 AND point_id IN (SELECT id FROM t_monitoring_point WHERE cabinet_id = ?)", cb.ID).
			Count(&st.Alerts)
		var tp model.MonitoringPoint
		if h.DB.Select("current_value").Where("cabinet_id = ? AND type = 'temp_humidity' AND current_value IS NOT NULL", cb.ID).
			Order("id").First(&tp).Error == nil {
			st.TempValue = tp.CurrentValue
		} else if cb.RoomID > 0 {
			// 回退: 机房级温湿度测点(未绑定到具体机柜)
			var rp model.MonitoringPoint
			if h.DB.Select("current_value").
				Where("room_id = ? AND cabinet_id IS NULL AND type = 'temp_humidity' AND current_value IS NOT NULL", cb.RoomID).
				Order("id").First(&rp).Error == nil {
				st.TempValue = rp.CurrentValue
			}
		}
		out = append(out, st)
	}
	response.Success(c, gin.H{"items": out, "total": len(out)})
}

// ---------- 网络拓扑富化 ----------

// NetworkTopologyRich 设备+端口统计+资产/备份/IP 业务关联
func (h *Handler) NetworkTopologyRich(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	var devices []model.NetworkDevice
	q := h.DB.Model(&model.NetworkDevice{})
	if siteID > 0 {
		q = q.Where("site_id = ?", siteID)
	}
	q.Find(&devices)
	if devices == nil {
		devices = []model.NetworkDevice{}
	}
	deviceIDs := make([]uint, len(devices))
	for i, d := range devices {
		deviceIDs[i] = d.ID
	}
	var ports []model.NetworkPort
	var links []model.NetworkLink
	if len(deviceIDs) > 0 {
		h.DB.Where("device_id IN ?", deviceIDs).Order("device_id, index").Find(&ports)
	}
	lq := h.DB.Model(&model.NetworkLink{})
	if siteID > 0 {
		lq = lq.Where("site_id = ?", siteID)
	}
	lq.Find(&links)
	if ports == nil {
		ports = []model.NetworkPort{}
	}
	if links == nil {
		links = []model.NetworkLink{}
	}

	// 端口被链路占用集合
	linkedPorts := map[uint]bool{}
	for _, l := range links {
		linkedPorts[l.PortAID] = true
		linkedPorts[l.PortBID] = true
	}

	type devView struct {
		model.NetworkDevice
		PortTotal   int    `json:"port_total"`
		PortUp      int    `json:"port_up"`
		PortLinked  int    `json:"port_linked"`
		AssetName   string `json:"asset_name"` // 台账资产(按管理IP匹配)
		AssetID     uint   `json:"asset_id"`
		BackupCnt   int64  `json:"backup_cnt"`
		LastBackup  string `json:"last_backup"`
		IPCnt       int64  `json:"ip_cnt"` // IPAM 分配的IP数(按设备关联)
		CabinetCode string `json:"cabinet_code"`
	}
	views := make([]devView, 0, len(devices))
	for _, d := range devices {
		v := devView{NetworkDevice: d}
		for _, p := range ports {
			if p.DeviceID != d.ID {
				continue
			}
			v.PortTotal++
			if p.Status == 1 {
				v.PortUp++
			}
			if linkedPorts[p.ID] {
				v.PortLinked++
			}
		}
		if d.MgmtIP != "" {
			var a model.Asset
			if h.DB.Select("id", "name").Where("ip = ?", d.MgmtIP).First(&a).Error == nil {
				v.AssetName, v.AssetID = a.Name, a.ID
			}
		}
		h.DB.Model(&model.ConfigBackup{}).Where("device_id = ?", d.ID).Count(&v.BackupCnt)
		var lb model.ConfigBackup
		if h.DB.Select("backed_up_at").Where("device_id = ? AND status = 1", d.ID).
			Order("backed_up_at DESC").First(&lb).Error == nil {
			v.LastBackup = lb.BackedUpAt.Format("2006-01-02 15:04")
		}
		h.DB.Model(&model.IPAssignment{}).Where("device_id = ?", d.ID).Count(&v.IPCnt)
		if d.CabinetID != nil && *d.CabinetID > 0 {
			var cb model.Cabinet
			if h.DB.Select("code").First(&cb, *d.CabinetID).Error == nil {
				v.CabinetCode = cb.Code
			}
		}
		views = append(views, v)
	}
	response.Success(c, gin.H{"devices": views, "ports": ports, "links": links})
}

// DevicePorts 单设备端口明细（拓扑抽屉用，含每端口链路对端）
func (h *Handler) DevicePorts(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ports []model.NetworkPort
	h.DB.Where("device_id = ?", id).Order("index, id").Find(&ports)
	if ports == nil {
		ports = []model.NetworkPort{}
	}
	portIDs := make([]uint, len(ports))
	for i, p := range ports {
		portIDs[i] = p.ID
	}
	var links []model.NetworkLink
	if len(portIDs) > 0 {
		h.DB.Where("port_a_id IN ? OR port_b_id IN ?", portIDs, portIDs).Find(&links)
	}
	var allPorts []model.NetworkPort
	var allDevs []model.NetworkDevice
	h.DB.Find(&allPorts)
	h.DB.Find(&allDevs)
	portDev := map[uint]uint{}
	portName := map[uint]string{}
	for _, p := range allPorts {
		portDev[p.ID] = p.DeviceID
		portName[p.ID] = p.Name
	}
	devName := map[uint]string{}
	for _, d := range allDevs {
		devName[d.ID] = d.Name
	}
	type portView struct {
		model.NetworkPort
		PeerDevice string `json:"peer_device"`
		PeerPort   string `json:"peer_port"`
		LinkStatus int    `json:"link_status"` // -1 未连线
		Bandwidth  string `json:"bandwidth"`
	}
	out := make([]portView, 0, len(ports))
	for _, p := range ports {
		v := portView{NetworkPort: p, LinkStatus: -1}
		for _, l := range links {
			var peer uint
			if l.PortAID == p.ID {
				peer = l.PortBID
			} else if l.PortBID == p.ID {
				peer = l.PortAID
			} else {
				continue
			}
			v.PeerDevice = devName[portDev[peer]]
			v.PeerPort = portName[peer]
			v.LinkStatus = l.Status
			v.Bandwidth = l.Bandwidth
			break
		}
		out = append(out, v)
	}
	response.Success(c, gin.H{"items": out, "total": len(out)})
}

// ---------- 机柜详情富化（容量页上下文面板） ----------

// CabinetContext 机柜业务上下文：实时温湿度/告警/维保/工单/网络设备
func (h *Handler) CabinetContext(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var cab model.Cabinet
	if err := h.DB.First(&cab, id).Error; err != nil {
		response.NotFound(c, "机柜不存在")
		return
	}
	// 关联测点与实时值（机柜级 + 所在机房级回退，测点未绑柜时按机房提供环境参考）
	var points []model.MonitoringPoint
	h.DB.Where("cabinet_id = ?", id).Find(&points)
	if points == nil {
		points = []model.MonitoringPoint{}
	}
	if cab.RoomID > 0 {
		var roomPts []model.MonitoringPoint
		h.DB.Where("room_id = ? AND cabinet_id IS NULL", cab.RoomID).Find(&roomPts)
		points = append(points, roomPts...)
	}
	// 活跃告警（该柜测点）
	type alertRow struct {
		ID       uint      `json:"id"`
		Severity int       `json:"severity"`
		Message  string    `json:"message"`
		Status   int       `json:"status"`
		PointName string   `json:"point_name"`
		TriggeredAt time.Time `json:"triggered_at"`
	}
	alerts := []alertRow{}
	h.DB.Model(&model.Alert{}).Select("t_alert.id, severity, message, status, triggered_at, COALESCE(p.name,'') as point_name").
		Joins("LEFT JOIN t_monitoring_point p ON p.id = t_alert.point_id").
		Where("t_alert.status < 3 AND t_alert.point_id IN (SELECT id FROM t_monitoring_point WHERE cabinet_id = ?)", id).
		Order("triggered_at DESC").Limit(20).Scan(&alerts)
	if alerts == nil {
		alerts = []alertRow{}
	}
	// 在架资产维保 30 天内到期
	type maintRow struct {
		AssetName string `json:"asset_name"`
		EndAt     string `json:"end_at"`
		DaysLeft  int    `json:"days_left"`
	}
	maints := []maintRow{}
	{
		var rows []struct {
			AssetName string
			EndAt     time.Time
		}
		h.DB.Table("t_maintenance m").Select("COALESCE(a.name,'') as asset_name, m.end_at").
			Joins("JOIN t_asset a ON a.id = m.asset_id").
			Where("a.cabinet_id = ? AND m.end_at IS NOT NULL AND m.end_at > now() AND m.end_at < now() + interval '30 days'", id).
			Scan(&rows)
		for _, r := range rows {
			maints = append(maints, maintRow{AssetName: r.AssetName, EndAt: r.EndAt.Format("2006-01-02"), DaysLeft: int(time.Until(r.EndAt).Hours() / 24)})
		}
		if maints == nil {
			maints = []maintRow{}
		}
	}
	// 关联未完成工单
	var woCnt int64
	h.DB.Model(&model.WorkOrder{}).
		Where("status < 3 AND description LIKE ?", fmt.Sprintf("%%%s%%", cab.Code)).Count(&woCnt)
	// 柜内网络设备
	var netDevs []model.NetworkDevice
	h.DB.Where("cabinet_id = ?", id).Find(&netDevs)
	if netDevs == nil {
		netDevs = []model.NetworkDevice{}
	}
	response.Success(c, gin.H{
		"cabinet": cab, "points": points, "alerts": alerts,
		"maintenances": maints, "open_work_orders": woCnt, "network_devices": netDevs,
	})
}
