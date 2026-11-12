package api

import (
	"fmt"
	"log"
	"time"

	"dcim/backend/internal/model"
)

// ==================== V22-② 告警风暴抑制 ====================
// 原理: 上游供电节点(市电/变压器/UPS)故障 → 下游机柜批量断电/温湿度连锁告警。
// 新告警产生后调用 DetectAlertStorm:
//   1. 找到告警测点关联的供电上下文(cabinet → 所属供电节点子树)
//   2. 查询近 stormWindow(默认10分钟) 内同子树的活跃告警数
//   3. ≥ stormThreshold(默认5) 条 → 聚合为 AlertStorm:
//      - 根因=子树最上游节点(utility>transformer>ups>pdu)
//      - 首条告警为根因告警(不抑制)，其余标记 suppressed+storm_id
//      - 后续同子树新告警直接归入风暴(仍创建但不发通知/不自动开工单)
//   4. 风暴结束: 所有成员告警结束(status>=3)或窗口静默(1小时无新告警)

const stormWindowMin = 10
const stormThreshold = 5

// pointPowerRoot 测点→所属供电上下文: 返回(子树节点ID列表, 根因节点)
// 路径: 测点 cabinet_id → 找绑定该机柜的供电节点 → 沿 parent 向上到根
func (h *Handler) pointPowerContext(point *model.MonitoringPoint) (subtreeIDs []uint, root *model.PowerChain) {
	if point == nil || point.CabinetID == nil || *point.CabinetID == 0 {
		return nil, nil
	}
	cabID := *point.CabinetID
	// 找绑定该机柜的供电节点(通常是末端PDU/支路)
	var node model.PowerChain
	if err := h.DB.Where("cabinet_id = ? AND site_id = ?", cabID, point.SiteID).First(&node).Error; err != nil {
		return nil, nil
	}
	// 沿 parent 向上找根
	rootNode := node
	for rootNode.ParentID != 0 {
		var p model.PowerChain
		if err := h.DB.First(&p, rootNode.ParentID).Error; err != nil {
			break
		}
		rootNode = p
	}
	// 收集根的整个子树节点ID
	var all []model.PowerChain
	h.DB.Where("site_id = ?", point.SiteID).Find(&all)
	ids := []uint{}
	stack := []uint{rootNode.ID}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		ids = append(ids, cur)
		for _, n := range all {
			if n.ParentID == cur {
				stack = append(stack, n.ID)
			}
		}
	}
	return ids, &rootNode
}

// cabinetIDsOfSubtree 子树覆盖的机柜ID集合
func (h *Handler) cabinetIDsOfSubtree(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	var nodes []model.PowerChain
	h.DB.Select("id, cabinet_id").Where("id IN ?", ids).Find(&nodes)
	set := map[uint]bool{}
	for _, n := range nodes {
		if n.CabinetID > 0 {
			set[n.CabinetID] = true
		}
	}
	// 资产所在机柜也算
	var assetCabs []model.Asset
	h.DB.Select("DISTINCT cabinet_id").Where("cabinet_id IN (SELECT id FROM t_cabinet WHERE id IN (?))", ids).Find(&assetCabs)
	out := make([]uint, 0, len(set))
	for cid := range set {
		out = append(out, cid)
	}
	return out
}

// DetectAlertStorm 告警创建后调用: 判定/归入风暴。返回 stormID(0=非风暴), suppressed bool
func (h *Handler) DetectAlertStorm(alert *model.Alert, point *model.MonitoringPoint) (uint, bool) {
	if point == nil {
		return 0, false
	}
	now := time.Now()
	// 1. 已归入活跃风暴? (同测点上下文)
	_, root := h.pointPowerContext(point)
	if root == nil {
		return 0, false
	}
	// 找该根节点的活跃风暴
	var active model.AlertStorm
	err := h.DB.Where("root_kind = 'power_node' AND root_id = ? AND status = 0", root.ID).First(&active).Error
	if err == nil {
		// 归入既有风暴
		h.DB.Model(&model.Alert{}).Where("id = ?", alert.ID).
			Updates(map[string]interface{}{"storm_id": active.ID, "suppressed": true})
		h.DB.Model(&active).Updates(map[string]interface{}{
			"alert_count":    active.AlertCount + 1,
			"suppressed_cnt": active.SuppressedCnt + 1,
			"last_alert_at":  now,
		})
		return active.ID, true
	}
	// 2. 窗口内同子树活跃告警数
	subIDs, _ := h.pointPowerContext(point)
	cabIDs := h.cabinetIDsOfSubtree(subIDs)
	if len(cabIDs) == 0 {
		return 0, false
	}
	var cnt int64
	h.DB.Model(&model.Alert{}).
		Where("status < 3 AND triggered_at >= ? AND site_id = ? AND point_id IN (?)",
			now.Add(-stormWindowMin*time.Minute), point.SiteID,
			h.DB.Model(&model.MonitoringPoint{}).Select("id").Where("cabinet_id IN ?", cabIDs),
		).Count(&cnt)
	if cnt < stormThreshold {
		return 0, false
	}
	// 3. 达到阈值 → 创建风暴
	var recent []model.Alert
	h.DB.Where("status < 3 AND triggered_at >= ? AND site_id = ? AND point_id IN (?)",
		now.Add(-stormWindowMin*time.Minute), point.SiteID,
		h.DB.Model(&model.MonitoringPoint{}).Select("id").Where("cabinet_id IN ?", cabIDs),
	).Order("id").Find(&recent)
	storm := model.AlertStorm{
		SiteID: point.SiteID, RootKind: "power_node", RootID: root.ID, RootName: root.Name,
		AlertCount: len(recent), SuppressedCnt: len(recent) - 1,
		Status: 0, FirstAlertAt: now, LastAlertAt: now,
	}
	if len(recent) > 0 {
		storm.FirstAlertAt = recent[0].TriggeredAt
	}
	if err := h.DB.Create(&storm).Error; err != nil {
		log.Printf("创建告警风暴失败: %v", err)
		return 0, false
	}
	// 首条=根因告警，其余抑制
	for i, al := range recent {
		updates := map[string]interface{}{"storm_id": storm.ID}
		if i > 0 {
			updates["suppressed"] = true
		}
		h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).Updates(updates)
	}
	// 通知 + 外发（风暴级）
	msg := fmt.Sprintf("供电节点 %s 子树 %d 分钟内产生 %d 条告警，疑似上游故障（根因告警 #%d），其余 %d 条已抑制",
		root.Name, stormWindowMin, len(recent), recent[0].ID, len(recent)-1)
	h.DB.Create(&model.Notification{
		UserID: 0, Kind: "alarm_storm",
		Title: "[告警风暴] " + root.Name, Content: msg,
	})
	h.DispatchWebhook(model.EvAlarm, "[告警风暴] "+root.Name, msg, map[string]interface{}{
		"storm_id": storm.ID, "root_name": root.Name, "alert_count": len(recent),
	})
	log.Printf("告警风暴聚合: root=%s alerts=%d storm_id=%d", root.Name, len(recent), storm.ID)
	// 当前告警若非首条则被抑制
	suppressed := len(recent) > 1 && recent[len(recent)-1].ID == alert.ID
	if suppressed {
		h.DB.Model(&model.Alert{}).Where("id = ?", alert.ID).Update("suppressed", true)
	}
	return storm.ID, suppressed
}

// ScanStormEnd 每分钟: 风暴成员全部结束或 1h 无新告警 → 关闭风暴
func (h *Handler) ScanStormEnd() int {
	var storms []model.AlertStorm
	h.DB.Where("status = 0").Find(&storms)
	now := time.Now()
	closed := 0
	for _, st := range storms {
		var active int64
		h.DB.Model(&model.Alert{}).Where("storm_id = ? AND status < 3", st.ID).Count(&active)
		if active == 0 || now.Sub(st.LastAlertAt) > time.Hour {
			h.DB.Model(&model.AlertStorm{}).Where("id = ?", st.ID).
				Updates(map[string]interface{}{"status": 1, "ended_at": now})
			h.DB.Create(&model.Notification{
				UserID: 0, Kind: "alarm_storm",
				Title:   "[风暴结束] " + st.RootName,
				Content: fmt.Sprintf("告警风暴已结束: 共 %d 条告警(抑制 %d 条)", st.AlertCount, st.SuppressedCnt),
			})
			closed++
		}
	}
	return closed
}
