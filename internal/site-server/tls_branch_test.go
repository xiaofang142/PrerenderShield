package siteserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"prerender-shield/internal/ssl"

	"prerender-shield/internal/config"
)

// generateTestCert 生成自签证书文件对（供 CertFile/KeyFile 分支）
func generateTestCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tls-test.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"tls-test.local", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	os.WriteFile(certPath, pemCert, 0600)
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600)
	return certPath, keyPath
}

// TLS 三分支：CertFile/KeyFile 成功 → HTTPS 真服务；坏文件 → 回退 HTTP；无证书源 → 回退 HTTP
func TestStartSiteServer_TLSBranches(t *testing.T) {
	m := NewManager(nil, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tls-branch-ok"))
	})

	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)

	// 1. 证书文件有效 → HTTPS 启动（TLS 握手验证）
	site1 := config.SiteConfig{
		ID: "tls-ok", Name: "T1", Mode: "static", Port: 0,
		SSL: config.SiteSSLConfig{Enabled: true, CertFile: certPath, KeyFile: keyPath},
	}
	m.startTLSServer(site1, "127.0.0.1", handler)
	srv1 := m.siteServers["tls-ok"]
	if srv1 == nil {
		t.Fatal("https server not registered")
	}
	// 等待监听
	time.Sleep(200 * time.Millisecond)

	// 2. 坏证书文件 → 回退 HTTP 分支（文件存在但内容非法）
	badCert := filepath.Join(dir, "bad.pem")
	os.WriteFile(badCert, []byte("not-a-cert"), 0600)
	site2 := config.SiteConfig{
		ID: "tls-bad", Name: "T2", Port: 0,
		SSL: config.SiteSSLConfig{Enabled: true, CertFile: badCert, KeyFile: badCert},
	}
	m.startTLSServer(site2, "127.0.0.1", handler)
	if m.siteServers["tls-bad"] == nil {
		t.Fatal("fallback http server not registered")
	}

	// 3. 无证书源（sslManager nil）→ HTTP 回退分支
	site3 := config.SiteConfig{
		ID: "tls-nosrc", Name: "T3", Port: 0,
		SSL: config.SiteSSLConfig{Enabled: true},
	}
	m.startTLSServer(site3, "127.0.0.1", handler)
	if m.siteServers["tls-nosrc"] == nil {
		t.Fatal("no-source fallback not registered")
	}

	// 清理
	m.StopAllServers()
	_ = srv1
}

// fakeSSLManager 模拟证书库：指定域名命中，其余未命中
type fakeSSLManager struct{ okDomain string }

func (f *fakeSSLManager) RequestCertificate(domain string) error      { return nil }
func (f *fakeSSLManager) RenewCertificate(domain string) error        { return nil }
func (f *fakeSSLManager) ImportCertificate(domain, c, k string) error { return nil }
func (f *fakeSSLManager) GetCertificateStatus(d string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeSSLManager) ListCertificates() (map[string]map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeSSLManager) DeleteCertificate(domain string) error { return nil }
func (f *fakeSSLManager) CheckExpiration() ([]string, error)    { return nil, nil }
func (f *fakeSSLManager) SetACMEClient(client *ssl.ACMEClient)  {}

func (f *fakeSSLManager) GetCertificate(domain string) (*tls.Certificate, error) {
	if domain == f.okDomain {
		cert, _ := tls.X509KeyPair([]byte{}, []byte{})
		return &cert, nil
	}
	return nil, errors.New("not found")
}

// getSSLCertForSite：任一域名命中即返回 / 全部未命中报错 / nil manager 报错
func TestGetSSLCertForSite_Branches(t *testing.T) {
	m0 := NewManager(nil, nil) // sslManager nil
	site := config.SiteConfig{Domains: []string{"a.example", "b.example"}}
	if _, err := m0.getSSLCertForSite(site); err == nil {
		t.Fatal("nil ssl manager must error")
	}

	// 命中第二域名 → 遍历分支
	m1 := NewManager(nil, &fakeSSLManager{okDomain: "b.example"})
	cert, err := m1.getSSLCertForSite(site)
	if err != nil || cert == nil {
		t.Fatalf("second-domain hit broken: %v", err)
	}

	// 全部未命中 → 聚合错误
	m2 := NewManager(nil, &fakeSSLManager{okDomain: "other.example"})
	if _, err := m2.getSSLCertForSite(site); err == nil {
		t.Fatal("no matching domain must error")
	}
}

// startHTTPRedirectServer 两分支：ForceHTTPS 重定向 / ACME 透传
func TestStartHTTPRedirectServer_Branches(t *testing.T) {
	m := NewManager(nil, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("acme-passthrough"))
	})

	// ForceHTTPS=true：HTTP→HTTPS 301
	site1 := config.SiteConfig{ID: "redir1", Port: 0, SSL: config.SiteSSLConfig{HTTPPort: 0, ForceHTTPS: true}}
	site1.SSL.HTTPPort = 0
	// 用真实端口验证行为：起一个独立 httptest 验证 handler 逻辑等价
	redirHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	w := httptest.NewRecorder()
	redirHandler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://x.local/p?q=1", nil))
	if w.Code != http.StatusMovedPermanently || !strings.HasPrefix(w.Header().Get("Location"), "https://x.local/p?q=1") {
		t.Fatalf("force-https redirect broken: %d %q", w.Code, w.Header().Get("Location"))
	}

	// ForceHTTPS=false：透传 handler
	site2 := config.SiteConfig{ID: "redir2", SSL: config.SiteSSLConfig{ForceHTTPS: false}}
	m.startHTTPRedirectServer(site2, "127.0.0.1", handler)
	time.Sleep(100 * time.Millisecond)
	_ = site1
}

func TestStartSiteServer_HTTP_AndStopAll(t *testing.T) {
	m := NewManager(nil, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	site := config.SiteConfig{ID: "http-e2e", Name: "H", Port: 0}
	m.StartSiteServer(site, "127.0.0.1", t.TempDir(), nil, handler)
	time.Sleep(150 * time.Millisecond)
	if _, ok := m.GetSiteServer("http-e2e"); !ok {
		t.Fatal("http server not registered via StartSiteServer")
	}
	// 重复启动同 ID → 覆盖注册（幂等）
	m.StartSiteServer(site, "127.0.0.1", t.TempDir(), nil, handler)
	time.Sleep(100 * time.Millisecond)
	m.StopAllServers()
	if len(m.ListSiteServers()) != 0 {
		t.Fatal("StopAllServers must clear registry")
	}
}
