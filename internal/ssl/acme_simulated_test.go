package ssl

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeACMEDirectory 模拟 ACME 目录服务器（directory/newNonce/newAccount 端点），
// 让 lego 客户端本地完成账户注册——不发任何外网请求。
type fakeACMEDirectory struct {
	srv      *httptest.Server
	mu       sync.Mutex
	nonce    int
	accounts int
}

func (f *fakeACMEDirectory) nextNonce() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nonce++
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("nonce-%d", f.nonce)))
}

func (f *fakeACMEDirectory) handler() http.Handler {
	mux := http.NewServeMux()
	// ACME 目录文档
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		base := f.srv.URL
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"newNonce":   base + "/new-nonce",
			"newAccount": base + "/new-account",
			"newOrder":   base + "/new-order",
			"revokeCert": base + "/revoke-cert",
			"keyChange":  base + "/key-change",
		})
	})
	// nonce 端点（HEAD/GET 两种）
	nonceHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", f.nextNonce())
		w.Header().Set("Link", fmt.Sprintf(`<%s/directory>;rel="index"`, f.srv.URL))
		w.WriteHeader(http.StatusOK)
	}
	mux.HandleFunc("/new-nonce", nonceHandler)
	// newAccount：接受任意 JWS，返回有效账户
	mux.HandleFunc("/new-account", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.accounts++
		f.mu.Unlock()
		w.Header().Set("Replay-Nonce", f.nextNonce())
		w.Header().Set("Location", f.srv.URL+"/acct/1")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "valid",
			"contact": []string{"mailto:ops@example.com"},
		})
	})
	return mux
}

func startFakeACME(t *testing.T) *fakeACMEDirectory {
	t.Helper()
	f := &fakeACMEDirectory{}
	f.srv = httptest.NewTLSServer(f.handler()) // lego 强制 HTTPS 目录 → TLS + 生产 INSECURE 插桩
	t.Cleanup(f.srv.Close)
	t.Setenv("ACME_TLS_INSECURE", "1")
	return f
}

// NewACMEClient 经模拟目录服务器完成账户注册（lego 客户端真实协议交互）
func TestNewACMEClient_WithSimulatedDirectory(t *testing.T) {
	fake := startFakeACME(t)
	t.Setenv("ACME_DIRECTORY_URL", fake.srv.URL+"/directory")

	c, err := NewACMEClient(ACMEConfig{Email: "ops@example.com", CertDir: t.TempDir(), HTTPPort: 0})
	if err != nil {
		t.Fatalf("NewACMEClient with simulated directory: %v", err)
	}
	if c.account == nil || c.account.Registration == nil {
		t.Fatal("account registration missing")
	}
	if c.account.Registration.URI == "" {
		t.Fatal("registration URI missing")
	}
	if fake.accounts != 1 {
		t.Fatalf("newAccount calls=%d want 1", fake.accounts)
	}
}

// GenerateSecret 类辅助确认（account key 为 EC P-256）
func TestNewACMEClient_AccountKeyType(t *testing.T) {
	fake := startFakeACME(t)
	t.Setenv("ACME_DIRECTORY_URL", fake.srv.URL+"/directory")
	c, err := NewACMEClient(ACMEConfig{Email: "x@example.com", CertDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	// lego 默认账户密钥为 ECDSA P-256；断言可签名（JWS 依赖）
	switch k := c.account.key.(type) {
	case *ecdsa.PrivateKey:
		if _, err := ecdsa.SignASN1(rand.Reader, k, []byte("payload")); err != nil {
			t.Fatalf("sign failed: %v", err)
		}
	default:
		t.Fatalf("unexpected account key type: %T", k)
	}
}
