package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/middleware"
)

// MockGeoIPResolver implements services.GeoIPResolver for testing
type MockGeoIPResolver struct {
	IPMap map[string]string
}

func (m *MockGeoIPResolver) LookupCountryISO(ip string) (string, error) {
	if country, ok := m.IPMap[ip]; ok {
		return country, nil
	}
	return "", fmt.Errorf("ip not found")
}

func TestWafMiddleware_IPAccessControl(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Define site config with IP access control
	site := config.SiteConfig{
		ID: "test-waf-site",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Blacklist: []string{"192.168.1.100"},
			Whitelist: []string{"10.0.0.1"},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access Denied",
			},
		},
	}

	// Setup router with WAF middleware
	r := gin.New()
	// Pass nil for repository, redis, and geoIP
	r.Use(middleware.WafMiddleware(site, nil, nil, nil))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	t.Run("Blacklisted IP should be blocked", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100") // Gin's ClientIP() uses this
		req.RemoteAddr = "192.168.1.100:12345"
		
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Access Denied")
	})

	t.Run("Whitelisted IP should be allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})

	t.Run("Normal IP should be allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "8.8.8.8:12345"
		
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})
}

func TestWafMiddleware_GeoIPAccessControl(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup Mock GeoIP
	mockGeoIP := &MockGeoIPResolver{
		IPMap: map[string]string{
			"1.1.1.1": "US",
			"2.2.2.2": "CN",
			"3.3.3.3": "RU",
		},
	}

	// Define site config with GeoIP access control
	site := config.SiteConfig{
		ID: "test-geoip-site",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"RU", "KP"}, // Block Russia and North Korea
				AllowList: []string{},           // Empty AllowList means no whitelist restriction (unless blocked)
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "GeoIP Blocked",
			},
		},
	}

	// Setup router
	r := gin.New()
	r.Use(middleware.WafMiddleware(site, nil, nil, mockGeoIP))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	t.Run("Blocked Country (RU) should be blocked", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "3.3.3.3") 
		req.RemoteAddr = "3.3.3.3:12345"
		
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "GeoIP Blocked")
	})

	t.Run("Allowed Country (US) should be allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.1.1.1")
		req.RemoteAddr = "1.1.1.1:12345"
		
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})

	t.Run("Country in AllowList should be allowed", func(t *testing.T) {
		// Update config to use AllowList
		siteWithAllow := site
		siteWithAllow.Firewall.GeoIPConfig.AllowList = []string{"CN"}
		
		rAllow := gin.New()
		rAllow.Use(middleware.WafMiddleware(siteWithAllow, nil, nil, mockGeoIP))
		rAllow.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "OK") })

		// CN is allowed
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "2.2.2.2")
		req.RemoteAddr = "2.2.2.2:12345"
		w := httptest.NewRecorder()
		rAllow.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// US is NOT in AllowList -> Blocked
		req2, _ := http.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Forwarded-For", "1.1.1.1")
		req2.RemoteAddr = "1.1.1.1:12345"
		w2 := httptest.NewRecorder()
		rAllow.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusForbidden, w2.Code)
	})
}
