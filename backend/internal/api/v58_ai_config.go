package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V58 大模型配置界面 ============
// 存储: t_system_setting 键 ai_provider/ai_base_url/ai_model/ai_api_key。
//   - ai_api_key 不加入 allowedSettingKeys 白名单 → ListSystemSettings 永不明文回显；专用接口只回掩码。
//   - 生效优先级: DB 配置 > ai.env 环境变量（启动载入的原值留档在 AIEnv*，恢复用）。
//   - 保存即热更新 Handler 内存字段（aiMu RWMutex 保护，v53 调用点统一走 getAI()），无需重启。

const (
	aiKeyProvider = "ai_provider"
	aiKeyBase     = "ai_base_url"
	aiKeyModel    = "ai_model"
	aiKeySecret   = "ai_api_key"
)

// getAI 锁保护读取当前生效配置
func (h *Handler) getAI() (key, base, mdl string) {
	h.aiMu.RLock()
	defer h.aiMu.RUnlock()
	return h.AIAPIKey, h.AIBaseURL, h.AIModel
}

func (h *Handler) getAIProvider() string {
	h.aiMu.RLock()
	defer h.aiMu.RUnlock()
	return h.AIProvider
}

// setAI 热更新内存配置
func (h *Handler) setAI(key, base, mdl, provider string) {
	h.aiMu.Lock()
	defer h.aiMu.Unlock()
	h.AIAPIKey, h.AIBaseURL, h.AIModel, h.AIProvider = key, base, mdl, provider
}

func (h *Handler) aiSettingGet(key string) string {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", key).Error; err == nil {
		return s.Value
	}
	return ""
}

func (h *Handler) aiSettingSet(key, val string) {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", key).Error; err == nil {
		h.DB.Model(&s).Update("value", val)
		return
	}
	h.DB.Create(&model.SystemSetting{Key: key, Value: val})
}

// LoadAIConfigFromDB 启动时调用：DB 有配置则覆盖 env 值
func (h *Handler) LoadAIConfigFromDB() bool {
	base := h.aiSettingGet(aiKeyBase)
	if base == "" {
		return false
	}
	key := h.aiSettingGet(aiKeySecret)
	mdl := h.aiSettingGet(aiKeyModel)
	h.setAI(key, base, mdl, h.aiSettingGet(aiKeyProvider))
	return true
}

// aiConfigSource 当前生效配置来源: db / env / none
func (h *Handler) aiConfigSource() string {
	if h.aiSettingGet(aiKeyBase) != "" {
		return "db"
	}
	if h.AIEnvKey != "" && h.AIEnvBase != "" {
		return "env"
	}
	return "none"
}

func maskKeyTail(k string) string {
	if len(k) <= 4 {
		if k == "" {
			return ""
		}
		return "****"
	}
	return "****" + k[len(k)-4:]
}

// GetAIConfig GET /settings/ai（admin）当前配置（密钥只回掩码）
func (h *Handler) GetAIConfig(c *gin.Context) {
	key, base, mdl := h.getAI()
	response.Success(c, gin.H{
		"provider":      h.getAIProvider(),
		"base_url":      base,
		"model":         mdl,
		"api_key_set":   key != "",
		"api_key_mask":  maskKeyTail(key),
		"source":        h.aiConfigSource(),
		"enabled":       key != "" && base != "",
		"has_env_value": h.AIEnvKey != "" && h.AIEnvBase != "",
	})
}

type aiConfigReq struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"` // 留空 = 保持现有密钥不变
}

// PutAIConfig PUT /settings/ai（admin）保存并热生效
func (h *Handler) PutAIConfig(c *gin.Context) {
	var req aiConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	if req.BaseURL == "" || !strings.HasPrefix(req.BaseURL, "http") {
		response.BadRequest(c, "接口地址必填且须以 http(s) 开头")
		return
	}
	if req.Model == "" {
		response.BadRequest(c, "模型名称必填")
		return
	}
	curKey, _, _ := h.getAI()
	newKey := strings.TrimSpace(req.APIKey)
	if newKey == "" {
		newKey = curKey // 留空保持
	}
	if newKey == "" && !strings.Contains(req.BaseURL, "localhost") && !strings.Contains(req.BaseURL, "127.0.0.1") {
		response.BadRequest(c, "API Key 必填（本地 Ollama 可留空）")
		return
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "custom"
	}
	// 存 DB（密钥也在 DB——服务器本机 PostgreSQL，t_system_setting 不进白名单接口）
	h.aiSettingSet(aiKeyProvider, provider)
	h.aiSettingSet(aiKeyBase, req.BaseURL)
	h.aiSettingSet(aiKeyModel, req.Model)
	if strings.TrimSpace(req.APIKey) != "" || curKey == "" {
		h.aiSettingSet(aiKeySecret, newKey)
	}
	h.setAI(newKey, req.BaseURL, req.Model, provider)
	// 审计不记密钥
	go middleware.WriteAudit(h.DB, c, "update", "ai_config", fmt.Sprintf("provider=%s model=%s base=%s", provider, req.Model, req.BaseURL))
	response.Success(c, gin.H{"enabled": newKey != "" || strings.Contains(req.BaseURL, "localhost")})
}

// TestAIConfig POST /settings/ai/test（admin）连接测试
// body 可带临时 {base_url, model, api_key}（表单当前值），留空字段用已保存配置
func (h *Handler) TestAIConfig(c *gin.Context) {
	var req aiConfigReq
	_ = c.ShouldBindJSON(&req) // 允许空 body = 测已保存配置
	savedKey, savedBase, savedModel := h.getAI()
	base := strings.TrimSpace(req.BaseURL)
	if base == "" {
		base = savedBase
	}
	mdl := strings.TrimSpace(req.Model)
	if mdl == "" {
		mdl = savedModel
	}
	key := strings.TrimSpace(req.APIKey)
	if key == "" {
		key = savedKey
	}
	if base == "" || mdl == "" {
		response.BadRequest(c, "接口地址与模型名称不能为空")
		return
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":       mdl,
		"max_tokens":  16,
		"temperature": 0,
		"messages":    []map[string]string{{"role": "user", "content": "连接测试。请只回复两个字：正常"}},
	})
	t0 := time.Now()
	httpReq, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		response.Success(c, gin.H{"ok": false, "error": "地址无效: " + err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key != "" {
		httpReq.Header.Set("Authorization", "Bear"+"er "+key)
	}
	cli := &http.Client{Timeout: 20 * time.Second}
	resp, err := cli.Do(httpReq)
	latency := time.Since(t0).Milliseconds()
	if err != nil {
		response.Success(c, gin.H{"ok": false, "latency_ms": latency, "error": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if resp.StatusCode != 200 {
		msg := string(raw)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		hint := ""
		switch resp.StatusCode {
		case 401, 403:
			hint = "（API Key 无效或无权限）"
		case 404:
			hint = "（地址或模型名不存在，请检查 base_url 是否带 /v1、模型名是否正确）"
		case 429:
			hint = "（限流或余额不足）"
		}
		response.Success(c, gin.H{"ok": false, "latency_ms": latency, "error": fmt.Sprintf("HTTP %d %s: %s", resp.StatusCode, hint, msg)})
		return
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	_ = json.Unmarshal(raw, &out)
	reply := ""
	if len(out.Choices) > 0 {
		reply = strings.TrimSpace(out.Choices[0].Message.Content)
	}
	if len(reply) > 60 {
		reply = reply[:60] + "…"
	}
	response.Success(c, gin.H{
		"ok": true, "latency_ms": latency, "reply": reply,
		"model_echo": out.Model, "model_used": mdl,
	})
}

// ResetAIConfig DELETE /settings/ai（admin）清除 DB 配置，回退 ai.env 环境配置
func (h *Handler) ResetAIConfig(c *gin.Context) {
	for _, k := range []string{aiKeyProvider, aiKeyBase, aiKeyModel, aiKeySecret} {
		h.DB.Where("key = ?", k).Delete(&model.SystemSetting{})
	}
	h.setAI(h.AIEnvKey, h.AIEnvBase, h.AIEnvModel, "env")
	go middleware.WriteAudit(h.DB, c, "delete", "ai_config", "清除界面配置，回退环境变量")
	response.Success(c, gin.H{"source": h.aiConfigSource()})
}
