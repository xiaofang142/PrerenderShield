package utils

import (
	"net/http/httptest"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	if got := FirstNonEmpty("", "a", "b"); got != "a" {
		t.Fatalf("FirstNonEmpty=%q", got)
	}
	if got := FirstNonEmpty("", ""); got != "" {
		t.Fatalf("all empty=%q", got)
	}
	if got := FirstNonEmpty("x"); got != "x" {
		t.Fatalf("single=%q", got)
	}
}

func TestSetAndGetAllowedOrigins(t *testing.T) {
	SetAllowedOrigins([]string{"http://ok.example", "http://ok2.example"})
	if !IsOriginAllowed("http://ok.example") {
		t.Fatal("allowed origin rejected")
	}
	if IsOriginAllowed("http://evil.example") {
		t.Fatal("evil origin allowed")
	}
}

func TestIsPortAvailable(t *testing.T) {
	// 超范围/非正数
	if IsPortAvailable(70000) || IsPortAvailable(-1) || IsPortAvailable(0) {
		t.Fatal("out-of-range ports must be unavailable")
	}
	// 保留端口集合
	if len(reservedPorts) > 0 {
		for p := range reservedPorts {
			if IsPortAvailable(p) {
				t.Fatalf("reserved port %d must be unavailable", p)
			}
			break
		}
	}
	// 占用检测
	ts := httptest.NewServer(nil)
	defer ts.Close()
}
