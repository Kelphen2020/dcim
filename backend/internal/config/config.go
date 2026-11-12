package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ServerCfg struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DBCfg struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

func (d *DBCfg) DSN() string {
	ssl := d.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	return "host=" + d.Host + " port=" + itoa(d.Port) + " user=" + d.User +
		" password=" + d.Password + " dbname=" + d.DBName + " sslmode=" + ssl + " TimeZone=Asia/Shanghai"
}

type JWTConf struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type SimulateCfg struct {
	Enabled     bool `yaml:"enabled"`
	IntervalSec int  `yaml:"interval_sec"`
}

// WechatCfg 微信小程序（AppSecret 仅服务端使用）
type WechatCfg struct {
	AppID     string `yaml:"appid"`
	AppSecret string `yaml:"app_secret"`
}

// AICfg V53 AI 运维助手（OpenAI 兼容接口；密钥优先环境变量，配置文件不落明文）
type AICfg struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type Config struct {
	Server   ServerCfg   `yaml:"server"`
	Database DBCfg       `yaml:"database"`
	JWT      JWTConf     `yaml:"jwt"`
	Simulate SimulateCfg `yaml:"simulate"`
	Wechat   WechatCfg   `yaml:"wechat"`
	AI       AICfg       `yaml:"ai"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil && path != "" {
		return nil, err
	}
	cfg := &Config{}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	// 环境变量覆盖（容器化部署免改配置文件）
	if v := os.Getenv("DCIM_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("DCIM_SERVER_PORT"); v != "" {
		cfg.Server.Port = atoiDef(v, cfg.Server.Port)
	}
	if v := os.Getenv("DCIM_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DCIM_DB_PORT"); v != "" {
		cfg.Database.Port = atoiDef(v, cfg.Database.Port)
	}
	if v := os.Getenv("DCIM_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DCIM_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DCIM_DB_NAME"); v != "" {
		cfg.Database.DBName = v
	}
	if v := os.Getenv("DCIM_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("DCIM_SIMULATE"); v == "true" || v == "1" {
		cfg.Simulate.Enabled = true
	} else if v == "false" || v == "0" {
		cfg.Simulate.Enabled = false
	}
	if v := os.Getenv("DCIM_WECHAT_APPID"); v != "" {
		cfg.Wechat.AppID = v
	}
	if v := os.Getenv("DCIM_WECHAT_SECRET"); v != "" {
		cfg.Wechat.AppSecret = v
	}
	// V53 AI 助手：DCIM_AI_* 优先，回退 CUSTOM_ALIYUN_*（本机既有百炼兼容端点）
	if v := os.Getenv("DCIM_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	} else if cfg.AI.APIKey == "" {
		if v := os.Getenv("CUSTOM_ALIYUN_API_KEY"); v != "" {
			cfg.AI.APIKey = v
		}
	}
	if v := os.Getenv("DCIM_AI_BASE_URL"); v != "" {
		cfg.AI.BaseURL = v
	} else if cfg.AI.BaseURL == "" {
		if v := os.Getenv("CUSTOM_ALIYUN_BASE_URL"); v != "" {
			cfg.AI.BaseURL = v
		}
	}
	if v := os.Getenv("DCIM_AI_MODEL"); v != "" {
		cfg.AI.Model = v
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "qwen3.7-plus"
	}
	// 容器/裸机默认值兜底
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8095
	}
	if cfg.Database.Host == "" {
		cfg.Database.Host = "127.0.0.1"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = "dcim-default-secret"
	}
	return cfg, nil
}

func atoiDef(s string, def int) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
