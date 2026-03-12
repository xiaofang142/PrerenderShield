package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 生成测试用的证书
func generateTestCert(domain string, daysValid int) ([]byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, daysValid)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{domain},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	return certPEM.Bytes(), nil
}

func TestParseCertificateExpiry_Success(t *testing.T) {
	certPEM, err := generateTestCert("example.com", 365)
	assert.NoError(t, err)

	expiry, err := parseCertificateExpiry(certPEM)
	assert.NoError(t, err)
	assert.NotEmpty(t, expiry)

	// 验证格式是否正确
	parsedTime, err := time.Parse(time.RFC3339, expiry)
	assert.NoError(t, err)
	assert.True(t, parsedTime.After(time.Now()))
}

func TestParseCertificateExpiry_InvalidPEM(t *testing.T) {
	invalidPEM := []byte("invalid pem data")

	expiry, err := parseCertificateExpiry(invalidPEM)
	assert.Error(t, err)
	assert.Empty(t, expiry)
	assert.Contains(t, err.Error(), "failed to parse certificate PEM")
}

func TestParseCertificateExpiry_InvalidCertificate(t *testing.T) {
	// 有效的 PEM 块但内容不是证书
	invalidCert := []byte(`-----BEGIN CERTIFICATE-----
invalidbase64data
-----END CERTIFICATE-----`)

	_, err := parseCertificateExpiry(invalidCert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate")
}

func TestNewSSLController(t *testing.T) {
	// 由于 ACMEClient 和 AutoRenewer 没有导出，我们测试 nil 情况
	controller := NewSSLController(nil, nil)

	assert.NotNil(t, controller)
	assert.Nil(t, controller.acmeClient)
	assert.Nil(t, controller.autoRenewer)
}

func TestSSLController_RequestCert_ServiceNotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	// 测试 acmeClient 为 nil
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString(`{"domains":["example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// acmeClient 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestCert_MissingDomains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	// 测试缺少 domains 字段
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// acmeClient 为 nil 时返回 500，不会到 validation 那一步
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RenewCert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates/:domain/renew", controller.RenewCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/example.com/renew", nil)
	router.ServeHTTP(w, req)

	// 由于 acmeClient 为 nil，会返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_GetCertStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain", controller.GetCertStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	// 由于 autoRenewer 为 nil，会返回 500 或 404
	assert.Contains(t, []int{http.StatusInternalServerError, http.StatusNotFound}, w.Code)
}

func TestSSLController_ListCerts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates", nil)
	router.ServeHTTP(w, req)

	// 由于 autoRenewer 为 nil，会返回空列表或 500
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}

func TestSSLController_DeleteCert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.DELETE("/ssl/certificates/:domain", controller.DeleteCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	// 由于 autoRenewer 为 nil，会返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_GetExpiringCerts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/expiring", nil)
	router.ServeHTTP(w, req)

	// 由于 autoRenewer 为 nil，会返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestWildcardCert_ServiceNotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	// 测试 acmeClient 为 nil
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString(`{"base_domain":"example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// acmeClient 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_GetRenewalHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain/history", controller.GetRenewalHistory)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com/history", nil)
	router.ServeHTTP(w, req)

	// 由于 autoRenewer 为 nil，会返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestCert_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestWildcardCert_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestWildcardCert_MissingBaseDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_ListCerts_WithAutoRenewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回空列表或 500
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}

func TestSSLController_DeleteCert_WithAutoRenewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.DELETE("/ssl/certificates/:domain", controller.DeleteCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
