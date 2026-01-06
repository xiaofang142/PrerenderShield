package firewall

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// DefaultActionHandler 默认动作处理器
type DefaultActionHandler struct {
	config    ActionConfig
	staticDir string
	siteName  string
}

// NewDefaultActionHandler 创建默认动作处理器
func NewDefaultActionHandler(config ActionConfig, staticDir, siteName string) *DefaultActionHandler {
	return &DefaultActionHandler{
		config:    config,
		staticDir: staticDir,
		siteName:  siteName,
	}
}

// Handle 处理请求
func (h *DefaultActionHandler) Handle(w http.ResponseWriter, req *http.Request, result *CheckResult) bool {
	if result.Allow {
		return true
	}

	// 阻止请求
	w.WriteHeader(http.StatusForbidden)

	// 尝试读取自定义拦截页面
	// 路径：staticDir/siteName/waf_block.html
	// 或者是全局的？用户需求是"站点管理 -> WAF设置"，所以是站点级别的。
	// 我们假设上传的文件名为 waf_block.html
	
	// 如果配置中指定了BlockPage路径，也可以使用
	// 但ActionConfig目前只有BlockMessage。
	
	customPagePath := filepath.Join(h.staticDir, h.siteName, "waf_block.html")
	if content, err := os.ReadFile(customPagePath); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
		return false
	}

	// 使用默认拦截页面
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	message := h.config.BlockMessage
	if message == "" {
		message = "Access Denied by WAF"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Access Denied</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding-top: 50px; }
        h1 { color: #d9534f; }
        p { color: #555; }
        .footer { margin-top: 50px; font-size: 12px; color: #999; }
    </style>
</head>
<body>
    <h1>Access Denied</h1>
    <p>%s</p>
    <div class="footer">Prerender Shield WAF</div>
</body>
</html>`, message)

	w.Write([]byte(html))
	return false
}
