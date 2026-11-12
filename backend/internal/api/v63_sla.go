package api

// V63 SLA 违约分析与考核
// GET /work-orders/sla-analysis?days=7|30|90&site_id=
// 闭环定位：采集=V18 SLA 扫描器写违约标记 + accepted_at/finished_at；
//           评估=本接口按人/优先级/类型/逐日聚合考核；
//           使用=工单页「SLA 考核」页签（KPI 穿透/考核表/违约清单/CSV 导出）；
//           报警=复用 V18 sla_warn/sla_breach 通知与 Webhook（不重复建通道）。

import (
	"sort"
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type slaUserAgg struct {
	UserID      uint    `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Assigned    int     `json:"assigned"`
	Finished    int     `json:"finished"`
	Open        int     `json:"open"`
	RespBreach  int     `json:"resp_breach"`
	ResBreach   int     `json:"resolve_breach"`
	Breach      int     `json:"breach"`
	BreachRate  float64 `json:"breach_rate"`
	AvgResolveH float64 `json:"avg_resolve_h"`
	resolveSum  float64
}

func (h *Handler) SLAAnalysis(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days != 7 && days != 90 {
		days = 30
	}
	siteID := c.Query("site_id")
	now := time.Now()
	since := now.AddDate(0, 0, -days)

	q := h.DB.Model(&model.WorkOrder{}).Where("created_at >= ?", since)
	if siteID != "" && siteID != "0" {
		q = q.Where("site_id = ?", siteID)
	}
	var rows []model.WorkOrder
	q.Find(&rows)

	// 用户名/站点名映射
	var users []model.User
	h.DB.Find(&users)
	uname := map[uint]string{}
	udisp := map[uint]string{}
	for _, u := range users {
		uname[u.ID] = u.Username
		udisp[u.ID] = u.DisplayName
	}
	var sites []model.Site
	h.DB.Find(&sites)
	sname := map[uint]string{}
	for _, s := range sites {
		sname[s.ID] = s.Name
	}

	kpi := gin.H{}
	respBreach, resBreach, breachTotal, openOverdue, finished := 0, 0, 0, 0, 0
	var respSum, respN, resSum, resN float64

	type priAgg struct {
		total, rb, sb, fin int
		resSum             float64
	}
	byPri := map[int]*priAgg{}
	byType := map[string]*priAgg{}
	userAgg := map[uint]*slaUserAgg{}

	// 逐日趋势（补零）
	trend := make([]gin.H, days)
	dayIdx := map[string]int{}
	for i := 0; i < days; i++ {
		d := since.AddDate(0, 0, i+1).Format("2006-01-02")
		dayIdx[d] = i
		trend[i] = gin.H{"date": d, "created": 0, "finished": 0, "breached": 0}
	}

	getPri := func(p int) *priAgg {
		if byPri[p] == nil {
			byPri[p] = &priAgg{}
		}
		return byPri[p]
	}
	getType := func(t string) *priAgg {
		if byType[t] == nil {
			byType[t] = &priAgg{}
		}
		return byType[t]
	}

	for i := range rows {
		w := &rows[i]
		breached := w.SlaRespondBreach || w.SlaResolveBreach
		if w.SlaRespondBreach {
			respBreach++
		}
		if w.SlaResolveBreach {
			resBreach++
		}
		if breached {
			breachTotal++
		}
		if w.Status >= 3 && w.Status != 5 {
			finished++
			if w.FinishedAt != nil {
				hh := w.FinishedAt.Sub(w.CreatedAt).Hours()
				if hh >= 0 {
					resSum += hh
					resN++
				}
			}
		} else if w.Status < 3 {
			if w.SlaResolveDue != nil && now.After(*w.SlaResolveDue) {
				openOverdue++
			}
		}
		if w.AcceptedAt != nil {
			hh := w.AcceptedAt.Sub(w.CreatedAt).Hours()
			if hh >= 0 {
				respSum += hh
				respN++
			}
		}
		p := getPri(w.Priority)
		p.total++
		if w.SlaRespondBreach {
			p.rb++
		}
		if w.SlaResolveBreach {
			p.sb++
		}
		if w.Status >= 3 && w.Status != 5 {
			p.fin++
			if w.FinishedAt != nil {
				p.resSum += w.FinishedAt.Sub(w.CreatedAt).Hours()
			}
		}
		t := getType(w.Type)
		t.total++
		if breached {
			t.rb++ // 类型维度复用 rb 存"任一违约"
		}
		if w.Status >= 3 && w.Status != 5 {
			t.fin++
		}
		// 按处理人考核（含未指派桶 user_id=0）
		ua := userAgg[w.AssigneeID]
		if ua == nil {
			ua = &slaUserAgg{UserID: w.AssigneeID}
			if w.AssigneeID == 0 {
				ua.Username = "-"
				ua.DisplayName = "未指派"
			} else {
				ua.Username = uname[w.AssigneeID]
				ua.DisplayName = udisp[w.AssigneeID]
				if ua.DisplayName == "" {
					ua.DisplayName = ua.Username
				}
			}
			userAgg[w.AssigneeID] = ua
		}
		ua.Assigned++
		if w.Status >= 3 && w.Status != 5 {
			ua.Finished++
			if w.FinishedAt != nil {
				ua.resolveSum += w.FinishedAt.Sub(w.CreatedAt).Hours()
			}
		} else if w.Status < 3 {
			ua.Open++
		}
		if w.SlaRespondBreach {
			ua.RespBreach++
		}
		if w.SlaResolveBreach {
			ua.ResBreach++
		}
		if breached {
			ua.Breach++
		}
		// 趋势桶（按创建日）
		dk := w.CreatedAt.Format("2006-01-02")
		if idx, ok := dayIdx[dk]; ok {
			trend[idx]["created"] = trend[idx]["created"].(int) + 1
			if breached {
				trend[idx]["breached"] = trend[idx]["breached"].(int) + 1
			}
			if w.FinishedAt != nil && w.Status >= 3 && w.Status != 5 {
				fk := w.FinishedAt.Format("2006-01-02")
				if fidx, ok2 := dayIdx[fk]; ok2 {
					trend[fidx]["finished"] = trend[fidx]["finished"].(int) + 1
				}
			}
		}
	}

	pct := func(ok, all int) float64 {
		if all == 0 {
			return 0
		}
		return mathRound(float64(ok)*100/float64(all), 1)
	}
	avg := func(s, n float64) float64 {
		if n == 0 {
			return 0
		}
		return mathRound(s/n, 1)
	}

	kpi["total"] = len(rows)
	kpi["finished"] = finished
	kpi["respond_breach"] = respBreach
	kpi["resolve_breach"] = resBreach
	kpi["breach_total"] = breachTotal
	kpi["open_overdue"] = openOverdue
	kpi["respond_rate"] = pct(len(rows)-respBreach, len(rows))
	kpi["resolve_rate"] = pct(len(rows)-resBreach, len(rows))
	kpi["avg_respond_h"] = avg(respSum, respN)
	kpi["avg_resolve_h"] = avg(resSum, resN)

	priOut := make([]gin.H, 0, 4)
	for p := 1; p <= 4; p++ {
		if a, ok := byPri[p]; ok {
			priOut = append(priOut, gin.H{
				"priority": p, "label": priName(p), "total": a.total,
				"resp_breach": a.rb, "resolve_breach": a.sb,
				"respond_rate": pct(a.total-a.rb, a.total),
				"finished":     a.fin,
				"avg_resolve_h": avg(a.resSum, float64(a.fin)),
			})
		}
	}
	typeOut := make([]gin.H, 0, len(byType))
	for t, a := range byType {
		typeOut = append(typeOut, gin.H{
			"type": t, "total": a.total, "breach": a.rb,
			"breach_rate": pct(a.rb, a.total), "finished": a.fin,
		})
	}
	sort.Slice(typeOut, func(i, j int) bool {
		return typeOut[i]["total"].(int) > typeOut[j]["total"].(int)
	})

	userOut := make([]*slaUserAgg, 0, len(userAgg))
	for _, ua := range userAgg {
		ua.BreachRate = pct(ua.Breach, ua.Assigned)
		ua.AvgResolveH = avg(ua.resolveSum, float64(ua.Finished))
		userOut = append(userOut, ua)
	}
	sort.Slice(userOut, func(i, j int) bool {
		if userOut[i].Breach != userOut[j].Breach {
			return userOut[i].Breach > userOut[j].Breach
		}
		return userOut[i].Assigned > userOut[j].Assigned
	})
	if len(userOut) > 50 {
		userOut = userOut[:50]
	}

	// 违约工单清单（最近 20 条，含完整对象供前端直接开详情抽屉）
	breachList := make([]model.WorkOrder, 0)
	for i := range rows {
		w := rows[i]
		if w.SlaRespondBreach || w.SlaResolveBreach {
			breachList = append(breachList, w)
		}
	}
	sort.Slice(breachList, func(i, j int) bool { return breachList[i].CreatedAt.After(breachList[j].CreatedAt) })
	if len(breachList) > 20 {
		breachList = breachList[:20]
	}
	topBreaches := make([]gin.H, 0, len(breachList))
	for _, w := range breachList {
		an := "-"
		if w.AssigneeID != 0 {
			if d := udisp[w.AssigneeID]; d != "" {
				an = d
			} else {
				an = uname[w.AssigneeID]
			}
		}
		kinds := make([]string, 0, 2)
		if w.SlaRespondBreach {
			kinds = append(kinds, "响应违约")
		}
		if w.SlaResolveBreach {
			kinds = append(kinds, "解决违约")
		}
		topBreaches = append(topBreaches, gin.H{
			"id": w.ID, "code": w.Code, "title": w.Title, "priority": w.Priority,
			"status": w.Status, "site_name": sname[w.SiteID], "assignee_name": an,
			"breach_kinds": kinds, "created_at": w.CreatedAt, "wo": w,
		})
	}

	response.Success(c, gin.H{
		"days":          days,
		"kpi":           kpi,
		"by_priority":   priOut,
		"by_type":       typeOut,
		"by_user":       userOut,
		"trend":         trend,
		"top_breaches":  topBreaches,
		"generated_at":  now,
	})
}
