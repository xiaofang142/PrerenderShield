package ssl

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"prerender-shield/internal/logging"
)

// HTTPProvider HTTP-01 挑战提供者
type HTTPProvider struct {
	port   int
	server *http.Server
	tokens map[string]string
	mu     sync.RWMutex
}

// NewHTTPProvider 创建 HTTP 挑战提供者
func NewHTTPProvider(port int) *HTTPProvider {
	if port <= 0 {
		port = 80 // 默认使用 80 端口
	}

	h := &HTTPProvider{
		port:   port,
		tokens: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/", h.handleChallenge)

	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return h
}

// Present 设置挑战响应
func (h *HTTPProvider) Present(domain, token, keyAuth string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tokens[token] = keyAuth
	return nil
}

// CleanUp 清理挑战响应
func (h *HTTPProvider) CleanUp(domain, token, keyAuth string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.tokens, token)
	return nil
}

// handleChallenge 处理 ACME 挑战请求
func (h *HTTPProvider) handleChallenge(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/.well-known/acme-challenge/"):]

	h.mu.RLock()
	keyAuth, exists := h.tokens[token]
	h.mu.RUnlock()

	if !exists {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(keyAuth))
}

// Start 启动挑战服务器
func (h *HTTPProvider) Start() error {
	go func() {
		logging.DefaultLogger.Info("Starting HTTP-01 challenge server on port %d", h.port)
		if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
			logging.DefaultLogger.Warn("HTTP challenge server error: %v", err)
		}
	}()
	return nil
}

// Stop 停止挑战服务器
func (h *HTTPProvider) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}
