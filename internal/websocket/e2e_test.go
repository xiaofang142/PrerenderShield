package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 端到端模拟：真实 WS 客户端拨号 → 欢迎消息 → 订阅 → 收到频道广播 → 取消订阅 → 收不到
func TestWebSocketE2E_SubscribeAndBroadcast(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws/realtime", HandleWebSocket(hub, nil))
	srv := httptest.NewServer(router)
	defer srv.Close()

	// gorilla 拨号（ws://）
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/realtime"
	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "ignored")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial failed: %v (http=%v)", err, resp)
	}
	defer conn.Close()

	readMsg := func(timeout time.Duration) (Message, error) {
		conn.SetReadDeadline(time.Now().Add(timeout))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return Message{}, err
		}
		var m Message
		err = json.Unmarshal(raw, &m)
		return m, err
	}

	// 1. 欢迎消息
	welcome, err := readMsg(3 * time.Second)
	if err != nil || welcome.Type != "system" {
		t.Fatalf("welcome broken: %+v err=%v", welcome, err)
	}

	// 2. 订阅 alerts 频道
	sub, _ := json.Marshal(map[string]string{"action": "subscribe", "channel": "alerts"})
	if err := conn.WriteMessage(websocket.TextMessage, sub); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	// 3. 服务端广播 → 客户端收到
	hub.BroadcastAlert(map[string]string{"rule": "e2e-rule"})
	alertMsg, err := readMsg(3 * time.Second)
	if err != nil || alertMsg.Channel != "alerts" {
		t.Fatalf("broadcast not received: %+v err=%v", alertMsg, err)
	}

	// 4. 取消订阅 → 收不到该频道
	unsub, _ := json.Marshal(map[string]string{"action": "unsubscribe", "channel": "alerts"})
	_ = conn.WriteMessage(websocket.TextMessage, unsub)
	time.Sleep(100 * time.Millisecond)
	hub.BroadcastAlert(map[string]string{"rule": "after-unsub"})
	if msg, err := readMsg(300 * time.Millisecond); err == nil {
		t.Fatalf("unsubscribed client must not receive: %+v", msg)
	}

	// 5. 服务端主动断开后客户端读出错（连接关闭路径）
	if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() > 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if hub.GetClientCount() != 0 {
		t.Fatalf("client must unregister after close, count=%d", hub.GetClientCount())
	}
}

// 非法升级路径：普通 HTTP GET 到 WS 端点 → 升级失败（400/非101）
func TestWebSocketE2E_UpgradeRejected(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws/realtime", HandleWebSocket(hub, nil))
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws/realtime")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatal("plain GET must not upgrade")
	}
}
