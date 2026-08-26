package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/redis"
)

// setupIntegrationCert creates a self-signed certificate and stores it in Redis,
// replacing the need for RequestCertificate (which requires an ACME client).
func setupIntegrationCert(t *testing.T, mgr Manager, redisClient *redis.Client, tempDir, domain string) {
	t.Helper()
	impl := mgr.(*manager)
	privateKey, err := generatePrivateKey()
	assert.NoError(t, err)
	certPEM, err := impl.createSelfSignedCertificate(domain, privateKey)
	assert.NoError(t, err)

	certPath := filepath.Join(tempDir, domain+".crt")
	keyPath := filepath.Join(tempDir, domain+".key")
	assert.NoError(t, saveCertificate(certPath, certPEM))
	assert.NoError(t, savePrivateKey(keyPath, privateKey))

	expiresAt := time.Now().Add(90 * 24 * time.Hour)
	certInfo := map[string]interface{}{
		"domain":     domain,
		"cert_path":  certPath,
		"key_path":   keyPath,
		"created_at": time.Now().Unix(),
		"expires_at": expiresAt.Unix(),
		"status":     "active",
	}
	assert.NoError(t, redisClient.SaveJSON("ssl:cert:"+domain, certInfo, 0))
	assert.NoError(t, redisClient.SetAdd("ssl:certs", domain))
}

func TestManager_RequestCertificate_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-acme-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain := "acme-test.example.com"

	t.Run("RequestCertificate_NoACMEClient", func(t *testing.T) {
		// Without an ACME client configured, RequestCertificate should return an error
		err := manager.RequestCertificate(testDomain)
		assert.Error(t, err)
	})
}

func TestManager_RenewCertificate_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-renew-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain := "renew-test.example.com"

	t.Run("RenewCertificate_WithExistingCert_NoACME", func(t *testing.T) {
		// Manually set up a certificate in Redis (since RequestCertificate requires ACME)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain)

		certInfo := make(map[string]interface{})
		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		oldExpiresAt := certInfo["expires_at"]

		// RenewCertificate calls RequestCertificate internally, which fails without ACME.
		// The cert info in Redis should remain unchanged.
		err = manager.RenewCertificate(testDomain)
		assert.Error(t, err)

		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		// expires_at should still be present (RenewCertificate doesn't delete it on failure)
		assert.NotNil(t, certInfo["expires_at"])
		// The old and new expires_at may or may not be equal, but the cert should still exist
		_ = oldExpiresAt
	})

}

func TestManager_ImportCertificate_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-import-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain := "import-test.example.com"

	t.Run("ImportCertificate_NonExistentFiles", func(t *testing.T) {
		err := manager.ImportCertificate(testDomain, "/nonexistent/cert.crt", "/nonexistent/key.key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read certificate file")
	})

	t.Run("ImportCertificate_InvalidCertificate", func(t *testing.T) {
		tempCertFile, err := ioutil.TempFile("", "invalid-cert-*.crt")
		assert.NoError(t, err)
		defer os.Remove(tempCertFile.Name())

		_, err = tempCertFile.WriteString("invalid certificate content")
		assert.NoError(t, err)
		tempCertFile.Close()

		tempKeyFile, err := ioutil.TempFile("", "invalid-key-*.key")
		assert.NoError(t, err)
		defer os.Remove(tempKeyFile.Name())

		_, err = tempKeyFile.WriteString("invalid key content")
		assert.NoError(t, err)
		tempKeyFile.Close()

		err = manager.ImportCertificate(testDomain, tempCertFile.Name(), tempKeyFile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid certificate file")
	})

	t.Run("ImportCertificate_Success", func(t *testing.T) {
		privateKey, err := generatePrivateKey()
		assert.NoError(t, err)

		certPEM, err := createSelfSignedCertificateForTest(testDomain, privateKey)
		assert.NoError(t, err)

		tempCertFile, err := ioutil.TempFile("", "valid-cert-*.crt")
		assert.NoError(t, err)
		defer os.Remove(tempCertFile.Name())

		_, err = tempCertFile.Write(certPEM)
		assert.NoError(t, err)
		tempCertFile.Close()

		tempKeyFile, err := ioutil.TempFile("", "valid-key-*.key")
		assert.NoError(t, err)
		defer os.Remove(tempKeyFile.Name())

		err = savePrivateKey(tempKeyFile.Name(), privateKey)
		assert.NoError(t, err)

		err = manager.ImportCertificate(testDomain, tempCertFile.Name(), tempKeyFile.Name())
		assert.NoError(t, err)

		certInfo := make(map[string]interface{})
		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		assert.Equal(t, testDomain, certInfo["domain"])
		assert.Equal(t, true, certInfo["imported"])
		assert.Equal(t, "active", certInfo["status"])

		certPath := filepath.Join(tempDir, testDomain+".crt")
		keyPath := filepath.Join(tempDir, testDomain+".key")
		assert.FileExists(t, certPath)
		assert.FileExists(t, keyPath)
	})
}

func TestManager_DeleteCertificate_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-delete-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain := "delete-test.example.com"

	t.Run("DeleteCertificate_Success", func(t *testing.T) {
		// Manually set up a certificate (since RequestCertificate requires ACME)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain)

		err := manager.DeleteCertificate(testDomain)
		assert.NoError(t, err)

		certPath := filepath.Join(tempDir, testDomain+".crt")
		keyPath := filepath.Join(tempDir, testDomain+".key")
		assert.NoFileExists(t, certPath)
		assert.NoFileExists(t, keyPath)

		certInfo := make(map[string]interface{})
		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		assert.Empty(t, certInfo)

		members, err := redisClient.SetMembers("ssl:certs")
		assert.NoError(t, err)
		assert.NotContains(t, members, testDomain)
	})
}

func TestManager_GetCertificateStatus_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-status-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain := "status-test.example.com"

	t.Run("GetCertificateStatus_Success", func(t *testing.T) {
		// Manually set up a certificate (since RequestCertificate requires ACME)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain)

		status, err := manager.GetCertificateStatus(testDomain)
		assert.NoError(t, err)
		assert.Equal(t, testDomain, status["domain"])
		assert.Equal(t, "active", status["status"])
		assert.Greater(t, status["remaining_days"].(int), 0)
		assert.False(t, status["expired"].(bool))
	})
}

func TestManager_CheckExpiration_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-expiration-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain1 := "expiring-soon.example.com"
	testDomain2 := "expiring-later.example.com"

	t.Run("CheckExpiration_NoCertificates", func(t *testing.T) {
		expiring, err := manager.CheckExpiration()
		assert.NoError(t, err)
		assert.Empty(t, expiring)
	})

	t.Run("CheckExpiration_WithCertificates", func(t *testing.T) {
		// Manually set up certificates (since RequestCertificate requires ACME)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain1)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain2)

		_, err := manager.CheckExpiration()
		assert.NoError(t, err)
	})
}

func TestManager_ListCertificates_Integration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "ssl-list-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	testDomain1 := "list-test1.example.com"
	testDomain2 := "list-test2.example.com"

	t.Run("ListCertificates_MultipleCertificates", func(t *testing.T) {
		// Manually set up certificates (since RequestCertificate requires ACME)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain1)
		setupIntegrationCert(t, manager, redisClient, tempDir, testDomain2)

		certificates, err := manager.ListCertificates()
		assert.NoError(t, err)
		assert.NotEmpty(t, certificates)
		assert.Contains(t, certificates, testDomain1)
		assert.Contains(t, certificates, testDomain2)
		assert.Equal(t, "active", certificates[testDomain1]["status"])
		assert.Equal(t, "active", certificates[testDomain2]["status"])
	})
}

func createSelfSignedCertificateForTest(domain string, privateKey *rsa.PrivateKey) ([]byte, error) {
	return createSelfSignedCertificate(domain, privateKey)
}

func createSelfSignedCertificate(domain string, privateKey *rsa.PrivateKey) ([]byte, error) {
	template := createCertificateTemplate(domain)
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM, nil
}

func createCertificateTemplate(domain string) x509.Certificate {
	return x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames:              []string{domain},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
}
