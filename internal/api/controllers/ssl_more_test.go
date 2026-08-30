package controllers

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCertificateExpiry_InvalidDER 合法 PEM(base64 可解码) 但 DER 内容非法 → x509 解析失败
func TestParseCertificateExpiry_InvalidDER(t *testing.T) {
	badPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("definitely-not-der-bytes")})
	_, err := parseCertificateExpiry(badPEM)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate")
}

// TestSSLController_RequestCert_RealCertPEM ACME 返回真实证书 PEM → expires 为真实时间而非 unknown
func TestSSLController_RequestCert_RealCertPEM(t *testing.T) {
	certPEM, err := generateTestCert("real.example", 90)
	require.NoError(t, err)
	fake := &fakeACMEClient{certs: map[string]map[string]interface{}{}}
	fake.withCertPEM = certPEM
	controller := NewSSLController(fake, nil)
	router := ginNewRouter()
	router.POST("/ssl/certificates", controller.RequestCert)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates", strings.NewReader(`{"domains":["real.example"]}`)))
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "unknown")
	assert.Contains(t, w.Body.String(), "expires")
}

// TestSSLController_RenewCert_RealCertPEM 续签返回真实证书 PEM
func TestSSLController_RenewCert_RealCertPEM(t *testing.T) {
	certPEM, err := generateTestCert("renew.example", 90)
	require.NoError(t, err)
	fake := &fakeACMEClient{certs: map[string]map[string]interface{}{"renew.example": {"domain": "renew.example"}}}
	fake.withCertPEM = certPEM
	controller := NewSSLController(fake, nil)
	router := newFakeSSLRouter(controller)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/renew.example/renew", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "unknown")
}

// TestSSLController_RequestWildcardCert_RealCertPEM 通配符签发返回真实证书 PEM
func TestSSLController_RequestWildcardCert_RealCertPEM(t *testing.T) {
	certPEM, err := generateTestCert("wild.example", 90)
	require.NoError(t, err)
	fake := &fakeACMEClient{certs: map[string]map[string]interface{}{}, wildcardOK: true}
	fake.withCertPEM = certPEM
	controller := NewSSLController(fake, nil)
	router := ginNewRouter()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ssl/certificates/wildcard", strings.NewReader(`{"base_domain":"wild.example"}`)))
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "unknown")
}

// TestSSLController_ListCerts_UpstreamError ListCertificates 上游错误 → 500
func TestSSLController_ListCerts_UpstreamError(t *testing.T) {
	controller := NewSSLController(&fakeACMEClient{certs: map[string]map[string]interface{}{}, failList: true}, nil)
	router := ginNewRouter()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ssl/certificates", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
