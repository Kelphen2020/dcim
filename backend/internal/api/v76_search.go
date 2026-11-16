package api

// V76 B5-1: 全局搜索（G9 断点接通）
// GET /api/v1/search?q=&types=&limit=
// 六类对象统一检索：资产/测点/工单/告警/网络设备/备件
// 权限：SiteScope 站点过滤 + allowed_groups 菜单组过滤（越权类型不返回）
// 出口标准：P95 < 1s；结果携带深链 route，前端直达详情抽屉

import (
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// searchItem 统一结果条目：前端命令面板直接渲染 + 深链跳转
type searchItem struct {
	ID       uint              `json:"id"`
	Title    string            `json:"title"`
	Subtitle string            `json:"subtitle"`
	Tag      string            `json:"tag"`
	Meta     map[string]any    `json:"meta,omitempty"`
	Route    searchRoute       `json:"route"`
}

type searchRoute struct {
	Path  string            `json:"path"`
	Query map[string]string `json:"query"`
}

type searchGroup struct {
	Type  string       `json:"type"`
	Label string       `json:"label"`
	Total int64        `json:"total"`
	Items []searchItem `json:"items"`
}

// searchTypeDef 类型注册表：菜单组 key 与前端 MENU/后端 allowed_groups 白名单一致
type searchTypeDef struct {
	Type  string
	Label string
	Group string // allowed_groups 菜单组 key
}

var searchTypes = []searchTypeDef{
	{"asset", "资产", "asset"},
	{"point", "测点", "monitor"},
	{"work_order", "工单", "ops"},
	{"alert", "告警", "monitor"},
	{"network_device", "网络设备", "network"},
	{"spare_part", "备件", "ops"},
}

func searchLike(q string) string {
	// 转义 LIKE 通配符，用户输入按字面匹配
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func (h *Handler) GlobalSearch(c *gin.Context) {
	start := time.Now()
	q := strings.TrimSpace(c.Query("q"))
	if n := len([]rune(q)); n < 1 || n > 64 {
		response.BadRequest(c, "关键词长度须为 1~64 个字符")
		return
	}
	limit := 5
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 20 {
		limit = 20
	}

	// 类型过滤（可选 types=asset,point）
	want := map[string]bool{}
	if ts := c.Query("types"); ts != "" {
		for _, t := range strings.Split(ts, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			known := false
			for _, d := range searchTypes {
				if d.Type == t {
					known = true
				}
			}
			if !known {
				response.BadRequest(c, "未知搜索类型: "+t)
				return
			}
			want[t] = true
		}
	}

	// allowed_groups 菜单组权限（与侧边栏可见性同口径）
	allowedGroups := "*"
	var u model.User
	if err := h.DB.Select("allowed_groups").First(&u, middleware.GetUserID(c)).Error; err == nil {
		allowedGroups = u.AllowedGroups
	}
	groupOK := func(g string) bool {
		if allowedGroups == "" || allowedGroups == "*" {
			return true
		}
		for _, p := range strings.Split(allowedGroups, ",") {
			if strings.TrimSpace(p) == g {
				return true
			}
		}
		return false
	}

	scope := middleware.GetSiteScope(c)
	like := searchLike(q)

	// 站点名映射（subtitle 可读性）
	siteNames := map[uint]string{}
	{
		var sites []model.Site
		h.DB.Select("id, name").Find(&sites)
		for _, s := range sites {
			siteNames[s.ID] = s.Name
		}
	}
	siteName := func(id uint) string {
		if n, ok := siteNames[id]; ok {
			return n
		}
		return ""
	}

	groups := []searchGroup{}
	for _, d := range searchTypes {
		if len(want) > 0 && !want[d.Type] {
			continue
		}
		if !groupOK(d.Group) {
			continue
		}
		g := searchGroup{Type: d.Type, Label: d.Label, Items: []searchItem{}}
		switch d.Type {
		case "asset":
			qb := h.DB.Model(&model.Asset{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("code ILIKE ? OR name ILIKE ? OR sn ILIKE ? OR ip ILIKE ? OR asset_no ILIKE ? OR brand ILIKE ? OR model ILIKE ?",
				like, like, like, like, like, like, like)
			qb.Count(&g.Total)
			var items []model.Asset
			qb.Order("id DESC").Limit(limit).Find(&items)
			st := map[int]string{1: "在架", 2: "在库", 3: "维修中", 4: "已报废"}
			for _, a := range items {
				sub := a.Code
				if sn := siteName(a.SiteID); sn != "" {
					sub += " · " + sn
				}
				if a.IP != "" {
					sub += " · " + a.IP
				}
				g.Items = append(g.Items, searchItem{
					ID: a.ID, Title: a.Name, Subtitle: sub, Tag: st[a.Status],
					Meta:  map[string]any{"code": a.Code, "category": a.Category, "site_id": a.SiteID},
					Route: searchRoute{Path: "/assets", Query: map[string]string{"id": strconv.Itoa(int(a.ID)), "detail": "1"}},
				})
			}
		case "point":
			qb := h.DB.Model(&model.MonitoringPoint{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("code ILIKE ? OR name ILIKE ? OR address ILIKE ?", like, like, like)
			qb.Count(&g.Total)
			var items []model.MonitoringPoint
			qb.Order("id DESC").Limit(limit).Find(&items)
			st := map[int]string{0: "离线", 1: "正常", 2: "告警", 3: "故障"}
			for _, p := range items {
				sub := p.Code
				if sn := siteName(p.SiteID); sn != "" {
					sub += " · " + sn
				}
				g.Items = append(g.Items, searchItem{
					ID: p.ID, Title: p.Name, Subtitle: sub, Tag: st[p.Status],
					Meta:  map[string]any{"code": p.Code, "type": p.Type, "site_id": p.SiteID},
					Route: searchRoute{Path: "/monitoring-points", Query: map[string]string{"id": strconv.Itoa(int(p.ID)), "detail": "1"}},
				})
			}
		case "work_order":
			qb := h.DB.Model(&model.WorkOrder{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("code ILIKE ? OR title ILIKE ? OR description ILIKE ?", like, like, like)
			qb.Count(&g.Total)
			var items []model.WorkOrder
			qb.Order("id DESC").Limit(limit).Find(&items)
			for _, w := range items {
				sub := w.Code
				if sn := siteName(w.SiteID); sn != "" {
					sub += " · " + sn
				}
				g.Items = append(g.Items, searchItem{
					ID: w.ID, Title: w.Title, Subtitle: sub, Tag: "工单",
					Meta:  map[string]any{"code": w.Code, "status": w.Status, "priority": w.Priority, "site_id": w.SiteID},
					Route: searchRoute{Path: "/work-orders", Query: map[string]string{"id": strconv.Itoa(int(w.ID)), "detail": "1"}},
				})
			}
		case "alert":
			qb := h.DB.Model(&model.Alert{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("message ILIKE ? OR detail ILIKE ?", like, like)
			qb.Count(&g.Total)
			var items []model.Alert
			qb.Order("triggered_at DESC").Limit(limit).Find(&items)
			sev := map[int]string{1: "提示", 2: "警告", 3: "严重"}
			st := map[int]string{0: "待处理", 1: "已挂起", 2: "已确认", 3: "已结束"}
			// 测点名（小批量逐条查，limit<=20 可接受）
			for _, a := range items {
				sub := st[a.Status] + " · " + a.TriggeredAt.Format("2006-01-02 15:04")
				pn := ""
				if a.PointID != nil && *a.PointID > 0 {
					var p model.MonitoringPoint
					if err := h.DB.Select("name").First(&p, *a.PointID).Error; err == nil {
						pn = p.Name
						sub = pn + " · " + sub
					}
				}
				g.Items = append(g.Items, searchItem{
					ID: a.ID, Title: a.Message, Subtitle: sub, Tag: sev[a.Severity],
					Meta:  map[string]any{"severity": a.Severity, "status": a.Status, "point_id": a.PointID, "point_name": pn, "site_id": a.SiteID},
					Route: searchRoute{Path: "/alert-wall", Query: map[string]string{"alert_id": strconv.Itoa(int(a.ID))}},
				})
			}
		case "network_device":
			qb := h.DB.Model(&model.NetworkDevice{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("code ILIKE ? OR name ILIKE ? OR mgmt_ip ILIKE ? OR serial_no ILIKE ? OR model ILIKE ? OR vendor ILIKE ?",
				like, like, like, like, like, like)
			qb.Count(&g.Total)
			var items []model.NetworkDevice
			qb.Order("id DESC").Limit(limit).Find(&items)
			st := map[int]string{0: "离线", 1: "在线", 2: "维护", 3: "故障"}
			for _, d := range items {
				sub := d.Code
				if sn := siteName(d.SiteID); sn != "" {
					sub += " · " + sn
				}
				if d.MgmtIP != "" {
					sub += " · " + d.MgmtIP
				}
				g.Items = append(g.Items, searchItem{
					ID: d.ID, Title: d.Name, Subtitle: sub, Tag: st[d.Status],
					Meta:  map[string]any{"code": d.Code, "type": d.Type, "site_id": d.SiteID},
					Route: searchRoute{Path: "/network-devices", Query: map[string]string{"id": strconv.Itoa(int(d.ID)), "detail": "1"}},
				})
			}
		case "spare_part":
			qb := h.DB.Model(&model.SparePart{})
			if scope != nil {
				qb = qb.Where("site_id IN ?", scope)
			}
			qb = qb.Where("code ILIKE ? OR name ILIKE ? OR spec ILIKE ? OR location ILIKE ? OR supplier ILIKE ?",
				like, like, like, like, like)
			qb.Count(&g.Total)
			var items []model.SparePart
			qb.Order("id DESC").Limit(limit).Find(&items)
			for _, p := range items {
				sub := p.Code + " · 库存 " + strconv.Itoa(p.Stock) + p.Unit
				if p.Location != "" {
					sub += " · " + p.Location
				}
				tag := "正常"
				if p.Stock <= p.SafetyStock {
					tag = "低库存"
				}
				g.Items = append(g.Items, searchItem{
					ID: p.ID, Title: p.Name, Subtitle: sub, Tag: tag,
					Meta:  map[string]any{"code": p.Code, "stock": p.Stock, "safety_stock": p.SafetyStock, "site_id": p.SiteID},
					Route: searchRoute{Path: "/spare-parts", Query: map[string]string{"id": strconv.Itoa(int(p.ID)), "detail": "1"}},
				})
			}
		}
		groups = append(groups, g)
	}

	response.Success(c, gin.H{
		"q":       q,
		"took_ms": time.Since(start).Milliseconds(),
		"groups":  groups,
	})
}
