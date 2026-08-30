package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certificate"
)

// fakeACMEClient 假 ACME 客户端：模拟证书签发/续签/删除全流程（不触 Let's Encrypt）
type fakeACMEClient struct {
	certs       map[string]map[string]interface{}
	failRequest bool
	failRenew   bool
	wildcardOK  bool
	failList    bool   // ListCertificates 返回错误
	withCertPEM []byte // 签发/续签结果附带的真实证书 PEM（覆盖 parseCertificateExpiry 成功路径）
}

func (f *fakeACMEClient) RequestCertificate(domains []string) (*certificate.Resource, error) {
	if f.failRequest {
		return nil, errors.New("acme: order failed (simulated)")
	}
	res := &certificate.Resource{Domain: domains[0], Certificate: f.withCertPEM}
	f.certs[domains[0]] = map[string]interface{}{"domain": domains[0], "expires_in": 90}
	return res, nil
}

func (f *fakeACMEClient) RenewCertificate(domain string) (*certificate.Resource, error) {
	if f.failRenew {
		return nil, errors.New("acme: rate limited (simulated)")
	}
	if _, ok := f.certs[domain]; !ok {
		return nil, errors.New("certificate not found")
	}
	return &certificate.Resource{Domain: domain, Certificate: f.withCertPEM}, nil
}

func (f *fakeACMEClient) GetCertificateInfo(domain string) (map[string]interface{}, error) {
	if c, ok := f.certs[domain]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeACMEClient) ListCertificates() ([]map[string]interface{}, error) {
	if f.failList {
		return nil, errors.New("list failed (simulated)")
	}
	out := make([]map[string]interface{}, 0, len(f.certs))
	for _, c := range f.certs {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeACMEClient) DeleteCertificate(domain string) error {
	if _, ok := f.certs[domain]; !ok {
		return errors.New("not found")
	}
	delete(f.certs, domain)
	return nil
}

func (f *fakeACMEClient) GetExpiringCertificates(days int) ([]map[string]interface{}, error) {
	return nil, nil
}

func (f *fakeACMEClient) RequestWildcardCertificate(baseDomain string, subdomains []string) (*certificate.Resource, error) {
	if !f.wildcardOK {
		return nil, errors.New("dns provider not configured (simulated)")
	}
	return &certificate.Resource{Domain: "*." + baseDomain, Certificate: f.withCertPEM}, nil
}

type fakeRenewer struct {
	history map[string][]map[string]interface{}
}

func (f *fakeRenewer) GetRenewalHistory(domain string) ([]map[string]interface{}, error) {
	return f.history[domain], nil
}

func newFakeSSLRouter(c *SSLController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ssl/certificates", c.RequestCert)
	r.POST("/ssl/certificates/:domain/renew", c.RenewCert)
	r.POST("/ssl/certificates/wildcard", c.RequestWildcardCert)
	r.DELETE("/ssl/certificates/:domain", c.DeleteCert)
	r.GET("/ssl/certificates/:domain", c.GetCertStatus)
	r.GET("/ssl/certificates/:domain/renewal-history", c.GetRenewalHistory)
	return r
}

func TestSSLController_SimulatedACME_FullFlow(t *testing.T) {
	fake := &fakeACMEClient{certs: map[string]map[string]interface{}{}, wildcardOK: true}
	controller := NewSSLController(fake, &fakeRenewer{history: map[string][]map[string]interface{}{
		"sim.example": {{"renewed_at": "2026-01-01", "status": "success"}},
	}})
	router := newFakeSSLRouter(controller)

	// 1. 签发
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates", strings.NewReader(`{"domains":["sim.example"]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("request cert failed: %d %s", w.Code, w.Body.String())
	}

	// 2. 查询状态
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ssl/certificates/sim.example", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get status failed: %d", w.Code)
	}

	// 3. 续签
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/sim.example/renew", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("renew failed: %d %s", w.Code, w.Body.String())
	}

	// 4. 续签历史
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ssl/certificates/sim.example/renewal-history", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("history failed: %d", w.Code)
	}
	var hist struct {
		Data struct {
			Domain  string                   `json:"domain"`
			History []map[string]interface{} `json:"history"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &hist)
	if len(hist.Data.History) != 1 || hist.Data.Domain != "sim.example" {
		t.Fatalf("history broken: %+v", hist.Data)
	}

	// 5. 通配符签发
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/wildcard", strings.NewReader(`{"base_domain":"sim.example"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("wildcard failed: %d %s", w.Code, w.Body.String())
	}

	// 6. 删除
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/ssl/certificates/sim.example", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d", w.Code)
	}
	// 删除后再查 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ssl/certificates/sim.example", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status after delete must 404, got %d", w.Code)
	}
}

func TestSSLController_SimulatedACME_ErrorPaths(t *testing.T) {
	fake := &fakeACMEClient{certs: map[string]map[string]interface{}{}, failRequest: true, failRenew: true}
	controller := NewSSLController(fake, &fakeRenewer{})
	router := newFakeSSLRouter(controller)

	// 签发失败 → 500（上游 ACME 拒绝）
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates", strings.NewReader(`{"domains":["fail.example"]}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("simulated ACME failure must yield 500, got %d", w.Code)
	}

	// 续签失败
	fake.certs["x.example"] = map[string]interface{}{"domain": "x.example"}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/x.example/renew", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("simulated renew failure must yield 500, got %d", w.Code)
	}

	// 通配符无 DNS provider → 失败
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/wildcard", strings.NewReader(`{"base_domain":"nowild.example"}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("wildcard without dns provider must fail, got %d", w.Code)
	}

	// 续签不存在的证书 → 500（控制器将上游错误统一映射 500；lego 错误无类型化）
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/ghost.example/renew", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("renew ghost cert must 500, got %d", w.Code)
	}
}
