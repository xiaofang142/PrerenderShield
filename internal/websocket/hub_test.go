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

func TestHub_ChannelFiltering(t *testing.T) {
	hub, cleanup := startTestHub(t)
	defer cleanup()

	// 订阅 alerts 的客户端 + 订阅 monitoring（其他频道）的客户端
	sub := newTestClient("sub")
	sub.channels["alerts"] = true
	hub.RegisterClient(sub)
	other := newTestClient("other")
	other.channels["monitoring"] = true
	hub.RegisterClient(other)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastToChannel("alerts", "alert", map[string]string{"x": "1"})

	select {
	case raw := <-sub.send:
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Channel != "alerts" {
			t.Fatalf("channel mismatch: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscribed client must receive")
	}

	// 订阅其他频道的客户端不得收到 alerts（频道过滤修复回归）
	select {
	case raw := <-other.send:
		t.Fatalf("cross-channel client must not receive: %s", raw)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHub_BroadcastMonitorAndWAF(t *testing.T) {
	hub, cleanup := startTestHub(t)
	defer cleanup()

	mon := newTestClient("mon")
	mon.channels["monitoring"] = true
	waf := newTestClient("waf")
	waf.channels["waf"] = true
	hub.RegisterClient(mon)
	hub.RegisterClient(waf)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastMonitor(map[string]float64{"cpu": 1.5})
	hub.BroadcastWAFEvent(map[string]string{"ip": "1.2.3.4"})
	hub.BroadcastSystem("sysmsg")

	received := 0
	deadline := time.After(2 * time.Second)
	for received < 2 {
		select {
		case <-mon.send:
			received++
		case <-waf.send:
			received++
		case <-deadline:
			t.Fatalf("expected 2 channel deliveries, got %d", received)
		}
	}
}

func TestHub_SlowClientEvicted(t *testing.T) {
	hub, cleanup := startTestHub(t)
	defer cleanup()

	// 慢客户端：send 缓冲塞满且无人消费
	slow := newTestClient("slow")
	hub.RegisterClient(slow)
	fast := newTestClient("fast")
	fast.channels["alerts"] = true
	hub.RegisterClient(fast)
	time.Sleep(50 * time.Millisecond)

	// 灌满慢客户端缓冲（client send buffer 大小见 client.go；这里循环灌大量消息）
	for i := 0; i < 10000; i++ {
		hub.BroadcastAlert(map[string]int{"n": i})
	}

	// 慢客户端最终被事件循环剔除（缓冲满即关闭+删除），不再计入客户端数
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() > 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.GetClientCount() > 1 {
		t.Fatalf("slow client must be evicted, remaining=%d", hub.GetClientCount())
	}
}
