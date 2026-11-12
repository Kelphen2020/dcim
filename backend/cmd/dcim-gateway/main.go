// dcim-gateway — DCIM 站点边缘网关 v0.1 (P0 骨架)
//
// 职责：
//  1. 向中心站注册（code + secret）
//  2. 周期心跳（15s），断线自动重连
//  3. 指令通道骨架：轮询 /gateway/commands 拉取待执行指令（P3 接入实际控制）
//  4. 采集驱动插槽：P3 阶段插入 Modbus/SNMP 驱动
//
// 用法：dcim-gateway -c gateway.yaml
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const version = "0.1.0-p0"

type GWConfig struct {
	Server     string `yaml:"server"`      // 中心站地址 如 http://127.0.0.1:8095
	Code       string `yaml:"code"`        // 网关编码
	Secret     string `yaml:"secret"`      // 接入密钥
	PollSec    int    `yaml:"poll_sec"`    // 指令轮询周期
	CollectSec int    `yaml:"collect_sec"` // 采集周期（P3 生效）
}

type apiBody struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func loadConfig(path string) (*GWConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &GWConfig{PollSec: 5, CollectSec: 5}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func postJSON(url string, body interface{}) (*apiBody, error) {
	buf, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var b apiBody
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("响应解析失败: %s", string(raw))
	}
	return &b, nil
}

func getJSON(url string) (*apiBody, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var b apiBody
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func main() {
	cfgPath := flag.String("c", "gateway.yaml", "配置文件路径")
	flag.Parse()
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if cfg.Server == "" || cfg.Code == "" || cfg.Secret == "" {
		log.Fatalf("配置不完整：需要 server / code / secret")
	}
	if cfg.PollSec <= 0 {
		cfg.PollSec = 5
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("dcim-gateway %s 启动，中心站=%s 网关=%s", version, cfg.Server, cfg.Code)

	// ===== 注册（失败重试） =====
	var gatewayID, siteID uint
	hbInterval := 15
	for {
		b, err := postJSON(cfg.Server+"/api/v1/gateway/register", map[string]string{
			"code": cfg.Code, "secret": cfg.Secret, "version": version,
		})
		if err == nil && b.Code == 0 {
			var d struct {
				GatewayID  uint `json:"gateway_id"`
				SiteID     uint `json:"site_id"`
				HBInterval int  `json:"hb_interval"`
			}
			json.Unmarshal(b.Data, &d)
			gatewayID, siteID = d.GatewayID, d.SiteID
			if d.HBInterval > 0 {
				hbInterval = d.HBInterval
			}
			log.Printf("注册成功 gateway_id=%d site_id=%d 心跳间隔=%ds", gatewayID, siteID, hbInterval)
			break
		}
		msg := "网络错误"
		if err == nil {
			msg = b.Msg
		} else {
			msg = err.Error()
		}
		log.Printf("注册失败: %s，10 秒后重试", msg)
		select {
		case <-stop:
			return
		case <-time.After(10 * time.Second):
		}
	}

	hbTicker := time.NewTicker(time.Duration(hbInterval) * time.Second)
	pollTicker := time.NewTicker(time.Duration(cfg.PollSec) * time.Second)
	defer hbTicker.Stop()
	defer pollTicker.Stop()

	log.Println("进入运行循环：心跳 + 指令轮询（采集驱动待 P3 接入）")
	for {
		select {
		case <-stop:
			log.Println("收到退出信号，网关下线")
			return

		case <-hbTicker.C:
			b, err := postJSON(cfg.Server+"/api/v1/gateway/heartbeat", map[string]string{
				"code": cfg.Code, "secret": cfg.Secret,
			})
			if err != nil || b.Code != 0 {
				log.Printf("心跳失败: %v（将自动重试）", err)
			}

		case <-pollTicker.C:
			// 指令通道骨架：拉取待执行指令
			url := fmt.Sprintf("%s/api/v1/gateway/commands?code=%s&secret=%s", cfg.Server, cfg.Code, cfg.Secret)
			b, err := getJSON(url)
			if err != nil {
				continue // 中心站可能尚未实现该端点（P0 骨架期）
			}
			if b.Code == 0 {
				var cmds []map[string]interface{}
				if json.Unmarshal(b.Data, &cmds) == nil && len(cmds) > 0 {
					log.Printf("收到 %d 条待执行指令（执行器待 P3 实现）", len(cmds))
				}
			}
		}
	}
}
