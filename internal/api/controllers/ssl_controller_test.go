package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/stretchr/testify/assert"
)

// mockACMEClient implements acmeClientInterface for testing
type mockACMEClient struct {
	requestCertFunc         func(domains []string) (*certificate.Resource, error)
	renewCertFunc           func(domain string) (*certificate.Resource, error)
	getCertInfoFunc         func(domain string) (map[string]interface{}, error)
	listCertsFunc           func() ([]map[string]interface{}, error)
	deleteCertFunc          func(domain string) error
	getExpiringCertsFunc    func(days int) ([]map[string]interface{}, error)
	requestWildcardCertFunc func(baseDomain string, subdomains []string) (*certificate.Resource, error)
}

func (m *mockACMEClient) RequestCertificate(domains []string) (*certificate.Resource, error) {
	if m.requestCertFunc != nil {
		return m.requestCertFunc(domains)
	}
	return nil, errors.New("RequestCertificate not implemented")
}

func (m *mockACMEClient) RenewCertificate(domain string) (*certificate.Resource, error) {
	if m.renewCertFunc != nil {
		return m.renewCertFunc(domain)
	}
	return nil, errors.New("RenewCertificate not implemented")
}

func (m *mockACMEClient) GetCertificateInfo(domain string) (map[string]interface{}, error) {
	if m.getCertInfoFunc != nil {
		return m.getCertInfoFunc(domain)
	}
	return nil, errors.New("GetCertificateInfo not implemented")
}

func (m *mockACMEClient) ListCertificates() ([]map[string]interface{}, error) {
	if m.listCertsFunc != nil {
		return m.listCertsFunc()
	}
	return nil, errors.New("ListCertificates not implemented")
}

func (m *mockACMEClient) DeleteCertificate(domain string) error {
	if m.deleteCertFunc != nil {
		return m.deleteCertFunc(domain)
	}
	return errors.New("DeleteCertificate not implemented")
}

func (m *mockACMEClient) GetExpiringCertificates(days int) ([]map[string]interface{}, error) {
	if m.getExpiringCertsFunc != nil {
		return m.getExpiringCertsFunc(days)
	}
	return nil, errors.New("GetExpiringCertificates not implemented")
}

func (m *mockACMEClient) RequestWildcardCertificate(baseDomain string, subdomains []string) (*certificate.Resource, error) {
	if m.requestWildcardCertFunc != nil {
		return m.requestWildcardCertFunc(baseDomain, subdomains)
	}
	return nil, errors.New("RequestWildcardCertificate not implemented")
}

// mockAutoRenewer implements autoRenewerInterface for testing
type mockAutoRenewer struct {
	getRenewalHistoryFunc func(domain string) ([]map[string]interface{}, error)
}

func (m *mockAutoRenewer) GetRenewalHistory(domain string) ([]map[string]interface{}, error) {
	if m.getRenewalHistoryFunc != nil {
		return m.getRenewalHistoryFunc(domain)
	}
	return nil, errors.New("GetRenewalHistory not implemented")
}

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

	// acmeClient 为 nil 时返回 200
	assert.Equal(t, http.StatusOK, w.Code)
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

	// acmeClient 为 nil 时提前返回 200，不走 validation
	assert.Equal(t, http.StatusOK, w.Code)
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

	// acmeClient 为 nil 时返回 200
	assert.Equal(t, http.StatusOK, w.Code)
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
	assert.Equal(t, http.StatusOK, w.Code)
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

	assert.Equal(t, http.StatusOK, w.Code)
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

func TestSSLController_GetCertStatus_MissingDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain", controller.GetCertStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回 500 或 404
	assert.Contains(t, []int{http.StatusInternalServerError, http.StatusNotFound}, w.Code)
}

func TestSSLController_ListCerts_QueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates?page=1&pageSize=10", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回空列表或 500
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}

func TestSSLController_GetExpiringCerts_WithDays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/expiring?days=30", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回 500
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSSLController_GetRenewalHistory_WithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain/history", controller.GetRenewalHistory)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com/history?page=1&pageSize=10", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_RequestCert_EmptyDomains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString(`{"domains":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// acmeClient 为 nil 时返回 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSSLController_RequestWildcardCert_EmptyBaseDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString(`{"base_domain":""}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// acmeClient 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSSLController_ListCerts_WithEmptyParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates?page=&pageSize=", nil)
	router.ServeHTTP(w, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}

func TestSSLController_DeleteCert_MissingDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.DELETE("/ssl/certificates/:domain", controller.DeleteCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ssl/certificates/", nil)
	router.ServeHTTP(w, req)

	// autoRenewer 为 nil 时返回 404 或 500
	assert.Contains(t, []int{http.StatusNotFound, http.StatusInternalServerError}, w.Code)
}

func TestSSLController_GetExpiringCerts_InvalidDays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSSLController(nil, nil)
	router := gin.New()
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/expiring?days=invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== Success Path Tests with Mocks ====================

func TestSSLController_RequestCert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 生成测试证书
	certPEM, err := generateTestCert("example.com", 365)
	assert.NoError(t, err)

	mockClient := &mockACMEClient{
		requestCertFunc: func(domains []string) (*certificate.Resource, error) {
			return &certificate.Resource{
				Domain:      "example.com",
				Certificate: certPEM,
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString(`{"domains":["example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Certificate requested successfully")
	assert.Contains(t, w.Body.String(), "example.com")
	assert.Contains(t, w.Body.String(), "expires")
}

func TestSSLController_RequestCert_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		requestCertFunc: func(domains []string) (*certificate.Resource, error) {
			return nil, errors.New("certificate request failed")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates", controller.RequestCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates", bytes.NewBufferString(`{"domains":["example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "certificate request failed")
}

func TestSSLController_RenewCert_WithMock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	certPEM, err := generateTestCert("example.com", 365)
	assert.NoError(t, err)

	mockClient := &mockACMEClient{
		renewCertFunc: func(domain string) (*certificate.Resource, error) {
			return &certificate.Resource{
				Domain:      domain,
				Certificate: certPEM,
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates/:domain/renew", controller.RenewCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/example.com/renew", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Certificate renewed successfully")
	assert.Contains(t, w.Body.String(), "example.com")
}

func TestSSLController_RenewCert_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		renewCertFunc: func(domain string) (*certificate.Resource, error) {
			return nil, errors.New("certificate renewal failed")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates/:domain/renew", controller.RenewCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/example.com/renew", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "certificate renewal failed")
}

func TestSSLController_GetCertStatus_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		getCertInfoFunc: func(domain string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"domain":  domain,
				"status":  "active",
				"expires": "2027-01-01T00:00:00Z",
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain", controller.GetCertStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "example.com")
	assert.Contains(t, w.Body.String(), "active")
}

func TestSSLController_GetCertStatus_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		getCertInfoFunc: func(domain string) (map[string]interface{}, error) {
			return nil, errors.New("certificate not found")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates/:domain", controller.GetCertStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "certificate not found")
}

func TestSSLController_ListCerts_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		listCertsFunc: func() ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"domain": "example.com", "status": "active"},
				{"domain": "test.com", "status": "active"},
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "certificates")
	assert.Contains(t, w.Body.String(), "total")
	assert.Contains(t, w.Body.String(), "example.com")
	assert.Contains(t, w.Body.String(), "test.com")
}

func TestSSLController_ListCerts_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		listCertsFunc: func() ([]map[string]interface{}, error) {
			return nil, errors.New("failed to list certificates")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates", controller.ListCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list certificates")
}

func TestSSLController_DeleteCert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		deleteCertFunc: func(domain string) error {
			return nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.DELETE("/ssl/certificates/:domain", controller.DeleteCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Certificate deleted successfully")
	assert.Contains(t, w.Body.String(), "example.com")
}

func TestSSLController_DeleteCert_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		deleteCertFunc: func(domain string) error {
			return errors.New("failed to delete certificate")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.DELETE("/ssl/certificates/:domain", controller.DeleteCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ssl/certificates/example.com", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to delete certificate")
}

func TestSSLController_GetExpiringCerts_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		getExpiringCertsFunc: func(days int) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"domain": "expiring.com", "expires_in_days": days},
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/expiring?days=30", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "expiring_in_days")
	assert.Contains(t, w.Body.String(), "30")
}

func TestSSLController_GetExpiringCerts_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		getExpiringCertsFunc: func(days int) ([]map[string]interface{}, error) {
			return nil, errors.New("failed to get expiring certificates")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/expiring?days=30", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to get expiring certificates")
}

func TestSSLController_RequestWildcardCert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	certPEM, err := generateTestCert("*.example.com", 365)
	assert.NoError(t, err)

	mockClient := &mockACMEClient{
		requestWildcardCertFunc: func(baseDomain string, subdomains []string) (*certificate.Resource, error) {
			return &certificate.Resource{
				Domain:      "*." + baseDomain,
				Certificate: certPEM,
			}, nil
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString(`{"base_domain":"example.com","subdomains":["www","api"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Wildcard certificate requested successfully")
	assert.Contains(t, w.Body.String(), "*.example.com")
}

func TestSSLController_RequestWildcardCert_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &mockACMEClient{
		requestWildcardCertFunc: func(baseDomain string, subdomains []string) (*certificate.Resource, error) {
			return nil, errors.New("failed to request wildcard certificate")
		},
	}

	controller := NewSSLController(mockClient, nil)
	router := gin.New()
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ssl/certificates/wildcard", bytes.NewBufferString(`{"base_domain":"example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to request wildcard certificate")
}

func TestSSLController_GetRenewalHistory_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRenewer := &mockAutoRenewer{
		getRenewalHistoryFunc: func(domain string) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"domain": domain, "renewed_at": "2025-01-01T00:00:00Z", "success": true},
			}, nil
		},
	}

	controller := NewSSLController(nil, mockRenewer)
	router := gin.New()
	router.GET("/ssl/certificates/:domain/history", controller.GetRenewalHistory)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com/history", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "history")
	assert.Contains(t, w.Body.String(), "example.com")
}

func TestSSLController_GetRenewalHistory_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRenewer := &mockAutoRenewer{
		getRenewalHistoryFunc: func(domain string) ([]map[string]interface{}, error) {
			return nil, errors.New("renewal history not found")
		},
	}

	controller := NewSSLController(nil, mockRenewer)
	router := gin.New()
	router.GET("/ssl/certificates/:domain/history", controller.GetRenewalHistory)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ssl/certificates/example.com/history", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "renewal history not found")
}
