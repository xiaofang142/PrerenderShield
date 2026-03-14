package waf

import (
	"net/http"

	"prerender-shield/internal/security/waf/types"
)

// ActionHandler 动作处理器接口
type ActionHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, result *types.CheckResult)
}

// DefaultActionHandler 默认动作处理器
type DefaultActionHandler struct {
	config types.ActionConfig
}

// NewDefaultActionHandler 创建默认动作处理器
func NewDefaultActionHandler(config types.ActionConfig) *DefaultActionHandler {
	return &DefaultActionHandler{
		config: config,
	}
}

// Handle 处理动作
func (h *DefaultActionHandler) Handle(w http.ResponseWriter, r *http.Request, result *types.CheckResult) {
	if result.Allowed {
		return
	}

	if result.Blocked {
		h.handleBlock(w, r, result)
		return
	}

	if result.Challenge {
		h.handleChallenge(w, r, result)
		return
	}
}

func (h *DefaultActionHandler) handleBlock(w http.ResponseWriter, r *http.Request, result *types.CheckResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(h.config.BlockMessage))
}

func (h *DefaultActionHandler) handleChallenge(w http.ResponseWriter, r *http.Request, result *types.CheckResult) {
	// 重定向到挑战页面
	if h.config.RedirectURL != "" {
		http.Redirect(w, r, h.config.RedirectURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("<html><body>Challenge Required</body></html>"))
}
