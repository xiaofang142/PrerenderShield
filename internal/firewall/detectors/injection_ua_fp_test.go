package detectors

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestInjectionDetector_NoFalsePositiveOnRealUA 回归测试（R12-BUG-3）：
// 主流浏览器/爬虫 UA 含分号、斜杠、括号，旧字符级模式会整批 403 误杀。
func TestInjectionDetector_NoFalsePositiveOnRealUA(t *testing.T) {
	d := NewInjectionDetector(nil)
	benign := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
		"sqlmap/1.8", // 自定义 WAF 规则的职责范围，而非内置命令注入规则
		"r12custombot/1.0",
		"curl/8.4.0",
	}
	for _, ua := range benign {
		req := httptest.NewRequest("GET", "/?q=hello+world&page=2", nil)
		req.Header.Set("User-Agent", ua)
		threats, err := d.Detect(req)
		if err != nil {
			t.Fatalf("Detect error for %q: %v", ua, err)
		}
		if len(threats) > 0 {
			t.Fatalf("benign UA falsely flagged: %q -> %+v", ua, threats)
		}
	}
}

// TestInjectionDetector_StillCatchesRealAttacks 修复后真实攻击载荷仍须命中
func TestInjectionDetector_StillCatchesRealAttacks(t *testing.T) {
	d := NewInjectionDetector(nil)
	malicious := []struct {
		name string
		uri  string
	}{
		{"rm -rf via pipe", "/cmd?exec=" + url.QueryEscape("|rm -rf /")},
		{"semicolon cat passwd", "/cmd?x=" + url.QueryEscape(";cat /etc/passwd")},
		{"command substitution", "/?f=$(whoami)"},
		{"backtick exec", "/?a=`id`"},
		{"union select", "/?q=" + url.QueryEscape("1 UNION SELECT * FROM users--")},
		{"drop table", "/?u=" + url.QueryEscape("admin'; DROP TABLE users;--")},
	}
	for _, tc := range malicious {
		req := httptest.NewRequest("GET", tc.uri, nil)
		threats, _ := d.Detect(req)
		if len(threats) == 0 {
			t.Fatalf("attack not caught: %s (%s)", tc.name, tc.uri)
		}
	}
}
