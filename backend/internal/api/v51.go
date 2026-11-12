package api

import (
	"fmt"
	"math"
	"strconv"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== V51 上架 What-if 四维容量仿真 ====================
// 维度：U位 / 功率 / 承重 / 制冷（房间级估算）
// 状态：ok / warn / over / na（未设限值）

// WhatIfDim 单维校验结果
type WhatIfDim struct {
	Dim     string  `json:"dim"`
	Label   string  `json:"label"`
	Current float64 `json:"current"` // 当前用量
	Add     float64 `json:"add"`     // 本次新增
	After   float64 `json:"after"`   // 上架后用量
	Limit   float64 `json:"limit"`   // 限值（0=未设）
	Unit    string  `json:"unit"`
	Status  string  `json:"status"` // ok/warn/over/na
	Message string  `json:"message"`
}

// WhatIfResult 四维校验汇总
type WhatIfResult struct {
	CabinetID   uint        `json:"cabinet_id"`
	CabinetCode string      `json:"cabinet_code"`
	RoomName    string      `json:"room_name"`
	Checks      []WhatIfDim `json:"checks"`
	Over        []string    `json:"over"`
	Warn        []string    `json:"warn"`
	Verdict     string      `json:"verdict"` // ok/warn/over
	Conclusion  string      `json:"conclusion"`
}

// coolingCapacityW 房间制冷能力估算（W）：每60㎡一台精密空调（既有口径），单台按25kW，2~8台
func coolingCapacityW(area float64) (capW float64, units int) {
	if area <= 0 {
		return 0, 0
	}
	units = int(math.Ceil(area / 60))
	if units < 2 {
		units = 2
	}
	if units > 8 {
		units = 8
	}
	return float64(units) * 25000, units
}

// whatIfCheck 四维容量校验核心（MountAsset 与 CabinetWhatIf 共用）
func whatIfCheck(db *gorm.DB, cab model.Cabinet, heightU int, powerW int, weightKG float64) WhatIfResult {
	if heightU < 1 {
		heightU = 1
	}
	res := WhatIfResult{
		CabinetID:   cab.ID,
		CabinetCode: cab.Code,
		Checks:      []WhatIfDim{},
		Over:        []string{},
		Warn:        []string{},
	}

	// —— 维度1：U位 ——
	var occupants []model.Asset
	db.Where("cabinet_id = ? AND status = 1", cab.ID).Find(&occupants)
	uSet := map[int]bool{}
	var usedW int64
	var usedKG float64
	for _, o := range occupants {
		for u := o.StartU; u < o.StartU+o.HeightU && u <= cab.TotalU; u++ {
			uSet[u] = true
		}
		usedW += int64(o.PowerW)
		usedKG += o.WeightKG
	}
	usedU := len(uSet)
	uDim := WhatIfDim{
		Dim: "u", Label: "U位", Current: float64(usedU), Add: float64(heightU),
		After: float64(usedU + heightU), Limit: float64(cab.TotalU), Unit: "U",
	}
	if uDim.After > uDim.Limit {
		uDim.Status = "over"
		uDim.Message = fmt.Sprintf("U位不足：机柜共 %dU，已用 %dU，上架后需 %dU，超出 %.0fU",
			cab.TotalU, usedU, usedU+heightU, uDim.After-uDim.Limit)
	} else {
		pct := 0.0
		if cab.TotalU > 0 {
			pct = uDim.After / float64(cab.TotalU)
		}
		if pct >= 0.9 {
			uDim.Status = "warn"
			uDim.Message = fmt.Sprintf("U位偏紧：上架后占用 %.0f%%（%d/%dU）", pct*100, usedU+heightU, cab.TotalU)
		} else {
			uDim.Status = "ok"
			uDim.Message = fmt.Sprintf("U位充足：上架后 %d/%dU，余 %dU", usedU+heightU, cab.TotalU, cab.TotalU-usedU-heightU)
		}
	}
	res.Checks = append(res.Checks, uDim)

	// —— 维度2：功率 ——
	pDim := WhatIfDim{
		Dim: "power", Label: "功率", Current: float64(usedW), Add: float64(powerW),
		After: float64(usedW + int64(powerW)), Unit: "W",
	}
	if cab.RatedPowerKW > 0 {
		pDim.Limit = cab.RatedPowerKW * 1000
		ratio := pDim.After / pDim.Limit
		switch {
		case ratio > 1:
			pDim.Status = "over"
			pDim.Message = fmt.Sprintf("功率超限：机柜额定 %.1fkW，上架后 %.0fW（%.0f%%）",
				cab.RatedPowerKW, pDim.After, ratio*100)
		case ratio >= 0.9:
			pDim.Status = "warn"
			pDim.Message = fmt.Sprintf("功率偏紧：上架后达额定 %.0f%%（%.0f/%.0fW）", ratio*100, pDim.After, pDim.Limit)
		default:
			pDim.Status = "ok"
			pDim.Message = fmt.Sprintf("功率充足：上架后 %.0fW / 额定 %.0fW（%.0f%%）", pDim.After, pDim.Limit, ratio*100)
		}
	} else {
		pDim.Status = "na"
		pDim.Message = "机柜未设额定功率，不做功率校验"
	}
	res.Checks = append(res.Checks, pDim)

	// —— 维度3：承重 ——
	wDim := WhatIfDim{
		Dim: "weight", Label: "承重", Current: usedKG, Add: weightKG,
		After: usedKG + weightKG, Unit: "kg",
	}
	if cab.MaxWeightKG > 0 {
		wDim.Limit = float64(cab.MaxWeightKG)
		ratio := wDim.After / wDim.Limit
		switch {
		case ratio > 1:
			wDim.Status = "over"
			wDim.Message = fmt.Sprintf("承重超限：机柜上限 %dkg，上架后 %.1fkg（%.0f%%）",
				cab.MaxWeightKG, wDim.After, ratio*100)
		case ratio >= 0.9:
			wDim.Status = "warn"
			wDim.Message = fmt.Sprintf("承重偏紧：上架后达上限 %.0f%%（%.1f/%dkg）", ratio*100, wDim.After, cab.MaxWeightKG)
		default:
			wDim.Status = "ok"
			wDim.Message = fmt.Sprintf("承重充足：上架后 %.1fkg / 上限 %dkg（%.0f%%）", wDim.After, cab.MaxWeightKG, ratio*100)
		}
	} else {
		wDim.Status = "na"
		wDim.Message = "机柜未设承重上限，不做承重校验"
	}
	res.Checks = append(res.Checks, wDim)

	// —— 维度4：制冷（房间级估算） ——
	cDim := WhatIfDim{Dim: "cooling", Label: "制冷", Current: float64(usedW), Add: float64(powerW), After: float64(usedW + int64(powerW)), Unit: "W"}
	// 房间 IT 负载 = 房间内所有机柜在架资产功率之和
	var roomITW int64
	db.Raw(`SELECT COALESCE(SUM(a.power_w),0) FROM t_asset a JOIN t_cabinet c ON a.cabinet_id = c.id
	      WHERE c.room_id = ? AND a.status = 1`, cab.RoomID).Scan(&roomITW)
	cDim.Current = float64(roomITW)
	cDim.After = float64(roomITW) + float64(powerW)
	var room model.Room
	roomName := ""
	if err := db.First(&room, cab.RoomID).Error; err == nil {
		roomName = room.Name
	}
	res.RoomName = roomName
	capW, units := coolingCapacityW(room.Area)
	if capW > 0 {
		cDim.Limit = capW
		ratio := cDim.After / capW
		switch {
		case ratio >= 0.9:
			cDim.Status = "over"
			cDim.Message = fmt.Sprintf("制冷能力不足：%s 估算制冷 %.0fkW（%d台×25kW，按面积%.0f㎡），上架后IT负载 %.1fkW 达 %.0f%%",
				roomName, capW/1000, units, room.Area, cDim.After/1000, ratio*100)
		case ratio >= 0.7:
			cDim.Status = "warn"
			cDim.Message = fmt.Sprintf("制冷偏紧：%s IT负载将达估算制冷能力的 %.0f%%（%.1f/%.0fkW）",
				roomName, ratio*100, cDim.After/1000, capW/1000)
		default:
			cDim.Status = "ok"
			cDim.Message = fmt.Sprintf("制冷充足：%s IT负载 %.1fkW / 估算制冷 %.0fkW（%d台，%.0f%%）",
				roomName, cDim.After/1000, capW/1000, units, ratio*100)
		}
	} else {
		cDim.Status = "na"
		cDim.Message = "房间未登记面积，无法估算制冷能力"
	}
	res.Checks = append(res.Checks, cDim)

	// —— 汇总 ——
	for _, d := range res.Checks {
		switch d.Status {
		case "over":
			res.Over = append(res.Over, d.Dim)
		case "warn":
			res.Warn = append(res.Warn, d.Dim)
		}
	}
	switch {
	case len(res.Over) > 0:
		res.Verdict = "over"
		res.Conclusion = "不可上架："
		firstOver := true
		for _, d := range res.Checks {
			if d.Status == "over" {
				if !firstOver {
					res.Conclusion += "；"
				}
				firstOver = false
				res.Conclusion += d.Message
			}
		}
	case len(res.Warn) > 0:
		res.Verdict = "warn"
		res.Conclusion = "可上架但需注意："
		first := true
		for _, d := range res.Checks {
			if d.Status == "warn" {
				if !first {
					res.Conclusion += "；"
				}
				first = false
				res.Conclusion += d.Message
			}
		}
	default:
		res.Verdict = "ok"
		res.Conclusion = fmt.Sprintf("可上架：机柜 %s 四维容量余量充足", cab.Code)
	}
	return res
}

// CabinetWhatIf GET /cabinets/:id/what-if?height_u=&power_w=&weight_kg=
func (h *Handler) CabinetWhatIf(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var cab model.Cabinet
	if err := h.DB.First(&cab, id).Error; err != nil {
		response.NotFound(c, "机柜不存在")
		return
	}
	if !middleware.InSiteScope(c, cab.SiteID) {
		response.Forbidden(c, "无权查看该机柜")
		return
	}
	heightU, _ := strconv.Atoi(c.DefaultQuery("height_u", "1"))
	powerW, _ := strconv.Atoi(c.DefaultQuery("power_w", "0"))
	weightKG, _ := strconv.ParseFloat(c.DefaultQuery("weight_kg", "0"), 64)
	res := whatIfCheck(h.DB, cab, heightU, powerW, weightKG)
	response.Success(c, res)
}
