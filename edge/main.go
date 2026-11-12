// dcim-edge: DCIM 边缘采集器（XM-E16 网关适配）
//
// 职责：
//  1. 启动注册（POST /gateway/register，Code 取 /proc/cpuinfo Serial 或配置）
//  2. 周期拉取测点（GET /gateway/points?since=），只执行 modbus-rtu / dio / manual(skip)
//  3. Modbus RTU 轮询（/dev/com1~/dev/com16，按串口分组串行化，避免总线冲突）
//  4. DI 位图采集（/dev/di，2 字节高位在前，bit 置 1 = 报警），支持边沿告警语义由平台阈值处理
//  5. 读数批量上报（POST /push-readings，带 X-Gateway-Code/Secret）；失败落盘缓存队列，恢复后续传
//  6. DO 命令执行（GET /gateway/commands/pull → 写 /dev/do 位图 → 回读确认 → POST result）
//  7. 心跳（POST /gateway/heartbeat2：di/do 位图、测点数、积压数、uptime、串口占用）
//
// 模拟模式（开发机无硬件）：DCIM_EDGE_SIMULATE=1 —— /dev/di、/dev/do、串口全部用内存仿真。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mb "github.com/goburrow/modbus"
)

// ---------------- 配置 ----------------

type Config struct {
	PlatformURL string `json:"platform_url"` // http://127.0.0.1:8095/api/v1
	Code        string `json:"code"`         // 网关编码（空则取 cpuinfo Serial）
	Secret      string `json:"secret"`
	CacheDir    string `json:"cache_dir"`
	Simulate    bool   `json:"simulate"`
	HbInterval  int    `json:"hb_interval"`  // 心跳秒，默认 15
	PullInterval int   `json:"pull_interval"` // 命令拉取秒，默认 3
	SyncInterval int   `json:"sync_interval"` // 测点同步秒，默认 300
}

func loadConfig(path string) *Config {
	cfg := &Config{PlatformURL: "http://127.0.0.1:8095/api/v1", CacheDir: "/var/lib/dcim-edge",
		HbInterval: 15, PullInterval: 3, SyncInterval: 300}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			log.Fatalf("配置文件解析失败: %v", err)
		}
	}
	if v := os.Getenv("DCIM_EDGE_SIMULATE"); v == "1" {
		cfg.Simulate = true
	}
	if cfg.Code == "" {
		cfg.Code = cpuSerial()
	}
	if cfg.Code == "" {
		cfg.Code = "edge-" + hostnameShort()
	}
	os.MkdirAll(cfg.CacheDir, 0755)
	return cfg
}

func cpuSerial() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Serial") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func hostnameShort() string {
	h, _ := os.Hostname()
	if i := strings.Index(h, "."); i > 0 {
		h = h[:i]
	}
	return h
}

// ---------------- HTTP 客户端（带网关认证头） ----------------

type Client struct {
	base   string
	code   string
	secret string
	http   *http.Client
}

func (c *Client) do(method, path string, body interface{}, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Code", c.code)
	req.Header.Set("X-Gateway-Secret", c.secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw)[:min(len(raw), 200)])
	}
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("响应解析失败: %v", err)
	}
	if env.Code != 0 {
		return fmt.Errorf("API 错误: %s", env.Msg)
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func min(a, b int) int { if a < b { return a }; return b }

// ---------------- 测点模型 ----------------

type Point struct {
	ID             uint   `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	DataType       string `json:"data_type"`
	PollInterval   int    `json:"poll_interval"`
	ScaleFactor    float64 `json:"scale_factor"`
	ProtocolConfig string `json:"protocol_config"`
	cfg            map[string]interface{}
}

func (p *Point) parseCfg() {
	p.cfg = map[string]interface{}{}
	_ = json.Unmarshal([]byte(p.ProtocolConfig), &p.cfg)
}
func (p *Point) cfgStr(k string) string {
	if v, ok := p.cfg[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}
func (p *Point) cfgInt(k string, def int) int {
	switch v := p.cfg[k].(type) {
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

// ---------------- DI/DO 硬件访问（含仿真） ----------------

type GPIO struct {
	sim    bool
	mu     sync.Mutex
	simDI  uint16 // 仿真 DI 位图
	simDO  uint8  // 仿真 DO 位图
}

func (g *GPIO) ReadDI() (uint16, error) {
	g.mu.Lock(); defer g.mu.Unlock()
	if g.sim {
		return g.simDI, nil
	}
	f, err := os.Open("/dev/di")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	buf := make([]byte, 2)
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, err
	}
	return uint16(buf[0])<<8 | uint16(buf[1]), nil
}

func (g *GPIO) ReadDO() (uint8, error) {
	g.mu.Lock(); defer g.mu.Unlock()
	if g.sim {
		return g.simDO, nil
	}
	f, err := os.OpenFile("/dev/do", os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	buf := make([]byte, 1)
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// WriteDO 按位操作: mode=set(或) clear(与非) write(覆写)，mask 0-15；返回回读值
func (g *GPIO) WriteDO(mask uint8, mode string) (uint8, error) {
	g.mu.Lock(); defer g.mu.Unlock()
	apply := func(cur uint8) uint8 {
		switch mode {
		case "set":
			return cur | mask
		case "clear":
			return cur & ^mask
		default: // write
			return mask & 0x0F
		}
	}
	if g.sim {
		g.simDO = apply(g.simDO)
		return g.simDO, nil
	}
	f, err := os.OpenFile("/dev/do", os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cur := make([]byte, 1)
	if _, err := io.ReadFull(f, cur); err != nil {
		return 0, err
	}
	next := apply(cur[0])
	if _, err := f.Write([]byte{next}); err != nil {
		return 0, err
	}
	// 回读确认
	back := make([]byte, 1)
	if _, err := f.ReadAt(back, 0); err == nil && back[0] != next {
		return back[0], fmt.Errorf("回读不一致: 期望0x%02X 实际0x%02X", next, back[0])
	}
	return next, nil
}

// ---------------- 断网续传缓存队列（JSON Lines 文件，无 CGO 依赖） ----------------

type CacheQueue struct {
	path string
	mu   sync.Mutex
}

func NewCacheQueue(dir string) *CacheQueue {
	return &CacheQueue{path: filepath.Join(dir, "push_queue.jsonl")}
}

func (q *CacheQueue) Append(items []map[string]interface{}) {
	q.mu.Lock(); defer q.mu.Unlock()
	f, err := os.OpenFile(q.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	for _, it := range items {
		b, _ := json.Marshal(it)
		f.Write(append(b, '\n'))
	}
}

func (q *CacheQueue) Drain(max int) ([]map[string]interface{}, int) {
	q.mu.Lock(); defer q.mu.Unlock()
	data, err := os.ReadFile(q.path)
	if err != nil || len(data) == 0 {
		return nil, 0
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var out []map[string]interface{}
	rest := []string{}
	for i, ln := range lines {
		if i < max {
			var m map[string]interface{}
			if json.Unmarshal([]byte(ln), &m) == nil {
				out = append(out, m)
			}
		} else {
			rest = append(rest, ln)
		}
	}
	os.WriteFile(q.path, []byte(strings.Join(rest, "\n")), 0644)
	if len(rest) > 0 {
		f, _ := os.OpenFile(q.path, os.O_APPEND|os.O_WRONLY, 0644)
		f.Write([]byte("\n")); f.Close()
	}
	return out, len(rest)
}

func (q *CacheQueue) Len() int {
	q.mu.Lock(); defer q.mu.Unlock()
	data, err := os.ReadFile(q.path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
}

// ---------------- Modbus RTU 串口池 ----------------

type SerialPool struct {
	sim   bool
	mu    sync.Mutex
	handlers map[string]*mb.RTUClientHandler // key: port|baud
	used  map[string]bool           // 串口占用
	simVals map[string]uint16       // 仿真寄存器值
}

func NewSerialPool(sim bool) *SerialPool {
	return &SerialPool{sim: sim, handlers: map[string]*mb.RTUClientHandler{}, used: map[string]bool{}, simVals: map[string]uint16{}}
}

func (sp *SerialPool) Read(port string, baud, slave int, regType string, addr, count int) ([]uint16, error) {
	if sp.sim {
		key := fmt.Sprintf("%s|%d|%d|%s|%d", port, slave, 0, regType, addr)
		sp.mu.Lock(); defer sp.mu.Unlock()
		sp.used[port] = true
		if v, ok := sp.simVals[key]; ok {
			return []uint16{v}, nil
		}
		// 仿真：按寄存器地址生成稳定伪值（addr%400 加小抖动）
		v := uint16((addr*7 + slave*13) % 400)
		sp.simVals[key] = v
		return []uint16{v}, nil
	}
	sp.mu.Lock()
	defer sp.mu.Unlock() // 同一总线串行化：读期间持锁，防止并发切换 SlaveId 串扰
	key := fmt.Sprintf("%s|%d", port, baud)
	h, ok := sp.handlers[key]
	if !ok {
		rtu := mb.NewRTUClientHandler(port)
		rtu.BaudRate = baud
		rtu.DataBits = 8
		rtu.Parity = "N"
		rtu.StopBits = 1
		rtu.Timeout = 2 * time.Second
		rtu.SlaveId = byte(slave)
		if err := rtu.Connect(); err != nil {
			return nil, fmt.Errorf("串口 %s 打开失败: %v", port, err)
		}
		h = rtu
		sp.handlers[key] = rtu
		sp.used[port] = true
	}
	h.SlaveId = byte(slave)

	cli := mb.NewClient(h)
	var (
		res []byte
		err error
	)
	switch regType {
	case "input":
		res, err = cli.ReadInputRegisters(uint16(addr), uint16(count))
	case "coil":
		res, err = cli.ReadCoils(uint16(addr), uint16(count))
	case "discrete":
		res, err = cli.ReadDiscreteInputs(uint16(addr), uint16(count))
	default:
		res, err = cli.ReadHoldingRegisters(uint16(addr), uint16(count))
	}
	if err != nil {
		return nil, err
	}
	if regType == "coil" || regType == "discrete" {
		vals := make([]uint16, count)
		for i := 0; i < count; i++ {
			if res[i/8]&(1<<(uint(i)%8)) != 0 {
				vals[i] = 1
			}
		}
		return vals, nil
	}
	vals := make([]uint16, 0, count)
	for i := 0; i+1 < len(res); i += 2 {
		// byte_order: ba = 大端交换（设备低位字在前）
		vals = append(vals, uint16(res[i])<<8|uint16(res[i+1]))
	}
	return vals, nil
}

func (sp *SerialPool) UsedSerials() []string {
	sp.mu.Lock(); defer sp.mu.Unlock()
	out := []string{}
	for k := range sp.used {
		out = append(out, strings.TrimPrefix(k, "/dev/"))
	}
	sort.Strings(out)
	return out
}

// ---------------- Edge 主体 ----------------

type Edge struct {
	cfg    *Config
	cli    *Client
	gpio   *GPIO
	pool   *SerialPool
	queue  *CacheQueue
	mu     sync.Mutex
	points []*Point
	byCode map[string]*Point
	serialID string
	started time.Time
	version string
}

func NewEdge(cfg *Config) *Edge {
	return &Edge{cfg: cfg,
		cli:   &Client{base: cfg.PlatformURL, code: cfg.Code, secret: cfg.Secret, http: &http.Client{Timeout: 10 * time.Second}},
		gpio:  &GPIO{sim: cfg.Simulate},
		pool:  NewSerialPool(cfg.Simulate),
		queue: NewCacheQueue(cfg.CacheDir),
		byCode: map[string]*Point{},
		started: time.Now(), version: "edge-1.0-v47",
	}
}

func (e *Edge) register() {
	var out struct {
		GatewayID uint `json:"gateway_id"`
		HbInterval int `json:"hb_interval"`
	}
	err := e.cli.do("POST", "/gateway/register", map[string]interface{}{
		"code": e.cfg.Code, "secret": e.cfg.Secret, "version": e.version,
	}, &out)
	if err != nil {
		log.Printf("[register] 注册失败（将重试）: %v", err)
		return
	}
	e.serialID = e.cfg.Code
	log.Printf("[register] 注册成功 gateway_id=%d", out.GatewayID)
}

func (e *Edge) syncPoints() {
	var out struct {
		Points []Point `json:"points"`
	}
	if err := e.cli.do("GET", "/gateway/points", nil, &out); err != nil {
		log.Printf("[sync] 拉取测点失败: %v", err)
		return
	}
	e.mu.Lock()
	e.points = make([]*Point, 0, len(out.Points))
	e.byCode = map[string]*Point{}
	for i := range out.Points {
		p := &out.Points[i]
		p.parseCfg()
		if p.PollInterval < 5 {
			p.PollInterval = 5
		}
		e.points = append(e.points, p)
		e.byCode[p.Code] = p
	}
	n := len(e.points)
	e.mu.Unlock()
	log.Printf("[sync] 测点同步完成: %d 个", n)
}

// collectOnce 执行一轮所有到期测点采集
func (e *Edge) collectOnce(last map[uint]time.Time) {
	e.mu.Lock()
	pts := make([]*Point, len(e.points))
	copy(pts, e.points)
	e.mu.Unlock()

	now := time.Now()
	readings := []map[string]interface{}{}
	for _, p := range pts {
		if p.Protocol == "manual" {
			continue // 人工录入测点不采集
		}
		if t, ok := last[p.ID]; ok && now.Sub(t) < time.Duration(p.PollInterval)*time.Second {
			continue
		}
		last[p.ID] = now
		val, err := e.collectPoint(p)
		if err != nil {
			log.Printf("[collect] %s 采集失败: %v", p.Code, err)
			readings = append(readings, map[string]interface{}{
				"point_code": p.Code, "value": 0, "quality": 2, "timestamp": now.Format(time.RFC3339)})
			continue
		}
		readings = append(readings, map[string]interface{}{
			"point_code": p.Code, "value": val, "quality": 1, "timestamp": now.Format(time.RFC3339)})
	}
	if len(readings) > 0 {
		e.pushReadings(readings)
	}
}

func (e *Edge) collectPoint(p *Point) (float64, error) {
	switch p.Protocol {
	case "modbus":
		port := p.cfgStr("port")
		if port == "" {
			return 0, fmt.Errorf("缺少串口配置 port")
		}
		baud := p.cfgInt("baud_rate", 9600)
		slave := p.cfgInt("slave_id", 1)
		regType := p.cfgStr("reg_type")
		if regType == "" {
			regType = "input"
		}
		addr := p.cfgInt("reg_addr", 0)
		count := p.cfgInt("count", 1)
		vals, err := e.pool.Read(port, baud, slave, regType, addr, count)
		if err != nil {
			return 0, err
		}
		var raw float64
		if len(vals) == 1 {
			// bool 型：非零为 1
			if p.DataType == "bool" {
				if vals[0] != 0 {
					raw = 1
				}
			} else {
				raw = float64(vals[0])
			}
		} else {
			// 32 位：字序按 byte_order，默认 ab(大端字序)
			bo := p.cfgStr("byte_order")
			if bo == "ba" && len(vals) >= 2 {
				raw = float64(uint32(vals[1])<<16 | uint32(vals[0]))
			} else {
				raw = float64(uint32(vals[0])<<16 | uint32(vals[1]))
			}
		}
		sf := p.ScaleFactor
		if sf == 0 {
			sf = 1
		}
		return raw * sf, nil
	case "dio": // DI 开关量输入
		bit := p.cfgInt("bit", -1)
		if bit < 0 {
			bit = p.cfgInt("di_bit", -1) // 前端字段名 di_bit
		}
		if bit < 0 || bit > 11 {
			return 0, fmt.Errorf("dio 测点需要 cfg.bit ∈ 0-11（DI1~DI12）")
		}
		bitmap, err := e.gpio.ReadDI()
		if err != nil {
			return 0, err
		}
		// 手册：高位在前，DI1 = bit0（最低位对应第一个通道）——0x0010 时 bit4=DI5
		invert := p.cfgStr("invert") == "1" || p.cfgStr("di_invert") == "1"
		v := float64((bitmap >> uint(bit)) & 1)
		if invert {
			if v == 1 { v = 0 } else { v = 1 }
		}
		return v, nil
	default:
		return 0, fmt.Errorf("协议 %s 不支持边缘采集", p.Protocol)
	}
}

func (e *Edge) pushReadings(items []map[string]interface{}) {
	var out struct{ Pushed int `json:"pushed"` }
	if err := e.cli.do("POST", "/push-readings", items, &out); err != nil {
		log.Printf("[push] 上报失败，落盘缓存 %d 条: %v", len(items), err)
		e.queue.Append(items)
		return
	}
	// 上报成功后尝试续传积压
	for {
		backlog, remain := e.queue.Drain(200)
		if len(backlog) == 0 {
			break
		}
		if err := e.cli.do("POST", "/push-readings", backlog, &out); err != nil {
			e.queue.Append(backlog)
			log.Printf("[push] 续传失败，回存 %d 条（剩余积压 %d）", len(backlog), e.queue.Len())
			break
		}
		log.Printf("[push] 断网续传成功 %d 条，剩余积压 %d", len(backlog), remain)
	}
}

// pollCommands 拉取并执行 DO 命令
func (e *Edge) pollCommands() {
	var out struct {
		Commands []struct {
			ID      uint   `json:"id"`
			Type    string `json:"type"`
			Payload string `json:"payload"`
		} `json:"commands"`
	}
	if err := e.cli.do("GET", "/gateway/commands/pull", nil, &out); err != nil {
		return // 静默：平台可能暂不可达
	}
	for _, cmd := range out.Commands {
		if cmd.Type != "do_set" {
			e.reportResult(cmd.ID, "failed", "不支持的命令类型: "+cmd.Type)
			continue
		}
		var p struct {
			Mask    *int   `json:"mask"`
			Channel *int   `json:"channel"`
			Action  string `json:"action"`
			Mode    string `json:"mode"`
		}
		if err := json.Unmarshal([]byte(cmd.Payload), &p); err != nil {
			e.reportResult(cmd.ID, "failed", "payload 解析失败")
			continue
		}
		var mask uint8
		mode := p.Mode
		if p.Channel != nil {
			// 单路：channel 1-4 → bit0-3；action on=set off=clear
			mask = 1 << uint(*p.Channel-1)
			if p.Action == "off" {
				mode = "clear"
			} else if mode == "" {
				mode = "set"
			}
		} else if p.Mask != nil {
			mask = uint8(*p.Mask)
			if mode == "" {
				mode = "write"
			}
		}
		got, err := e.gpio.WriteDO(mask, mode)
		if err != nil {
			e.reportResult(cmd.ID, "failed", err.Error())
			continue
		}
		e.reportResult(cmd.ID, "done", fmt.Sprintf("DO 写入成功 mode=%s mask=0x%02X 回读=0x%02X", mode, mask, got))
	}
}

func (e *Edge) reportResult(id uint, status, result string) {
	var out interface{}
	if err := e.cli.do("POST", fmt.Sprintf("/gateway/commands/%d/result", id),
		map[string]string{"status": status, "result": result}, &out); err != nil {
		log.Printf("[cmd] 命令 %d 结果回报失败: %v", id, err)
	} else {
		log.Printf("[cmd] 命令 %d %s: %s", id, status, result)
	}
}

func (e *Edge) heartbeat() {
	di, derr := e.gpio.ReadDI()
	do, derr2 := e.gpio.ReadDO()
	diState, doState := -1, -1
	if derr == nil {
		diState = int(di)
	}
	if derr2 == nil {
		doState = int(do)
	}
	e.mu.Lock()
	n := len(e.points)
	e.mu.Unlock()
	body := map[string]interface{}{
		"version":     e.version,
		"di_state":    diState,
		"do_state":    doState,
		"edge_points": n,
		"push_fail":   e.queue.Len(),
		"uptime_sec":  int64(time.Since(e.started).Seconds()),
		"serials":     strings.Join(e.pool.UsedSerials(), ","),
	}
	if err := e.cli.do("POST", "/gateway/heartbeat2", body, nil); err != nil {
		log.Printf("[hb] 心跳失败: %v", err)
	}
}

func (e *Edge) Run() {
	log.Printf("dcim-edge 启动 code=%s simulate=%v platform=%s", e.cfg.Code, e.cfg.Simulate, e.cfg.PlatformURL)
	e.register()
	e.syncPoints()

	// 注册重试 + 周期同步
	go func() {
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for range tk.C {
			if e.serialID == "" {
				e.register()
			}
		}
	}()
	go func() {
		tk := time.NewTicker(time.Duration(e.cfg.SyncInterval) * time.Second)
		defer tk.Stop()
		for range tk.C {
			e.syncPoints()
		}
	}()
	// 心跳
	go func() {
		tk := time.NewTicker(time.Duration(e.cfg.HbInterval) * time.Second)
		defer tk.Stop()
		for { e.heartbeat(); <-tk.C }
	}()
	// 命令拉取
	go func() {
		tk := time.NewTicker(time.Duration(e.cfg.PullInterval) * time.Second)
		defer tk.Stop()
		for range tk.C {
			e.pollCommands()
		}
	}()
	// 采集主循环：每秒 tick，按 poll_interval 到期采集
	last := map[uint]time.Time{}
	for {
		e.collectOnce(last)
		time.Sleep(1 * time.Second)
	}
}

func main() {
	cfgPath := "/opt/dcim/edge.json"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg := loadConfig(cfgPath)
	if cfg.Secret == "" && !cfg.Simulate {
		log.Println("警告: 未配置 secret，注册/推送将被平台拒绝")
	}
	NewEdge(cfg).Run()
}
