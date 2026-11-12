package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"
)

// V46: 测点批量导入——前端把 CSV 解析成行对象提交，后端逐行白话校验、部分成功入库。
// 目标用户是非技术人员：错误信息必须是"第3行：站点「XX机房」不存在，请检查站点名称列"这种话。

type ImportRow struct {
	RowNo        int    `json:"row_no"`
	SiteName     string `json:"site_name"`
	RoomName     string `json:"room_name"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Protocol     string `json:"protocol"`
	DataType     string `json:"data_type"`
	Unit         string `json:"unit"`
	Host         string `json:"host"`
	SlaveID      string `json:"slave_id"`
	RegType      string `json:"reg_type"`
	RegAddr      string `json:"reg_addr"`
	ScaleFactor  string `json:"scale_factor"`
	OID          string `json:"oid"`
	Topic        string `json:"topic"`
	MinThreshold string `json:"min_threshold"`
	MaxThreshold string `json:"max_threshold"`
	HighThresh   string `json:"high_threshold"`
	PollInterval string `json:"poll_interval"`
	Description  string `json:"description"`
}

type ImportReport struct {
	Total    int              `json:"total"`
	Created  int              `json:"created"`
	Failed   int              `json:"failed"`
	DryRun   bool             `json:"dry_run"`
	Errors   []ImportRowError `json:"errors"`
	CreatedP []string         `json:"created_points"`
}

type ImportRowError struct {
	RowNo int      `json:"row_no"`
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Errs  []string `json:"errors"`
}

var importTypeMap = map[string]string{
	"温湿度": "temp_humidity", "温度": "temp_humidity", "温度湿度": "temp_humidity",
	"漏水": "water_leak", "漏水检测": "water_leak",
	"液冷": "cdu", "液冷CDU": "cdu", "CDU": "cdu", "冷量分配单元": "cdu",
	"漏液绳": "leak_rope", "漏液": "leak_rope", "绳式漏液": "leak_rope",
	"烟感": "smoke", "烟雾": "smoke",
	"配电": "power_dist", "配电柜": "power_dist", "PDU": "power_dist",
	"UPS": "ups", "ups": "ups",
	"空调": "hvac", "精密空调": "hvac", "暖通": "hvac",
	"门禁": "access", "安防": "security",
	"视频": "video", "消防": "fire", "电池": "battery",
}

var importProtoMap = map[string]string{
	"Modbus": "modbus", "modbus": "modbus", "MODBUS": "modbus", "modbus-tcp": "modbus", "Modbus TCP": "modbus",
	"SNMP": "snmp", "snmp": "snmp",
	"MQTT": "mqtt", "mqtt": "mqtt",
	"手动": "manual", "人工": "manual", "manual": "manual", "无": "manual",
	"网关DI": "dio", "DI": "dio", "开关量": "dio", "dio": "dio",
}

var importRegMap = map[string]string{
	"输入寄存器": "input", "input": "input", "3x": "input", "只读": "input",
	"保持寄存器": "holding", "holding": "holding", "4x": "holding", "可写": "holding",
	"线圈": "coil", "coil": "coil", "0x": "coil",
	"离散输入": "discrete", "discrete": "discrete", "1x": "discrete",
}

// ImportMonitoringPoints POST /monitoring-points/import  {dry_run:bool, rows:[ImportRow]}
func (h *Handler) ImportMonitoringPoints(c *gin.Context) {
	var body struct {
		DryRun bool        `json:"dry_run"`
		Rows   []ImportRow `json:"rows"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if len(body.Rows) == 0 {
		response.Error(c, http.StatusBadRequest, "没有可导入的数据行")
		return
	}
	if len(body.Rows) > 500 {
		response.Error(c, http.StatusBadRequest, "单次最多导入 500 行，请拆分文件")
		return
	}

	// 预加载站点/机房映射（按名称找 id，大小写不敏感）
	sites := map[string]uint{}
	rooms := map[string]uint{}
	roomSite := map[string]uint{}
	var siteList []model.Site
	h.DB.Find(&siteList)
	for _, s := range siteList {
		sites[s.Name] = s.ID
	}
	var roomList []model.Room
	h.DB.Find(&roomList)
	for _, r := range roomList {
		var sName string
		for _, s := range siteList {
			if s.ID == r.SiteID {
				sName = s.Name
				break
			}
		}
		key := sName + "/" + r.Name
		rooms[key] = r.ID
		roomSite[r.Name] = r.SiteID
	}
	// 已有测点编码集合（查重用）
	existing := map[string]bool{}
	var codes []string
	h.DB.Model(&model.MonitoringPoint{}).Pluck("code", &codes)
	for _, cd := range codes {
		existing[cd] = true
	}

	rep := ImportReport{Total: len(body.Rows), Errors: []ImportRowError{}, CreatedP: []string{}}
	for _, row := range body.Rows {
		errs := []string{}
		addErr := func(msg string) { errs = append(errs, msg) }

		// 站点
		siteID := uint(0)
		if row.SiteName == "" {
			addErr("「站点」列为空——填机房/园区名称，如「XX数据中心」")
		} else if id, ok := sites[row.SiteName]; ok {
			siteID = id
		} else {
			addErr(fmt.Sprintf("站点「%s」不存在——请先在站点管理里创建，或核对名称（区分错别字/空格）", row.SiteName))
		}
		// 机房（可空）
		var roomID *uint
		if row.RoomName != "" {
			found := false
			for key, rid := range rooms {
				parts := splitKey(key)
				if parts == row.RoomName && (siteID == 0 || roomSite[parts] == siteID) {
					rid2 := rid; roomID = &rid2
					found = true
					break
				}
			}
			if !found {
				addErr(fmt.Sprintf("机房「%s」在该站点下不存在——核对名称，或留空稍后补", row.RoomName))
			}
		}
		// 编码
		if row.Code == "" {
			addErr("「测点编码」列为空——编码是测点的唯一身份证，如「A1-TEMP-01」")
		} else if existing[row.Code] {
			addErr(fmt.Sprintf("编码「%s」已存在——编码不能重复，请换一个", row.Code))
		} else {
			existing[row.Code] = true // 同批次内也查重
		}
		// 名称
		if row.Name == "" {
			addErr("「测点名称」列为空——填人看的名字，如「A1机柜温度」")
		}
		// 类型
		ptype := row.Type
		if cn, ok := importTypeMap[row.Type]; ok {
			ptype = cn
		}
		if ptype == "" {
			ptype = "other"
		}
		// 协议
		proto := row.Protocol
		if p, ok := importProtoMap[row.Protocol]; ok {
			proto = p
		}
		if proto == "" {
			proto = "manual"
		}
		// 数据类型
		dt := row.DataType
		dtMap := map[string]string{"浮点数": "float", "数值": "float", "float": "float", "整数": "int", "int": "int", "开关": "bool", "布尔": "bool", "bool": "bool", "文字": "string", "string": "string"}
		if d, ok := dtMap[dt]; ok {
			dt = d
		}
		if dt == "" {
			dt = "float"
		}
		// 协议配置校验
		cfg := map[string]interface{}{}
		switch proto {
		case "modbus":
			host := row.Host
			if host == "" {
				addErr("Modbus 协议必须填「设备IP:端口」列（如 10.1.2.3:502）——这是采集网关或设备的地址")
			} else {
				cfg["host"] = host
				cfg["conn_type"] = "tcp"
			}
			sid := 1
			if row.SlaveID != "" {
				v, err := strconv.Atoi(row.SlaveID)
				if err != nil || v < 1 || v > 247 {
					addErr(fmt.Sprintf("「从站地址」%s 不是 1-247 的数字——它是设备编号，见设备说明书", row.SlaveID))
				} else {
					sid = v
				}
			}
			cfg["slave_id"] = sid
			rt := "input"
			if row.RegType != "" {
				if v, ok := importRegMap[row.RegType]; ok {
					rt = v
				} else {
					addErr(fmt.Sprintf("「寄存器类型」%s 不认识——可填：输入寄存器/保持寄存器/线圈/离散输入", row.RegType))
				}
			}
			cfg["reg_type"] = rt
			if row.RegAddr != "" {
				v, err := strconv.Atoi(row.RegAddr)
				if err != nil || v < 0 {
					addErr(fmt.Sprintf("「寄存器地址」%s 不是非负整数——如 210，从设备点表里抄", row.RegAddr))
				} else {
					cfg["reg_addr"] = v
				}
			}
			sf := 1.0
			if row.ScaleFactor != "" {
				v, err := strconv.ParseFloat(row.ScaleFactor, 64)
				if err != nil || v <= 0 {
					addErr(fmt.Sprintf("「缩放系数」%s 不是大于0的数字——常见 0.1（设备传2205=实际220.5）或 1（不换算）", row.ScaleFactor))
				} else {
					sf = v
				}
			}
			cfg["scale_factor"] = sf
		case "snmp":
			if row.Host == "" {
				addErr("SNMP 协议必须填「设备IP」列")
			} else {
				cfg["host"] = row.Host
			}
			if row.OID == "" {
				addErr("SNMP 协议必须填「OID」列——数据编号，如 .1.3.6.1.4.1.2021.10.1.3.1（温度），从设备 MIB 手册查")
			} else {
				cfg["oid"] = row.OID
			}
			cfg["snmp_ver"] = "v2c"
			cfg["community"] = "public"
			cfg["port"] = 161
		case "mqtt":
			if row.Topic == "" {
				addErr("MQTT 协议必须填「Topic」列——设备发布数据的频道，如 sensors/temp-01")
			} else {
				cfg["topic"] = row.Topic
			}
			cfg["json_path"] = "$.value"
		case "dio":
			// 网关 DI：复用「从站地址」列作为 DI 通道号（1-12），模板表头注明
			ch := 0
			if row.SlaveID != "" {
				v, err := strconv.Atoi(row.SlaveID)
				if err != nil || v < 1 || v > 12 {
					addErr(fmt.Sprintf("网关DI 协议请在「从站地址」列填 DI 通道号（1-12），当前值 %s 无效", row.SlaveID))
				} else {
					ch = v - 1
				}
			} else {
				addErr("网关DI 协议必须在「从站地址」列填 DI 通道号（1-12），即接线的 DI 端子编号")
			}
			cfg["di_bit"] = ch
			cfg["di_invert"] = "0"
		case "manual":
			// 手动录入无需配置
		default:
			addErr(fmt.Sprintf("「协议」%s 不认识——可填：Modbus / SNMP / MQTT / 手动", row.Protocol))
		}
		// 阈值
		parseOptFloat := func(s, label string) *float64 {
			if s == "" {
				return nil
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				addErr(fmt.Sprintf("「%s」%s 不是数字", label, s))
				return nil
			}
			return &v
		}
		minT := parseOptFloat(row.MinThreshold, "下限")
		maxT := parseOptFloat(row.MaxThreshold, "上限")
		highT := parseOptFloat(row.HighThresh, "过高线")
		if minT != nil && maxT != nil && *minT >= *maxT {
			addErr(fmt.Sprintf("下限(%v)应小于上限(%v)——下限是「低于报警」，上限是「高于报警」", *minT, *maxT))
		}
		if highT != nil && maxT != nil && *highT <= *maxT {
			addErr(fmt.Sprintf("过高线(%v)应大于上限(%v)——过高线是严重告警，要比普通报警更严", *highT, *maxT))
		}
		poll := 30
		if row.PollInterval != "" {
			v, err := strconv.Atoi(row.PollInterval)
			if err != nil || v < 5 || v > 3600 {
				addErr(fmt.Sprintf("「采集间隔」%s 不是 5-3600 的秒数——安全类建议5秒，一般环境30秒", row.PollInterval))
			} else {
				poll = v
			}
		}

		if len(errs) > 0 {
			rep.Failed++
			rep.Errors = append(rep.Errors, ImportRowError{RowNo: row.RowNo, Code: row.Code, Name: row.Name, Errs: errs})
			continue
		}

		cfgJSON := "{}"
		if len(cfg) > 0 {
			b, _ := json.Marshal(cfg)
			cfgJSON = string(b)
		}
		addr := ""
		if proto == "modbus" && cfg["host"] != nil {
			addr = fmt.Sprintf("modbus://%v/%v/%v/%v", cfg["host"], cfg["slave_id"], cfg["reg_type"], cfg["reg_addr"])
		} else if proto == "snmp" && cfg["host"] != nil {
			addr = fmt.Sprintf("snmp://%v:161/%v", cfg["host"], cfg["oid"])
		} else if proto == "mqtt" && cfg["topic"] != nil {
			addr = fmt.Sprintf("mqtt://%v", cfg["topic"])
		}
		sf := 1.0
		if v, ok := cfg["scale_factor"].(float64); ok {
			sf = v
		}
		pt := model.MonitoringPoint{
			SiteID: siteID, RoomID: roomID,
			Code: row.Code, Name: row.Name, Type: ptype,
			Protocol: proto, Address: addr, DataType: dt, Unit: row.Unit,
			MinThreshold: minT, MaxThreshold: maxT, HighThreshold: highT,
			ScaleFactor: sf, PollInterval: poll,
			ProtocolConfig: cfgJSON, Description: row.Description,
		}
		if !body.DryRun {
			if err := h.DB.Create(&pt).Error; err != nil {
				rep.Failed++
				rep.Errors = append(rep.Errors, ImportRowError{RowNo: row.RowNo, Code: row.Code, Name: row.Name,
					Errs: []string{"写入数据库失败: " + err.Error()}})
				delete(existing, row.Code)
				continue
			}
		}
		rep.Created++
		rep.CreatedP = append(rep.CreatedP, row.Code)
	}
	rep.DryRun = body.DryRun

	if !body.DryRun && rep.Created > 0 {
		go middleware.WriteAudit(h.DB, c, "import", "monitoring_point", fmt.Sprintf("批量导入: 成功%d 失败%d", rep.Created, rep.Failed))
	}
	response.Success(c, rep)
}

func splitKey(key string) string {
	// "siteName/roomName" -> roomName
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[i+1:]
		}
	}
	return key
}
