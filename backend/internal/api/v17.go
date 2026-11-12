package api

import (
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V17 运维联动闭环 ====================

// MaintenanceCoverage V17 维保覆盖分析
// 口径: 资产「在保」= 存在 kind=contract 且 status=1 且 end_at>now 的维保记录
//
//	「即将到期」= 在保但 end_at 在 N 天内(默认30)
//	「脱保」= 在架/在库/维修资产(status<>4)且无在保记录
func (h *Handler) MaintenanceCoverage(c *gin.Context) {
	days := 30
	if d, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && d > 0 && d <= 365 {
		days = d
	}
	now := time.Now()
	soon := now.AddDate(0, 0, days)

	// 资产范围: 非报废; 可按站点/分类过滤
	aq := h.DB.Model(&model.Asset{}).Where("status <> 4")
	if scope := middleware.GetSiteScope(c); scope != nil {
		aq = aq.Where("site_id IN ?", scope)
	}
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		aq = aq.Where("site_id = ?", sid)
	}
	if cat := c.Query("category"); cat != "" {
		aq = aq.Where("category = ?", cat)
	}
	if cab := c.Query("cabinet_id"); cab != "" && cab != "0" {
		aq = aq.Where("cabinet_id = ?", cab)
	}
	var assets []model.Asset
	aq.Find(&assets)
	if assets == nil {
		assets = []model.Asset{}
	}

	// 在保维保(合同类, 未到期): asset_id → 最晚 end_at
	covered := map[uint]time.Time{}
	{
		var ms []model.Maintenance
		h.DB.Where("kind = 'contract' AND end_at IS NOT NULL AND end_at > ?", now).Find(&ms)
		for _, m := range ms {
			if e := *m.EndAt; covered[m.AssetID].IsZero() || e.After(covered[m.AssetID]) {
				covered[m.AssetID] = e
			}
		}
	}

	siteName := map[uint]string{}
	{
		var ss []model.Site
		h.DB.Select("id", "name").Find(&ss)
		for _, s := range ss {
			siteName[s.ID] = s.Name
		}
	}
	cabCode := map[uint]string{}
	{
		var cs []model.Cabinet
		h.DB.Select("id", "code").Find(&cs)
		for _, cb := range cs {
			cabCode[cb.ID] = cb.Code
		}
	}

	type covRow struct {
		AssetID    uint   `json:"asset_id"`
		Code       string `json:"code"`
		Name       string `json:"name"`
		Category   string `json:"category"`
		SiteID     uint   `json:"site_id"`
		SiteName   string `json:"site_name"`
		CabinetID  uint   `json:"cabinet_id"`
		CabinetCode string `json:"cabinet_code"`
		Status     int    `json:"status"`
		PowerW     int    `json:"power_w"`
		Coverage   string `json:"coverage"` // covered/expiring/uncovered
		EndAt      string `json:"end_at"`
		DaysLeft   int    `json:"days_left"` // -1=脱保
	}
	rows := make([]covRow, 0, len(assets))
	var nCovered, nExpiring, nUncovered int
	catStat := map[string]map[string]int{}
	for _, a := range assets {
		r := covRow{
			AssetID: a.ID, Code: a.Code, Name: a.Name, Category: a.Category,
			SiteID: a.SiteID, SiteName: siteName[a.SiteID],
			CabinetID: a.CabinetID, CabinetCode: cabCode[a.CabinetID],
			Status: a.Status, PowerW: a.PowerW,
		}
		if cs := catStat[a.Category]; cs == nil {
			cs = map[string]int{}
			catStat[a.Category] = cs
		}
		if end, ok := covered[a.ID]; ok {
			r.EndAt = end.Format("2006-01-02")
			r.DaysLeft = int(end.Sub(now).Hours() / 24)
			if end.Before(soon) {
				r.Coverage = "expiring"
				nExpiring++
				catStat[a.Category]["expiring"]++
			} else {
				r.Coverage = "covered"
				nCovered++
				catStat[a.Category]["covered"]++
			}
		} else {
			r.Coverage = "uncovered"
			r.DaysLeft = -1
			nUncovered++
			catStat[a.Category]["uncovered"]++
		}
		rows = append(rows, r)
	}
	total := len(assets)
	covPct := 0.0
	if total > 0 {
		covPct = float64(nCovered+nExpiring) / float64(total) * 100
	}
	response.Success(c, gin.H{
		"days_window":    days,
		"total":          total,
		"covered":        nCovered,
		"expiring":       nExpiring,
		"uncovered":      nUncovered,
		"coverage_pct":   covPct,
		"category_stats": catStat,
		"items":          rows,
	})
}

// ChangeImpact V17 变更影响预分析
// 变更关联机柜(cabinet_ids)/供电节点(power_node_ids) → 复用停电影响逻辑评估波及面
func (h *Handler) ChangeImpact(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ch model.ChangeRequest
	if err := h.DB.First(&ch, id).Error; err != nil {
		response.NotFound(c, "变更不存在")
		return
	}
	cabIDs := parseUintCSV(ch.CabinetIDs)
	pcIDs := parseUintCSV(ch.PowerNodeIDs)

	// 供电节点 → 子树波及机柜(同 PowerChainImpact 逻辑)
	cabSet := map[uint]bool{}
	for _, cid := range cabIDs {
		cabSet[cid] = true
	}
	var pcNodes []model.PowerChain
	if len(pcIDs) > 0 {
		var all []model.PowerChain
		h.DB.Where("site_id = ?", ch.SiteID).Find(&all)
		// 收集每个选中节点的子树
		for _, rootID := range pcIDs {
			stack := []uint{rootID}
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, n := range all {
					if n.ParentID == cur {
						if n.CabinetID > 0 {
							cabSet[n.CabinetID] = true
						}
						stack = append(stack, n.ID)
					}
				}
				var self model.PowerChain
				if h.DB.Select("cabinet_id", "asset_id").First(&self, cur).Error == nil {
					if self.CabinetID > 0 {
						cabSet[self.CabinetID] = true
					}
					if self.AssetID > 0 {
						var a model.Asset
						if h.DB.Select("cabinet_id").First(&a, self.AssetID).Error == nil && a.CabinetID > 0 {
							cabSet[a.CabinetID] = true
						}
					}
				}
			}
		}
		// 找节点信息
		h.DB.Where("id IN ?", pcIDs).Find(&pcNodes)
	}

	cabIDsAll := make([]uint, 0, len(cabSet))
	for cid := range cabSet {
		cabIDsAll = append(cabIDsAll, cid)
	}

	type cabRow struct {
		ID       uint    `json:"id"`
		Code     string  `json:"code"`
		RoomID   uint    `json:"room_id"`
		AssetCnt int64   `json:"asset_cnt"`
		PowerKW  float64 `json:"power_kw"`
	}
	cabs := []cabRow{}
	totalW := 0
	assets := []gin.H{}
	if len(cabIDsAll) > 0 {
		h.DB.Model(&model.Cabinet{}).Select("id, code, room_id").Where("id IN ?", cabIDsAll).Order("code").Scan(&cabs)
		if cabs == nil {
			cabs = []cabRow{}
		}
		for i := range cabs {
			var w int64
			h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", cabs[i].ID).
				Select("COALESCE(SUM(power_w),0)").Scan(&w)
			h.DB.Model(&model.Asset{}).Where("cabinet_id = ? AND status = 1", cabs[i].ID).Count(&cabs[i].AssetCnt)
			cabs[i].PowerKW = float64(w) / 1000.0
			totalW += int(w)
		}
		var rows []struct {
			ID     uint   `json:"id"`
			Code   string `json:"code"`
			Name   string `json:"name"`
			StartU int    `json:"start_u"`
			PowerW int    `json:"power_w"`
			IP     string `json:"ip"`
		}
		h.DB.Model(&model.Asset{}).Select("id, code, name, start_u, power_w, ip").
			Where("cabinet_id IN ? AND status = 1", cabIDsAll).Order("cabinet_id, start_u").Scan(&rows)
		for _, r := range rows {
			assets = append(assets, gin.H{"id": r.ID, "code": r.Code, "name": r.Name, "start_u": r.StartU, "power_w": r.PowerW, "ip": r.IP})
		}
	}
	if assets == nil {
		assets = []gin.H{}
	}

	// 风险评级建议: 波及设备数+功率 与 申报风险对照
	suggestRisk := 1
	if len(assets) >= 20 || totalW >= 10000 {
		suggestRisk = 3
	} else if len(assets) >= 5 || totalW >= 3000 {
		suggestRisk = 2
	}

	response.Success(c, gin.H{
		"change":         ch,
		"power_nodes":    pcNodes,
		"cabinets":       cabs,
		"cabinet_count":  len(cabs),
		"assets":         assets,
		"asset_count":    len(assets),
		"total_power_w":  totalW,
		"suggested_risk": suggestRisk,
	})
}

func parseUintCSV(s string) []uint {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	out := []uint{}
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

// InspectionItemsLive V17 巡检检查项实时值
// 检查项文本中含 @测点编码 (如 "检查UPS负载 @demo_ups_a1") → 返回对应测点实时值与阈值状态
func (h *Handler) InspectionItemsLive(c *gin.Context) {
	var req struct {
		Items []string `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.Error(c, 400, "参数错误: 须传 items 数组")
		return
	}
	if len(req.Items) > 200 {
		response.Error(c, 400, "单次最多200项")
		return
	}
	type itemLive struct {
		Item         string   `json:"item"`
		PointCode    string   `json:"point_code"`
		PointID      uint     `json:"point_id"`
		PointName    string   `json:"point_name"`
		CurrentValue  *float64 `json:"current_value"`
		Unit          string   `json:"unit"`
		Status        int      `json:"status"` // 测点状态 2=告警
		MinThreshold  *float64 `json:"min_threshold"`
		MaxThreshold  *float64 `json:"max_threshold"`
		HighThreshold *float64 `json:"high_threshold"` // V21-④ 过高线
		InRange       *bool    `json:"in_range"`       // nil=无阈值
		RangeStatus   string   `json:"range_status"`   // V21-④ normal/high/low/over(过高)
	}
	out := make([]itemLive, 0, len(req.Items))
	for _, it := range req.Items {
		code := extractPointCode(it)
		row := itemLive{Item: it}
		if code != "" {
			var p model.MonitoringPoint
			if h.DB.Where("code = ?", code).First(&p).Error == nil {
				row.PointCode = p.Code
				row.PointID = p.ID
				row.PointName = p.Name
				row.CurrentValue = p.CurrentValue
				row.Unit = p.Unit
				row.Status = p.Status
				row.MinThreshold = p.MinThreshold
				row.MaxThreshold = p.MaxThreshold
				row.HighThreshold = p.HighThreshold
				if p.CurrentValue != nil && (p.MinThreshold != nil || p.MaxThreshold != nil || p.HighThreshold != nil) {
					ok := true
					v := *p.CurrentValue
					rs := "normal"
					if p.MinThreshold != nil && v < *p.MinThreshold {
						ok = false
						rs = "low"
					}
					if p.MaxThreshold != nil && v > *p.MaxThreshold {
						ok = false
						rs = "high"
					}
					if p.HighThreshold != nil && v > *p.HighThreshold {
						ok = false
						rs = "over"
					}
					row.InRange = &ok
					row.RangeStatus = rs
				}
			} else {
				row.PointCode = code // 编码无效也回传, 前端提示
			}
		}
		out = append(out, row)
	}
	response.Success(c, gin.H{"items": out})
}

// extractPointCode 从检查项文本提取 @code 标记
func extractPointCode(s string) string {
	i := strings.Index(s, "@")
	if i < 0 {
		return ""
	}
	rest := []rune(s[i+1:])
	j := 0
	for j < len(rest) {
		ch := rest[j]
		if ch == ' ' || ch == ',' || ch == '\uFF0C' || ch == ')' || ch == '\uFF09' || ch == '	' || ch == '\n' {
			break
		}
		j++
	}
	return string(rest[:j])
}
