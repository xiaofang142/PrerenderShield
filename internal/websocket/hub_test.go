package websocket

import (
	"encoding/json"
	"testing"
	"time"
)

func newTestClient(id string) *Client {
	return &Client{
		send:     make(chan []byte, 8),
		userID:   id,
		channels: make(map[string]bool),
	}
}

// startTestHub 启动一个运行中的 Hub，返回清理函数
func startTestHub(t *testing.T) (*Hub, func()) {
	t.Helper()
	hub := NewHub(nil)
	go hub.Run()
	return hub, func() { hub.Stop() }
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	msg := Message{Type: "alert", Channel: "alerts", Data: map[string]string{"k": "v"}, Timestamp: 123}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "alert" || got.Channel != "alerts" || got.Timestamp != 123 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub, cleanup := startTestHub(t)
	defer cleanup()

	client := newTestClient("u1")
	hub.RegisterClient(client)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.GetClientCount() != 1 {
		t.Fatalf("expected 1 client after register, got %d", hub.GetClientCount())
	}

	hub.UnregisterClient(client)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.GetClientCount() != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", hub.GetClientCount())
	}
}

func TestHub_BroadcastDeliversToSubscribedClient(t *testing.T) {
	hub, cleanup := startTestHub(t)
	defer cleanup()

	client := newTestClient("u1")
	client.channels["alerts"] = true
	hub.RegisterClient(client)
	time.Sleep(50 * time.Millisecond) // 等待注册事件被事件循环消费

	hub.BroadcastAlert(map[string]string{"rule": "cpu_high"})

	select {
	case raw := <-client.send:
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != "alert" || msg.Channel != "alerts" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast message not delivered")
	}
}

func TestHub_BroadcastAfterStop_DoesNotPanic(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()
	hub.Stop() // 幂等

	// 停止后广播必须安全丢弃
	hub.Broadcast(Message{Type: "system"})
	hub.BroadcastToChannel("monitoring", "monitor", map[string]int{"x": 1})
	hub.BroadcastSystem("hello")
}

func TestHub_StopClosesAllClients(t *testing.T) {
	hub, _ := startTestHub(t)
	c1 := newTestClient("u1")
	c2 := newTestClient("u2")
	hub.RegisterClient(c1)
	hub.RegisterClient(c2)
	time.Sleep(50 * time.Millisecond)

	hub.Stop()

	// 两个客户端的 send 通道都应被关闭
	for i, c := range []*Client{c1, c2} {
		select {
		case _, ok := <-c.send:
			if ok {
				t.Fatalf("client %d send channel should be closed", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("client %d send channel not closed after Stop", i)
		}
	}
	if hub.GetClientCount() != 0 {
		t.Fatalf("clients should be cleared after Stop, got %d", hub.GetClientCount())
	}
}
