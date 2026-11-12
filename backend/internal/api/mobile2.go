package api

// mobile2.go — M2 移动端增值接口
// POST /api/v1/duty/handover/sign  交接签字落库
// GET  /api/v1/duty/handover/signs 交接签字历史

import (
	"fmt"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// DutyHandoverSign 值班交接签字记录
type DutyHandoverSign struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SiteID       uint      `gorm:"index" json:"site_id"`
	HandoverDate time.Time `gorm:"index" json:"handover_date"` // 交接日(零点)
	Shift        string    `gorm:"size:20" json:"shift"`       // day/night/full
	FromUserID   uint      `json:"from_user_id"`
	FromUserName string    `gorm:"size:50" json:"from_user_name"`
	ToUserID     uint      `json:"to_user_id"`
	ToUserName   string    `gorm:"size:50" json:"to_user_name"`
	SignatureURL string    `gorm:"size:300" json:"signature_url"` // 手写签字图片(经 /mobile/upload)
	Note         string    `gorm:"size:500" json:"note"`
	Summary      string    `gorm:"type:text" json:"summary"` // 交接清单快照 JSON(告警/工单/巡检/风暴计数)
	CreatedAt    time.Time `json:"created_at"`
}

func (DutyHandoverSign) TableName() string { return "t_duty_handover_sign" }

type handoverSignReq struct {
	SiteID       uint   `json:"site_id"`
	Shift        string `json:"shift"`
	ToUserID     uint   `json:"to_user_id"`
	SignatureURL string `json:"signature_url"`
	Note         string `json:"note"`
}

// SignDutyHandover POST /duty/handover/sign
func (h *Handler) SignDutyHandover(c *gin.Context) {
	var req handoverSignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.SignatureURL == "" {
		response.BadRequest(c, "缺少签字图片")
		return
	}
	uid := middleware.GetUserID(c)
	var fromUser model.User
	h.DB.First(&fromUser, uid)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	shift := req.Shift
	if shift == "" {
		shift = currentShift(now)
	}

	// 交接清单快照（计数级）
	var alCount, woCount, stormCount int64
	alQ := h.DB.Model(&model.Alert{}).Where("status < 3")
	woQ := h.DB.Model(&model.WorkOrder{}).Where("status IN (1,2)")
	stQ := h.DB.Model(&model.AlertStorm{}).Where("status = 0")
	if req.SiteID > 0 {
		alQ = alQ.Where("site_id = ?", req.SiteID)
		woQ = woQ.Where("site_id = ?", req.SiteID)
		stQ = stQ.Where("site_id = ?", req.SiteID)
	}
	alQ.Count(&alCount)
	woQ.Count(&woCount)
	stQ.Count(&stormCount)

	toName := ""
	if req.ToUserID > 0 {
		var toUser model.User
		if h.DB.First(&toUser, req.ToUserID).Error == nil {
			toName = toUser.DisplayName
			if toName == "" {
				toName = toUser.Username
			}
		}
	}

	rec := DutyHandoverSign{
		SiteID: req.SiteID, HandoverDate: today, Shift: shift,
		FromUserID: uid, FromUserName: fromUser.DisplayName,
		ToUserID: req.ToUserID, ToUserName: toName,
		SignatureURL: req.SignatureURL, Note: req.Note,
		Summary: fmt.Sprintf(`{"open_alerts":%d,"open_orders":%d,"active_storms":%d,"signed_at":"%s"}`,
			alCount, woCount, stormCount, now.Format("2006-01-02 15:04:05")),
	}
	if err := h.DB.Create(&rec).Error; err != nil {
		response.ServerError(c, "保存失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "sign", "duty_handover",
		fmt.Sprintf("值班交接签字 site=%d shift=%s from=%s to=%s", req.SiteID, shift, fromUser.Username, toName))
	// 给接班人发通知
	if req.ToUserID > 0 {
		h.DB.Create(&model.Notification{
			UserID: req.ToUserID, Kind: "duty",
			Title:   "值班交接完成",
			Content: fmt.Sprintf("%s 已将 %s %s 值班交接给你：未闭环告警 %d、进行中工单 %d、活跃风暴 %d。%s",
				fromUser.DisplayName, today.Format("01-02"), shift, alCount, woCount, stormCount, req.Note),
			RefType: "duty_handover", RefID: rec.ID,
		})
	}
	response.Success(c, rec)
}

// ListDutyHandoverSigns GET /duty/handover/signs?site_id=&limit=
func (h *Handler) ListDutyHandoverSigns(c *gin.Context) {
	q := h.DB.Model(&DutyHandoverSign{})
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	limit := 30
	var items []DutyHandoverSign
	q.Order("id DESC").Limit(limit).Find(&items)
	if items == nil {
		items = []DutyHandoverSign{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}
