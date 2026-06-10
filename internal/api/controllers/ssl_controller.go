package controllers

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certificate"
)

// parseCertificateExpiry parse PEM certificate to get expiry date
func parseCertificateExpiry(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.NotAfter.Format("2006-01-02T15:04:05Z07:00"), nil
}

// acmeClientInterface defines the interface for ACME operations
type acmeClientInterface interface {
	RequestCertificate(domains []string) (*certificate.Resource, error)
	RenewCertificate(domain string) (*certificate.Resource, error)
	GetCertificateInfo(domain string) (map[string]interface{}, error)
	ListCertificates() ([]map[string]interface{}, error)
	DeleteCertificate(domain string) error
	GetExpiringCertificates(days int) ([]map[string]interface{}, error)
	RequestWildcardCertificate(baseDomain string, subdomains []string) (*certificate.Resource, error)
}

// autoRenewerInterface defines the interface for auto-renewal operations
type autoRenewerInterface interface {
	GetRenewalHistory(domain string) ([]map[string]interface{}, error)
}

// SSLController SSL 控制器
type SSLController struct {
	acmeClient  acmeClientInterface
	autoRenewer autoRenewerInterface
}

// NewSSLController 创建 SSL 控制器
func NewSSLController(acmeClient acmeClientInterface, autoRenewer autoRenewerInterface) *SSLController {
	return &SSLController{
		acmeClient:  acmeClient,
		autoRenewer: autoRenewer,
	}
}

// RequestCert 申请证书
// POST /api/v1/ssl/certificates
// Body: { "domains": ["example.com", "www.example.com"] }
func (c *SSLController) RequestCert(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "SSL service not initialized"})
		return
	}

	var req struct {
		Domains []string `json:"domains" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	cert, err := c.acmeClient.RequestCertificate(req.Domains)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	expires, err := parseCertificateExpiry(cert.Certificate)
	if err != nil {
		expires = "unknown"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Certificate requested successfully",
		"data": gin.H{
			"domain":  cert.Domain,
			"expires": expires,
		},
	})
}

// RenewCert 续签证书
// POST /api/v1/ssl/certificates/:domain/renew
func (c *SSLController) RenewCert(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	domain := ctx.Param("domain")

	cert, err := c.acmeClient.RenewCertificate(domain)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	expires, err := parseCertificateExpiry(cert.Certificate)
	if err != nil {
		expires = "unknown"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Certificate renewed successfully",
		"data": gin.H{
			"domain":  cert.Domain,
			"expires": expires,
		},
	})
}

// GetCertStatus 获取证书状态
// GET /api/v1/ssl/certificates/:domain
func (c *SSLController) GetCertStatus(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	domain := ctx.Param("domain")

	info, err := c.acmeClient.GetCertificateInfo(domain)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"domain": domain,
			"status": info,
		},
	})
}

// ListCerts 列出所有证书
// GET /api/v1/ssl/certificates
func (c *SSLController) ListCerts(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	certs, err := c.acmeClient.ListCertificates()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"certificates": certs,
			"total":        len(certs),
		},
	})
}

// DeleteCert 删除证书
// DELETE /api/v1/ssl/certificates/:domain
func (c *SSLController) DeleteCert(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	domain := ctx.Param("domain")

	err := c.acmeClient.DeleteCertificate(domain)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Certificate deleted successfully",
		"data": gin.H{
			"domain": domain,
		},
	})
}

// GetExpiringCerts 获取即将过期的证书
// GET /api/v1/ssl/certificates/expiring?days=30
func (c *SSLController) GetExpiringCerts(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	days := ctx.DefaultQuery("days", "30")
	var daysInt int
	if _, err := fmt.Sscanf(days, "%d", &daysInt); err != nil {
		daysInt = 30
	}

	certs, err := c.acmeClient.GetExpiringCertificates(daysInt)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"expiring_in_days": daysInt,
			"certificates":     certs,
			"total":            len(certs),
		},
	})
}

// RequestWildcardCert 申请通配符证书
// POST /api/v1/ssl/certificates/wildcard
// Body: { "base_domain": "example.com", "subdomains": ["www", "api"] }
func (c *SSLController) RequestWildcardCert(ctx *gin.Context) {
	if c.acmeClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	var req struct {
		BaseDomain string   `json:"base_domain" binding:"required"`
		Subdomains []string `json:"subdomains"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	cert, err := c.acmeClient.RequestWildcardCertificate(req.BaseDomain, req.Subdomains)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	expires, err := parseCertificateExpiry(cert.Certificate)
	if err != nil {
		expires = "unknown"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Wildcard certificate requested successfully",
		"data": gin.H{
			"domain":  cert.Domain,
			"expires": expires,
		},
	})
}

// GetRenewalHistory 获取续签历史
// GET /api/v1/ssl/certificates/:domain/renewal-history
func (c *SSLController) GetRenewalHistory(ctx *gin.Context) {
	if c.autoRenewer == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 503, "message": "SSL service not initialized"})
		return
	}

	domain := ctx.Param("domain")

	history, err := c.autoRenewer.GetRenewalHistory(domain)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"domain":  domain,
			"history": history,
		},
	})
}
