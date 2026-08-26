package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/utils"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 安全检查：验证 Origin
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // 非浏览器客户端
		}
		// 与 HTTP API 的 CORS 白名单保持一致（含配置热更新的自定义来源）
		return utils.IsOriginAllowed(origin)
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
	sendBufferSize = 256
)

// NewClient 创建新的 WebSocket 客户端
func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		userID:   userID,
		channels: make(map[string]bool),
	}
}

// HandleWebSocket 处理 WebSocket 连接的 Gin Handler
func HandleWebSocket(hub *Hub, logger *logging.StructuredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			userID = "anonymous"
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			if logger != nil {
				logger.Errorf("WebSocket upgrade failed: %v", err)
			}
			return
		}

		client := NewClient(hub, conn, userID)
		hub.RegisterClient(client)

		// 发送欢迎消息
		welcome := Message{
			Type:    "system",
			Channel: "system",
			Data: map[string]interface{}{
				"message": "WebSocket connected",
				"user_id": userID,
			},
			Timestamp: time.Now().Unix(),
		}
		if data, err := json.Marshal(welcome); err == nil {
			client.send <- data
		}

		go client.writePump()
		go client.readPump()
	}
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				if c.hub.logger != nil {
					c.hub.logger.Errorf("WebSocket read error: %v, userID: %s", err, c.userID)
				}
			}
			break
		}

		// 解析订阅/取消订阅消息
		var msg struct {
			Action  string `json:"action"`  // subscribe, unsubscribe
			Channel string `json:"channel"` // 频道名
		}
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Action {
			case "subscribe":
				c.channels[msg.Channel] = true
			case "unsubscribe":
				delete(c.channels, msg.Channel)
			}
		}
	}
}

// writePump 向客户端发送消息
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 检查消息频道，如果客户端未订阅则跳过
			var msg Message
			if err := json.Unmarshal(message, &msg); err == nil && msg.Channel != "" {
				if !c.channels[msg.Channel] && msg.Channel != "system" {
					continue // 客户端未订阅此频道
				}
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送排队消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
