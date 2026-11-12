package api

import (
	"io"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== V49 PUE 能效合规 ====================

// ListPUEPolicies 政策库列表（可按省份/区域过滤，keyword 匹配省市名）
func (h *Handler) ListPUEPolicies(c *gin.Context) {
	q := h.DB.Model(&model.PUEPolicy{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("province LIKE ?", "%"+kw+"%")
	}
	if region := strings.TrimSpace(c.Query("region")); region != "" {
		q = q.Where("region = ?", region)
	}
	items := make([]model.PUEPolicy, 0)
	if err := q.Order("id").Find(&items).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// GetSitePUEPolicy 按站点匹配政策：站点 province 字段精确匹配 → 地址包含省名模糊匹配 → 空
func (h *Handler) matchSitePolicy(site *model.Site) (*model.PUEPolicy, string) {
	var p model.PUEPolicy
	if strings.TrimSpace(site.Province) != "" {
		if err := h.DB.Where("province = ?", strings.TrimSpace(site.Province)).First(&p).Error; err == nil {
			return &p, "province"
		}
	}
	if strings.TrimSpace(site.Address) != "" {
		// 地址模糊：取地址前 3 个字符与政策省市名互相包含
		prefix := []rune(site.Address)
		if len(prefix) > 3 {
			prefix = prefix[:3]
		}
		if err := h.DB.Where("province LIKE ? OR ? LIKE ('%' || province || '%')", string(prefix)+"%", site.Address).First(&p).Error; err == nil {
			return &p, "address"
		}
	}
	return nil, "none"
}

// tariffExposure 差别电价敞口：按政策加价规则计算年额外电费
// rules 形如 "1.35-1.7:0.2;>1.7:0.5" 或 "1.4-1.8:0.11;>1.8:0.5"（PUE区间:加价元/度）
func tariffExposure(rules string, pue, annualKWh float64) (extra float64, matched string) {
	rules = strings.TrimSpace(rules)
	if rules == "" || pue <= 0 || annualKWh <= 0 {
		return 0, ""
	}
	for _, seg := range strings.Split(rules, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		parts := strings.SplitN(seg, ":", 2)
		if len(parts) != 2 {
			continue
		}
		add, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		rng := strings.TrimSpace(parts[0])
		var lo, hi float64
		if strings.HasPrefix(rng, ">") {
			hi = 999
			lo, _ = strconv.ParseFloat(strings.TrimPrefix(rng, ">"), 64)
			if pue > lo {
				return annualKWh * add, rng + " 加价 " + parts[1] + " 元/度"
			}
		} else if idx := strings.Index(rng, "-"); idx > 0 {
			lo, _ = strconv.ParseFloat(rng[:idx], 64)
			hi, _ = strconv.ParseFloat(rng[idx+1:], 64)
			if pue > lo && pue <= hi {
				return annualKWh * add, rng + " 加价 " + parts[1] + " 元/度"
			}
		}
	}
	return 0, ""
}

// EnergyCompliance 能效合规：站点当前 PUE vs 政策红线 + 电价敞口 + 补贴提示
func (h *Handler) EnergyCompliance(c *gin.Context) {
	siteID, _ := strconv.ParseUint(c.DefaultQuery("site_id", "1"), 10, 64)
	var site model.Site
	if err := h.DB.First(&site, siteID).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	policy, matchBy := h.matchSitePolicy(&site)

	// 最新 PUE 与近30天均值
	type pueRow struct {
		PUE float64
	}
	var lr pueRow
	h.DB.Raw(`SELECT COALESCE(pue,0) as pue FROM t_pue_record WHERE site_id = ? ORDER BY recorded_at DESC LIMIT 1`, siteID).Scan(&lr)
	cutoff := time.Now().AddDate(0, 0, -30)
	var avg30 float64
	h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, cutoff).Scan(&avg30)

	// 年化电量估算：近30天平均总功率 × 24 × 365
	var avgTotal float64
	h.DB.Raw(`SELECT COALESCE(AVG(total_power),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, cutoff).Scan(&avgTotal)
	annualKWh := avgTotal * 24 * 365
	if annualKWh <= 0 {
		// 无PUE记录时用IT装机功率×PUE估算
		var itPower float64
		h.DB.Raw(`SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset WHERE site_id = ? AND power_w > 0 AND status = 1`, siteID).Scan(&itPower)
		p := lr.PUE
		if p <= 0 {
			p = avg30
		}
		if p <= 0 {
			p = 1.4
		}
		annualKWh = itPower * p * 24 * 365
	}

	result := gin.H{
		"site_id":     siteID,
		"site_name":   site.Name,
		"province":    site.Province,
		"match_by":    matchBy,
		"latest_pue":  roundV49(lr.PUE),
		"avg_pue_30d": roundV49(avg30),
		"annual_kwh":  roundV49(annualKWh),
		"policy":      nil,
	}
	result["pue_notify"] = h.v73NotifyInfo(uint(siteID)) // V73/B3-3 主动告警状态（开关/持续时长/活跃告警/最近通知）
	if policy == nil {
		result["status"] = "unknown"
		result["message"] = "未匹配到省份政策（请在站点信息填写省份，或到能效合规页选择适用政策）"
		response.Success(c, result)
		return
	}

	pue := lr.PUE
	if pue <= 0 {
		pue = avg30
	}
	limit := policy.NewLargeLimit
	status := "ok"
	margin := 0.0
	if pue > 0 && limit > 0 {
		margin = roundV49(limit-pue)
		if pue > limit {
			status = "over"
		} else if margin < 0.05 {
			status = "edge"
		}
	} else if pue <= 0 {
		status = "nodata"
	}

	extra, matchedRule := tariffExposure(policy.SurchargeRules, pue, annualKWh)
	nationalAvg := 1.3 // 八大枢纽集群平均（国家数据局口径）

	result["policy"] = gin.H{
		"id": policy.ID, "province": policy.Province, "region": policy.Region,
		"new_large_limit": policy.NewLargeLimit, "new_small_limit": policy.NewSmallLimit,
		"existing_target": policy.ExistingTarget, "surcharge_rules": policy.SurchargeRules,
		"subsidy": policy.Subsidy, "features": policy.Features,
	}
	result["status"] = status
	result["margin"] = margin
	result["national_avg"] = nationalAvg
	result["surcharge_hit"] = matchedRule
	result["surcharge_annual"] = roundV49(extra)
	result["subsidy_hint"] = policy.Subsidy
	response.Success(c, result)
}

func roundV49(v float64) float64 {
	if v >= 0 {
		return float64(int64(v*100+0.5)) / 100
	}
	return float64(int64(v*100-0.5)) / 100
}

// UpsertPUEPolicy 管理员维护政策（PUT /pue-policies/:id 局部更新，POST 新增）
func (h *Handler) CreatePUEPolicy(c *gin.Context) {
	var p model.PUEPolicy
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if strings.TrimSpace(p.Province) == "" {
		response.BadRequest(c, "省份必填")
		return
	}
	var cnt int64
	h.DB.Model(&model.PUEPolicy{}).Where("province = ?", strings.TrimSpace(p.Province)).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "该省份政策已存在，请直接编辑")
		return
	}
	p.Province = strings.TrimSpace(p.Province)
	if err := h.DB.Create(&p).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	middleware.WriteAudit(h.DB, c, "create", "pue_policy", p.Province)
	response.Success(c, p)
}

func (h *Handler) UpdatePUEPolicy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p model.PUEPolicy
	if err := h.DB.First(&p, id).Error; err != nil {
		response.NotFound(c, "政策不存在")
		return
	}
	// 局部更新：raw body 键探测（陷阱：ShouldBindJSON 消费 body，ReadAll 必须先）
	body, _ := io.ReadAll(c.Request.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	strFields := map[string]string{"province": "province", "region": "region", "existing_target": "existing_target", "surcharge_rules": "surcharge_rules", "subsidy": "subsidy", "features": "features"}
	for jk, col := range strFields {
		if v, ok := m[jk]; ok {
			if s, ok2 := v.(string); ok2 {
				updates[col] = s
			}
		}
	}
	for _, jk := range []string{"new_large_limit", "new_small_limit"} {
		if v, ok := m[jk]; ok {
			switch n := v.(type) {
			case float64:
				updates[jk] = n
			case string:
				if f, err := strconv.ParseFloat(n, 64); err == nil {
					updates[jk] = f
				}
			}
		}
	}
	if len(updates) == 0 {
		response.BadRequest(c, "无可更新字段")
		return
	}
	if err := h.DB.Model(&p).Updates(updates).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	middleware.WriteAudit(h.DB, c, "update", "pue_policy", p.Province)
	h.DB.First(&p, id)
	response.Success(c, p)
}

func (h *Handler) DeletePUEPolicy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p model.PUEPolicy
	if err := h.DB.First(&p, id).Error; err != nil {
		response.NotFound(c, "政策不存在")
		return
	}
	h.DB.Delete(&p)
	middleware.WriteAudit(h.DB, c, "delete", "pue_policy", p.Province)
	response.Success(c, gin.H{"deleted": id})
}

// SeedPUEPolicies 种子：库空时导入 34 省政策（幂等，按 province 去重）
func (h *Handler) SeedPUEPolicies() int {
	var cnt int64
	h.DB.Model(&model.PUEPolicy{}).Count(&cnt)
	if cnt > 0 {
		return 0
	}
	rows := puePolicySeed()
	n := 0
	for _, r := range rows {
		if err := h.DB.Create(&r).Error; err == nil {
			n++
		}
	}
	return n
}


// 检查政策是否有站点在用（无引用限制，直接允许删除，留审计）
func (h *Handler) PolicySiteRef(province string) int64 {
	var n int64
	h.DB.Model(&model.Site{}).Where("province = ?", province).Count(&n)
	return n
}

var _ = gorm.ErrRecordNotFound
