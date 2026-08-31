package utils

import "sync"

// 内置允许来源（不可变，无需加锁）
var allowedOriginsStatic = map[string]bool{
	"http://localhost:9597": true,
	"http://localhost:3000": true,
	"http://127.0.0.1:9597": true,
	"http://127.0.0.1:3000": true,
}

var (
	customOriginsMu sync.RWMutex
	// customOrigins 运行时可被 SetAllowedOrigins 重写（配置热更新），
	// 而 Origin 校验每请求并发读，必须用 RWMutex 保护
	customOrigins map[string]bool
)

// SetAllowedOrigins 设置自定义允许来源（整体覆盖）
func SetAllowedOrigins(origins []string) {
	m := make(map[string]bool, len(origins))
	for _, o := range origins {
		m[o] = true
	}
	customOriginsMu.Lock()
	customOrigins = m
	customOriginsMu.Unlock()
}

// AddDynamicOrigin 增量追加运行时来源（R14-BUG-3）：
// 进程实际监听的 console/api 端口可配置，启动时按端口注入，
// 避免白名单硬编码端口与真实部署端口脱节。
func AddDynamicOrigin(origin string) {
	if origin == "" {
		return
	}
	customOriginsMu.Lock()
	defer customOriginsMu.Unlock()
	if customOrigins == nil {
		customOrigins = make(map[string]bool)
	}
	customOrigins[origin] = true
}

// IsOriginAllowed 校验请求来源是否在白名单内。
// HTTP CORS 中间件与 WebSocket 握手共用同一份名单，保证策略一致。
func IsOriginAllowed(origin string) bool {
	if origin == "" || allowedOriginsStatic[origin] {
		return origin != ""
	}
	customOriginsMu.RLock()
	defer customOriginsMu.RUnlock()
	return customOrigins[origin]
}
