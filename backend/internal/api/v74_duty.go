package api

// V74 B4-1（G5）值班表与工单指派联动
// 工单创建：未指定处理人 → 自动取站点当班人（精确站点+班次 > 全站+班次 > 精确+full > 全站+full）
// 改派：未指定目标 → 默认当班人；前端打开改派抽屉时预填当班人并提示班次
// 无当班人：保持未指派，由前端提示「当前无当班人，请手动选择」，不阻断创建

import (
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type v74DutyBrief struct {
	OnDuty     bool   `json:"on_duty"`
	UserID     uint   `json:"user_id"`
	UserName   string `json:"user_name"`
	Shift      string `json:"shift"`
	ShiftLabel string `json:"shift_label"`
	DutyDate   string `json:"duty_date"`
	Scope      string `json:"scope"` // site=本站排班 all=全站排班 none=无
	Fallback   string `json:"fallback"`
}

func v74ShiftLabel(shift string) string {
	switch shift {
	case "day":
		return "白班 08:00-20:00"
	case "night":
		return "夜班 20:00-08:00"
	case "full":
		return "全天"
	}
	return shift
}

func (h *Handler) v74DutyBrief(siteID uint) v74DutyBrief {
	nowShift := currentShift(time.Now())
	b := v74DutyBrief{Shift: nowShift, ShiftLabel: v74ShiftLabel(nowShift), Scope: "none", Fallback: "none"}
	ds := h.OnDutyUser(siteID)
	if ds == nil {
		return b
	}
	var u model.User
	if err := h.DB.First(&u, ds.UserID).Error; err != nil || u.Status == 0 {
		return b
	}
	b.OnDuty = true
	b.UserID = ds.UserID
	b.UserName = u.DisplayName
	if b.UserName == "" {
		b.UserName = u.Username
	}
	b.Shift = ds.Shift
	b.ShiftLabel = v74ShiftLabel(ds.Shift)
	b.DutyDate = ds.DutyDate.Format("2006-01-02")
	if ds.SiteID == siteID {
		b.Scope = "site"
	} else {
		b.Scope = "all"
	}
	return b
}

// WorkOrderAssigneeSuggestion GET /work-orders/assignee-suggestion?site_id=
func (h *Handler) WorkOrderAssigneeSuggestion(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.DefaultQuery("site_id", "0"))
	b := h.v74DutyBrief(uint(siteID))
	response.Success(c, gin.H{
		"on_duty": b.OnDuty, "user_id": b.UserID, "user_name": b.UserName,
		"shift": b.Shift, "shift_label": b.ShiftLabel, "duty_date": b.DutyDate,
		"scope": b.Scope, "fallback": b.Fallback,
	})
}