package sitehandler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newCrawlerTestContext(headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c, w
}

func TestServeCrawlerPage_ConditionalRequests(t *testing.T) {
	html := []byte("<html><body>rendered page content</body></html>")
	createdAt := time.Now().Unix()

	// 首次响应：200 + 弱 ETag + Last-Modified
	c, w := newCrawlerTestContext(nil)
	served := serveCrawlerPage(c, nil, http.StatusOK, html, "fresh", createdAt, 0)
	if served != http.StatusOK {
		t.Fatalf("first request must be 200, got %d", served)
	}
	etag := w.Header().Get("ETag")
	if etag == "" || !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("weak ETag expected, got %q", etag)
	}
	if w.Header().Get("Last-Modified") == "" {
		t.Fatal("Last-Modified header missing")
	}
	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatal("Vary: Accept-Encoding missing")
	}

	// If-None-Match 命中 → 304 无 body
	c2, w2 := newCrawlerTestContext(map[string]string{"If-None-Match": etag})
	served = serveCrawlerPage(c2, nil, http.StatusOK, html, "fresh", createdAt, 0)
	if served != http.StatusNotModified || w2.Code != http.StatusNotModified {
		t.Fatalf("INM must yield 304, got served=%d http=%d", served, w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Fatalf("304 must have empty body, got %d bytes", w2.Body.Len())
	}
	if w2.Header().Get("ETag") == "" {
		t.Fatal("304 must still carry ETag")
	}

	// If-None-Match 带不同 ETag → 200
	c3, w3 := newCrawlerTestContext(map[string]string{"If-None-Match": `W/"deadbeefdeadbeef"`})
	served = serveCrawlerPage(c3, nil, http.StatusOK, html, "fresh", createdAt, 0)
	if served != http.StatusOK || w3.Code != http.StatusOK {
		t.Fatalf("stale INM must yield 200, got %d", served)
	}

	// If-Modified-Since 命中 → 304（同秒/更晚）
	ims := time.Unix(createdAt, 0).UTC().Format(http.TimeFormat)
	c4, _ := newCrawlerTestContext(map[string]string{"If-Modified-Since": ims})
	served = serveCrawlerPage(c4, nil, http.StatusOK, html, "fresh", createdAt, 0)
	if served != http.StatusNotModified {
		t.Fatalf("IMS equal timestamp must yield 304, got %d", served)
	}

	// If-Modified-Since 更早 → 200
	c5, _ := newCrawlerTestContext(map[string]string{"If-Modified-Since": time.Unix(createdAt-10, 0).UTC().Format(http.TimeFormat)})
	served = serveCrawlerPage(c5, nil, http.StatusOK, html, "fresh", createdAt, 0)
	if served != http.StatusOK {
		t.Fatalf("older IMS must yield 200, got %d", served)
	}

	// 非 200 不做条件请求
	c6, w6 := newCrawlerTestContext(map[string]string{"If-None-Match": "*"})
	served = serveCrawlerPage(c6, nil, http.StatusServiceUnavailable, []byte("503 page"), "miss", 0, 0)
	if served != http.StatusServiceUnavailable || w6.Code != http.StatusServiceUnavailable {
		t.Fatalf("non-200 must bypass conditionals, got %d", served)
	}
	if w6.Header().Get("ETag") != "" {
		t.Fatal("non-200 must not carry ETag")
	}
}

func TestServeCrawlerPage_Gzip(t *testing.T) {
	big := bytes.Repeat([]byte("<p>rendered paragraph block for gzip testing</p>"), 100) // >1KB
	etag := ""

	// 客户端接受 gzip → 压缩响应
	c, w := newCrawlerTestContext(map[string]string{"Accept-Encoding": "gzip, deflate"})
	served := serveCrawlerPage(c, nil, http.StatusOK, big, "fresh", time.Now().Unix(), 0)
	if served != http.StatusOK || w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", served)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("Content-Encoding: gzip missing")
	}
	etag = w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing on gzip response")
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read failed: %v", err)
	}
	if !bytes.Equal(plain, big) {
		t.Fatal("gunzipped body mismatch")
	}

	// 客户端不接受 gzip → 明文
	c2, w2 := newCrawlerTestContext(nil)
	serveCrawlerPage(c2, nil, http.StatusOK, big, "fresh", time.Now().Unix(), 0)
	if w2.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("must not gzip when Accept-Encoding absent")
	}

	// 小响应不压缩
	c3, w3 := newCrawlerTestContext(map[string]string{"Accept-Encoding": "gzip"})
	serveCrawlerPage(c3, nil, http.StatusOK, []byte("<html>tiny</html>"), "fresh", time.Now().Unix(), 0)
	if w3.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("small body must not be gzipped")
	}

	// 压缩响应的 ETag 可用于 304 协商
	c4, w4 := newCrawlerTestContext(map[string]string{"If-None-Match": etag})
	served = serveCrawlerPage(c4, nil, http.StatusOK, big, "fresh", time.Now().Unix(), 0)
	if served != http.StatusNotModified {
		t.Fatalf("gzip ETag must negotiate 304, got %d", served)
	}
	if w4.Body.Len() != 0 {
		t.Fatal("304 body must be empty")
	}
	_ = w
}
