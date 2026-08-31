package detectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeCustomRulesRedis struct {
	value string
	err   error
	gets  int
}

func (f *fakeCustomRulesRedis) Get(key string) (string, error) {
	f.gets++
	return f.value, f.err
}

func customRuleReq(method, target, ua string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader("username=admin'; DROP TABLE users;--"))
	req.Header.Set("User-Agent", ua)
	return req
}

// TestCustomRuleDetector_BlockUA 回归测试（R12-BUG-1）：UI 自定义规则必须对流量生效
func TestCustomRuleDetector_BlockUA(t *testing.T) {
	payload, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{{
		ID: "r1", Name: "block-sqlmap", Field: "user_agent", Operator: "contains",
		Value: "sqlmap", Action: "block", Enabled: true,
	}}})
	d := NewCustomRuleDetector("site-x", &fakeCustomRulesRedis{value: string(payload)})

	// 命中：UA 含 sqlmap → block（high）
	req := customRuleReq("GET", "/", "sqlmap/1.8")
	threats, err := d.Detect(req)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(threats) != 1 {
		t.Fatalf("expected 1 threat, got %d", len(threats))
	}
	th := threats[0]
	if th.Severity != "high" || th.Type != "custom_rule" || th.RuleName != "block-sqlmap" {
		t.Fatalf("unexpected threat: %+v", th)
	}

	// 未命中：正常 UA → 无威胁
	threats, _ = d.Detect(customRuleReq("GET", "/", "Mozilla/5.0"))
	if len(threats) != 0 {
		t.Fatalf("normal UA should pass, got %d threats", len(threats))
	}
}

// TestCustomRuleDetector_Operators 算子矩阵：contains/equals/matches/in/gt/lt
func TestCustomRuleDetector_Operators(t *testing.T) {
	cases := []struct {
		name     string
		rule     UICustomRule
		target   string
		ua       string
		expected bool
	}{
		{"path contains dotdot", UICustomRule{ID: "a", Name: "n", Field: "path", Operator: "contains", Value: "../", Action: "block", Enabled: true}, "/a/../../etc", "", true},
		{"path equals", UICustomRule{ID: "b", Name: "n", Field: "path", Operator: "equals", Value: "/admin", Action: "block", Enabled: true}, "/admin", "", true},
		{"path equals miss", UICustomRule{ID: "b2", Name: "n", Field: "path", Operator: "equals", Value: "/admin", Action: "block", Enabled: true}, "/admin/x", "", false},
		{"query matches regex", UICustomRule{ID: "c", Name: "n", Field: "query", Operator: "matches", Value: "(?i)union\\s+select", Action: "block", Enabled: true}, "/?q=UNION+SELECT", "", true},
		{"query in list", UICustomRule{ID: "d", Name: "n", Field: "query", Operator: "in", Value: "a,b,c", Action: "block", Enabled: true}, "/?t=c", "", true},
		{"gt numeric", UICustomRule{ID: "e", Name: "n", Field: "query", Operator: "gt", Value: "100", Action: "block", Enabled: true}, "/?n=150", "", true},
		{"lt numeric miss", UICustomRule{ID: "f", Name: "n", Field: "query", Operator: "lt", Value: "100", Action: "block", Enabled: true}, "/?n=150", "", false},
		{"header contains", UICustomRule{ID: "g", Name: "n", Field: "header", Operator: "contains", Value: "onerror=", Action: "block", Enabled: true}, "/", "Mozilla", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{tc.rule}})
			d := NewCustomRuleDetector("site-x", &fakeCustomRulesRedis{value: string(payload)})
			u := tc.target
			req := httptest.NewRequest("GET", u, strings.NewReader(""))
			req.Header.Set("X-Test", "onerror=1")
			req.Header.Set("User-Agent", tc.ua)
			threats, _ := d.Detect(req)
			got := len(threats) > 0
			if got != tc.expected {
				t.Fatalf("expected %v got %v (threats=%+v)", tc.expected, got, threats)
			}
		})
	}
}

// TestCustomRuleDetector_LogAction 记录但不拦截（low severity）
func TestCustomRuleDetector_LogAction(t *testing.T) {
	payload, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{{
		ID: "l1", Name: "log-sqlmap", Field: "user_agent", Operator: "contains",
		Value: "sqlmap", Action: "log", Enabled: true,
	}}})
	d := NewCustomRuleDetector("site-x", &fakeCustomRulesRedis{value: string(payload)})
	threats, _ := d.Detect(customRuleReq("GET", "/", "sqlmap/1.8"))
	if len(threats) != 1 || threats[0].Severity != "low" {
		t.Fatalf("log action must produce low severity threat, got %+v", threats)
	}
}

// TestCustomRuleDetector_DisabledAndAllow 被禁用规则与 allow 规则不产生威胁
func TestCustomRuleDetector_DisabledAndAllow(t *testing.T) {
	payload, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{
		{ID: "d1", Name: "disabled", Field: "user_agent", Operator: "contains", Value: "sqlmap", Action: "block", Enabled: false},
		{ID: "d2", Name: "allowed", Field: "user_agent", Operator: "contains", Value: "sqlmap", Action: "allow", Enabled: true},
	}})
	d := NewCustomRuleDetector("site-x", &fakeCustomRulesRedis{value: string(payload)})
	threats, _ := d.Detect(customRuleReq("GET", "/", "sqlmap/1.8"))
	if len(threats) != 0 {
		t.Fatalf("disabled/allow rules must not fire, got %+v", threats)
	}
}

// TestCustomRuleDetector_HotReload 规则热加载：保存新规则后 ≤ 缓存TTL 生效
func TestCustomRuleDetector_HotReload(t *testing.T) {
	empty, _ := json.Marshal(customRulePayload{Rules: nil})
	withRule, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{{
		ID: "h1", Name: "late-rule", Field: "user_agent", Operator: "contains",
		Value: "evilbot", Action: "block", Enabled: true,
	}}})
	fake := &fakeCustomRulesRedis{value: string(empty)}
	d := NewCustomRuleDetector("site-x", fake)

	req := customRuleReq("GET", "/", "evilbot/1.0")
	if threats, _ := d.Detect(req); len(threats) != 0 {
		t.Fatal("no rule should match initially")
	}
	// 控制台"保存"规则 → Redis 更新 → 热加载窗口后生效（模拟规则热加载）
	fake.value = string(withRule)
	time.Sleep(customRulesCacheTTL + 10*time.Millisecond)
	threats, _ := d.Detect(req)
	if len(threats) != 1 || threats[0].RuleName != "late-rule" {
		t.Fatalf("rule should hot-reload and fire, got %+v", threats)
	}
}

// TestCustomRuleDetector_InvalidRegex 非法正则不 panic、不匹配
func TestCustomRuleDetector_InvalidRegex(t *testing.T) {
	payload, _ := json.Marshal(customRulePayload{Rules: []UICustomRule{{
		ID: "x1", Name: "bad-regex", Field: "path", Operator: "matches",
		Value: "([unclosed", Action: "block", Enabled: true,
	}}})
	d := NewCustomRuleDetector("site-x", &fakeCustomRulesRedis{value: string(payload)})
	threats, err := d.Detect(httptest.NewRequest("GET", "/anything", nil))
	if err != nil {
		t.Fatalf("invalid regex must not error: %v", err)
	}
	if len(threats) != 0 {
		t.Fatalf("invalid regex must not match, got %+v", threats)
	}
}

var _ = context.Background
