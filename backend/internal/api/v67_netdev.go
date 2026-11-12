package api

// V67/B2-2 网络设备离线判定 → 既有告警管道（G1 网络域孤岛打通第二步）
//
// 判定信号（任一成立即离线，维护中 status=2 跳过）：
//  ① 设备状态被显式标记为离线（status=0）
//  ② 心跳超时：last_seen_at 距今超过 netdev_offline_min 分钟（默认 10，t_system_setting 可调）
//  ③ 设备绑定的测点（B2-1 network_device_id）全部离线（status=0 且至少绑定 1 个）
//
// 告警走既有管道：幂等内置告警（rule_id IS NULL + detail 含 netdev_offline 标记，与 V21 过高线同款模式）；
// 若存在启用的 point_type='netdev' 告警规则，则沿用其 severity 与 auto_create_work_order（自动开工单→指派当班人→SLA，全走 createAlertWorkOrder 既有链路）；
// 恢复（重新在线/心跳新鲜/绑定测点恢复）→ 告警自动结束 + 留痕 + 通知 + Webhook。
//
// 心跳来源：POST /network-devices/:id/heartbeat（外部拨测/SNMP 轮询器上报）+ 配置备份成功自动 touch。

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ScanNetdevOffline 网络设备离线扫描（每分钟定时 + POST /network-devices/offline-scan 手动触发）
func (h *Handler) ScanNetdevOffline() map[string]interface{} {
	res := map[string]interface{}{"offline": 0, "new_alerts": 0, "recovered": 0}
	offMin := 10
	if v, err := strconv.Atoi(h.healthSetting("netdev_offline_min", "10")); err == nil && v > 0 {
		offMin = v
	}
	// netdev 类告警规则（可选）：提供 severity 与自动开工单开关
	var rule model.AlertRule
	hasRule := h.DB.Where("point_type = ? AND enabled = ?", "netdev", true).Order("id").First(&rule).Error == nil
	sev := 2
	if hasRule && rule.Severity >= 1 && rule.Severity <= 3 {
		sev = rule.Severity
	}

	var devs []model.NetworkDevice
	h.DB.Where("status <> 2").Find(&devs) // 维护中不判离线

	// 现有活跃 netdev_offline 告警 → 按设备索引（幂等与恢复的依据）
	var actives []model.Alert
	h.DB.Where("rule_id IS NULL AND status < 3 AND detail LIKE ?", "%netdev_offline%").Find(&actives)
	activeByDev := map[uint]model.Alert{}
	for _, a := range actives {
		var d struct {
			NetdevID uint `json:"netdev_id"`
		}
		if json.Unmarshal([]byte(a.Detail), &d) == nil && d.NetdevID > 0 {
			activeByDev[d.NetdevID] = a
		}
	}

	now := time.Now()
	for i := range devs {
		dev := &devs[i]
		reason := ""
		switch {
		case dev.Status == 0:
			reason = "设备状态已标记离线"
		case dev.LastSeenAt != nil && now.Sub(*dev.LastSeenAt) > time.Duration(offMin)*time.Minute:
			reason = fmt.Sprintf("心跳超时：最后在线 %s，超过 %d 分钟未更新", dev.LastSeenAt.Format("2006-01-02 15:04"), offMin)
		default:
			var total, off int64
			h.DB.Model(&model.MonitoringPoint{}).Where("network_device_id = ?", dev.ID).Count(&total)
			if total > 0 {
				h.DB.Model(&model.MonitoringPoint{}).Where("network_device_id = ? AND status = 0", dev.ID).Count(&off)
				if off == total {
					reason = fmt.Sprintf("设备绑定的 %d 个测点全部离线", total)
				}
			}
		}

		if reason != "" {
			res["offline"] = res["offline"].(int) + 1
			if _, ok := activeByDev[dev.ID]; ok {
				continue // 幂等：已有活跃离线告警
			}
			msg := fmt.Sprintf("网络设备 %s（%s）离线：%s", dev.Name, dev.Code, reason)
			detail, _ := json.Marshal(gin.H{
				"kind": "netdev_offline", "netdev_id": dev.ID,
				"netdev_code": dev.Code, "netdev_name": dev.Name, "reason": reason,
			})
			al := model.Alert{
				SiteID: dev.SiteID, Severity: sev,
				Message: msg, Detail: string(detail),
				Status: 0, TriggeredAt: now, CreatedAt: now,
			}
			if h.DB.Create(&al).Error != nil {
				continue
			}
			res["new_alerts"] = res["new_alerts"].(int) + 1
			h.DB.Create(&model.Notification{
				UserID: 0, Kind: "alarm",
				Title: "[离线] " + dev.Name, Content: msg,
				RefType: "alert", RefID: al.ID,
			})
			h.DispatchWebhook(model.EvAlarm, "[离线] "+dev.Name, msg, map[string]interface{}{
				"alert_id": al.ID, "kind": "netdev_offline", "netdev_id": dev.ID,
				"netdev_code": dev.Code, "severity": sev, "site_id": dev.SiteID,
			})
			if hasRule && rule.AutoCreateWorkOrder {
				pseudo := &model.MonitoringPoint{SiteID: dev.SiteID, Name: dev.Name + "（网络设备）"}
				h.createAlertWorkOrder(pseudo, &al, &rule)
			}
			continue
		}

		// 在线且无离线信号 → 若有活跃离线告警则自动恢复
		if al, ok := activeByDev[dev.ID]; ok {
			h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).Updates(map[string]interface{}{"status": 3, "resolved_at": now})
			h.DB.Create(&model.AlertActionLog{AlertID: al.ID, Action: "resolve", Reason: "设备恢复在线，系统自动结束离线告警", Operator: 0})
			rmsg := fmt.Sprintf("网络设备 %s（%s）已恢复在线，离线告警自动结束", dev.Name, dev.Code)
			h.DB.Create(&model.Notification{
				UserID: 0, Kind: "alarm",
				Title: "[恢复] " + dev.Name + " 离线告警解除", Content: rmsg,
				RefType: "alert", RefID: al.ID,
			})
			h.DispatchWebhook(model.EvAlarmResolve, "[恢复] "+dev.Name+" 离线告警解除", rmsg, map[string]interface{}{
				"alert_id": al.ID, "kind": "netdev_offline", "netdev_id": dev.ID,
			})
			res["recovered"] = res["recovered"].(int) + 1
		}
	}
	return res
}

// NetdevHeartbeat 网络设备心跳上报（外部拨测/轮询器调用；离线设备心跳后自动回上线）
func (h *Handler) NetdevHeartbeat(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, 400, "设备 ID 无效")
		return
	}
	var dev model.NetworkDevice
	if h.DB.First(&dev, id).Error != nil {
		response.Error(c, 404, "设备不存在")
		return
	}
	now := time.Now()
	updates := map[string]interface{}{"last_seen_at": now}
	backOnline := false
	if dev.Status == 0 {
		updates["status"] = 1
		backOnline = true
	}
	h.DB.Model(&model.NetworkDevice{}).Where("id = ?", dev.ID).Updates(updates)
	response.Success(c, gin.H{"id": dev.ID, "last_seen_at": now, "back_online": backOnline})
}

// TriggerNetdevScan 手动触发离线判定（admin，与其他 *-scan 端点同款）
func (h *Handler) TriggerNetdevScan(c *gin.Context) {
	response.Success(c, h.ScanNetdevOffline())
}
