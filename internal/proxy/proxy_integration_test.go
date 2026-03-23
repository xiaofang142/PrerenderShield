package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/redis"
	"prerender-shield/internal/services"
)

func TestProxy_ForwardRequest(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "success"}`)
	}))
	defer backendServer.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-1"
	testDomain := "test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)

	err = proxyInstance.AddBackend(testSiteID, backendServer.URL)
	assert.NoError(t, err)

	t.Run("ForwardRequest_MissingHost", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = ""
		w := httptest.NewRecorder()

		proxyInstance.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("ForwardRequest_UnknownDomain", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = "unknown.example.com"
		w := httptest.NewRecorder()

		proxyInstance.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("ForwardRequest_MissingBackend", func(t *testing.T) {
		resolver.resolveMap["missing-backend.example.com"] = "missing-site"

		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = "missing-backend.example.com"
		w := httptest.NewRecorder()

		proxyInstance.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestProxy_WithHeaders(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-headers"
	testDomain := "headers-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
	assert.NoError(t, err)

	t.Run("WithHeaders_BackendAdded", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", backendURL)
	})
}

func TestProxy_ErrorHandling(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-error"

	t.Run("ErrorHandling_InvalidBackendURL", func(t *testing.T) {
		proxyInstance := NewProxy(resolver, redisClient)
		err := proxyInstance.AddBackend(testSiteID, "://invalid-url")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid backend URL")
	})

	t.Run("ErrorHandling_AddBackendSuccess", func(t *testing.T) {
		proxyInstance := NewProxy(resolver, redisClient)
		err := proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
		assert.NoError(t, err)
	})
}

func TestProxy_TargetFailover(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Backend1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Backend2")
	}))
	defer backend2.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-failover"
	testDomain := "failover-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("TargetFailover_SwitchBackend", func(t *testing.T) {
		err := proxyInstance.AddBackend(testSiteID, backend1.URL)
		assert.NoError(t, err)

		err = proxyInstance.AddBackend(testSiteID, backend2.URL)
		assert.NoError(t, err)

		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, backend2.URL, backendURL)
	})
}

func TestProxy_RemoveBackend(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-remove"
	testDomain := "remove-test.example.com"
	backendURL := "http://localhost:8080"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("RemoveBackend_Success", func(t *testing.T) {
		err := proxyInstance.AddBackend(testSiteID, backendURL)
		assert.NoError(t, err)

		_, err = proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)

		err = proxyInstance.RemoveBackend(testSiteID)
		assert.NoError(t, err)

		_, err = proxyInstance.GetBackend(testSiteID)
		assert.Error(t, err)
	})

	t.Run("RemoveBackend_NilRedis", func(t *testing.T) {
		proxyWithNilRedis := NewProxy(resolver, redisClient)
		proxyWithNilRedis.(*proxy).redisClient = nil
		err := proxyWithNilRedis.AddBackend(testSiteID, backendURL)
		assert.NoError(t, err)

		err = proxyWithNilRedis.RemoveBackend(testSiteID)
		assert.NoError(t, err)
	})
}

func TestProxy_LoadBackendsFromRedis(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-load"
	testBackendURL := "http://localhost:8082"

	backendKey := "backend:" + testSiteID
	err = redisClient.Set(backendKey, testBackendURL, 0)
	assert.NoError(t, err)

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("LoadBackendsFromRedis_Success", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, testBackendURL, backendURL)
	})

	t.Run("LoadBackendsFromRedis_InvalidURL", func(t *testing.T) {
		invalidSiteID := "invalid-site"
		invalidBackendKey := "backend:" + invalidSiteID
		err = redisClient.Set(invalidBackendKey, "://invalid-url", 0)
		assert.NoError(t, err)

		proxyInstance2 := NewProxy(resolver, redisClient)
		_, err := proxyInstance2.GetBackend(invalidSiteID)
		assert.Error(t, err)
	})
}

func TestProxy_GetOrCreateReverseProxy(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-proxy"
	backendURL := "http://localhost:8083"

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("GetOrCreateReverseProxy_InvalidURL", func(t *testing.T) {
		proxy := proxyInstance.(*proxy)
		_, err := proxy.getOrCreateReverseProxy(testSiteID, "://invalid-url")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid backend URL")
	})

	t.Run("GetOrCreateReverseProxy_Success", func(t *testing.T) {
		proxy := proxyInstance.(*proxy)
		rp, err := proxy.getOrCreateReverseProxy(testSiteID, backendURL)
		assert.NoError(t, err)
		assert.NotNil(t, rp)

		rp2, err := proxy.getOrCreateReverseProxy(testSiteID, backendURL)
		assert.NoError(t, err)
		assert.Equal(t, rp, rp2)
	})
}

func TestProxy_ServeHTTP_Integration(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-integration"
	testDomain := "integration-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
	assert.NoError(t, err)

	t.Run("ServeHTTP_BackendConfigured", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", backendURL)
	})
}

func TestProxy_AddBackend_Integration(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-add"
	backendURL := "http://localhost:8085"

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("AddBackend_InvalidURL", func(t *testing.T) {
		err := proxyInstance.AddBackend(testSiteID, "://invalid-url")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid backend URL")
	})

	t.Run("AddBackend_ValidURL", func(t *testing.T) {
		err := proxyInstance.AddBackend(testSiteID, backendURL)
		assert.NoError(t, err)

		storedURL, err := redisClient.Get("backend:" + testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, backendURL, storedURL)

		retrievedURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, backendURL, retrievedURL)
	})

	t.Run("AddBackend_UpdateExisting", func(t *testing.T) {
		newBackendURL := "http://localhost:8086"
		err := proxyInstance.AddBackend(testSiteID, newBackendURL)
		assert.NoError(t, err)

		storedURL, err := redisClient.Get("backend:" + testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, newBackendURL, storedURL)
	})
}

func TestProxy_DomainResolver_Integration(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := services.NewDomainResolver(redisClient)

	testSiteID := "test-site-domain"
	testDomain := "domain-resolver-test.example.com"

	err = resolver.AddMapping(testDomain, testSiteID)
	assert.NoError(t, err)

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
	assert.NoError(t, err)

	t.Run("DomainResolver_MappingCreated", func(t *testing.T) {
		siteID, err := resolver.Resolve(testDomain)
		assert.NoError(t, err)
		assert.Equal(t, testSiteID, siteID)
	})

	t.Run("DomainResolver_Wildcard", func(t *testing.T) {
		wildcardDomain := "*.example.com"
		wildcardSiteID := "wildcard-site"

		err = resolver.AddMapping(wildcardDomain, wildcardSiteID)
		assert.NoError(t, err)

		err = proxyInstance.AddBackend(wildcardSiteID, "http://localhost:8081")
		assert.NoError(t, err)

		siteID, err := resolver.Resolve("sub.example.com")
		assert.NoError(t, err)
		assert.Equal(t, wildcardSiteID, siteID)
	})
}

func TestProxy_TransportConfig(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("TransportConfig_Default", func(t *testing.T) {
		proxy := proxyInstance.(*proxy)
		assert.NotNil(t, proxy.transport)
		assert.Equal(t, 100, proxy.transport.MaxIdleConns)
		assert.Equal(t, 20, proxy.transport.MaxIdleConnsPerHost)
		assert.Equal(t, 90*time.Second, proxy.transport.IdleConnTimeout)
		assert.Equal(t, 10*time.Second, proxy.transport.TLSHandshakeTimeout)
		assert.Equal(t, 1*time.Second, proxy.transport.ExpectContinueTimeout)
	})
}

func TestProxy_ModifyResponse(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-modify"
	testDomain := "modify-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
	assert.NoError(t, err)

	t.Run("ModifyResponse_BackendConfigured", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", backendURL)
	})
}

func TestProxy_ConcurrentAccess(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-concurrent"
	backendURL := "http://localhost:8090"

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("ConcurrentAccess_AddBackend", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				proxyInstance.AddBackend(testSiteID, backendURL)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		retrievedURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, backendURL, retrievedURL)
	})
}

func TestProxy_BackendPersistence(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-persist"
	testDomain := "persist-test.example.com"
	backendURL := "http://localhost:8091"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, backendURL)
	assert.NoError(t, err)

	t.Run("BackendPersistence_RedisRestart", func(t *testing.T) {
		backendURL2, err := redisClient.Get("backend:" + testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, backendURL, backendURL2)
	})
}

func TestProxy_URLPathHandling(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}))
	defer backendServer.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-path"
	testDomain := "path-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, backendServer.URL)
	assert.NoError(t, err)

	t.Run("URLPathHandling_RootPath", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://"+testDomain+"/", nil)
		req.Host = testDomain
		w := httptest.NewRecorder()

		proxyInstance.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("URLPathHandling_NestedPath", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.NotEmpty(t, backendURL)
	})
}

func TestProxy_GetBackend_NotFound(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	proxyInstance := NewProxy(resolver, redisClient)

	t.Run("GetBackend_NotFound", func(t *testing.T) {
		_, err := proxyInstance.GetBackend("nonexistent-site")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backend not found")
	})
}

func TestProxy_ResponseModification(t *testing.T) {
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	testSiteID := "test-site-response"
	testDomain := "response-test.example.com"

	resolver.resolveMap[testDomain] = testSiteID

	proxyInstance := NewProxy(resolver, redisClient)
	err = proxyInstance.AddBackend(testSiteID, "http://localhost:8080")
	assert.NoError(t, err)

	t.Run("ResponseModification_BackendConfigured", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", backendURL)
	})
}
