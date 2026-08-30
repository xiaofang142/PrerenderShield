package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"prerender-shield/internal/logging"
)

// Client WebSocket 客户端
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	channels map[string]bool // 订阅的频道
}

// Hub WebSocket 连接管理中心
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	stopped    bool
	mu         sync.RWMutex
	logger     *logging.StructuredLogger
}

// Message WebSocket 消息结构
type Message struct {
	Type      string      `json:"type"`      // 消息类型: alert, monitor, waf, system
	Channel   string      `json:"channel"`   // 频道: alerts, monitoring, waf, system
	Data      interface{} `json:"data"`      // 消息内容
	Timestamp int64       `json:"timestamp"` // 时间戳
}

// NewHub 创建新的 Hub
func NewHub(logger *logging.StructuredLogger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
		logger:     logger,
	}
}

// Run 启动 Hub 事件循环（直至 Stop 被调用）
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			// 优雅停止：关闭所有剩余客户端连接
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
			}
			h.clients = make(map[*Client]bool)
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Infof("WebSocket hub stopped")
			}
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Infof("WebSocket client registered, totalClients: %d", len(h.clients))
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Infof("WebSocket client unregistered, totalClients: %d", len(h.clients))
			}

		case message := <-h.broadcast:
			// 解析频道：订阅了频道的客户端只收所订阅频道；未订阅任何频道的客户端收全部（兼容旧行为）
			var raw Message
			if err := json.Unmarshal(message, &raw); err == nil && raw.Channel != "" {
				h.mu.RLock()
				for client := range h.clients {
					if len(client.channels) > 0 && !client.channels[raw.Channel] {
						continue
					}
					select {
					case client.send <- message:
					default:
						// 客户端发送缓冲区已满，关闭连接
						close(client.send)
						delete(h.clients, client)
					}
				}
				h.mu.RUnlock()
			} else {
				// 无频道消息（如 system）：全员广播
				h.mu.RLock()
				for client := range h.clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
				h.mu.RUnlock()
			}
		}
	}
}

// Stop 停止 Hub 事件循环并断开所有客户端（幂等）
func (h *Hub) Stop() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true
	close(h.stop)
	h.mu.Unlock()
}

// RegisterClient 注册客户端
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient 注销客户端
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Broadcast 广播消息到所有客户端
func (h *Hub) Broadcast(msg Message) {
	msg.Timestamp = time.Now().Unix()
	data, err := json.Marshal(msg)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to marshal WebSocket message: %v", err)
		}
		return
	}
	select {
	case <-h.stop:
		return // 已停止，静默丢弃
	case h.broadcast <- data:
	default:
		// 广播通道已满，丢弃消息
	}
}

// BroadcastToChannel 广播消息到指定频道的客户端
func (h *Hub) BroadcastToChannel(channel string, msgType string, data interface{}) {
	msg := Message{
		Type:    msgType,
		Channel: channel,
		Data:    data,
	}
	h.Broadcast(msg)
}

// BroadcastAlert 广播告警消息
func (h *Hub) BroadcastAlert(alert interface{}) {
	h.BroadcastToChannel("alerts", "alert", alert)
}

// BroadcastMonitor 广播监控数据
func (h *Hub) BroadcastMonitor(metrics interface{}) {
	h.BroadcastToChannel("monitoring", "monitor", metrics)
}

// BroadcastWAFEvent 广播 WAF 事件
func (h *Hub) BroadcastWAFEvent(event interface{}) {
	h.BroadcastToChannel("waf", "waf_event", event)
}

// BroadcastSystem 广播系统消息
func (h *Hub) BroadcastSystem(message string) {
	h.BroadcastToChannel("system", "system", map[string]string{"message": message})
}

// GetClientCount 获取当前连接数
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
