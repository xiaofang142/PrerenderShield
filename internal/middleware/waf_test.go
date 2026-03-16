package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
)

// MockGeoIPResolver 模拟 GeoIP 解析器
type MockGeoIPResolver struct {
	countryCode string
	err         error
}

func (m *MockGeoIPResolver) LookupCountryISO(ip string) (string, error) {
	return m.countryCode, m.err
}

// MockWafRepository 模拟 WAF 仓库
type MockWafRepository struct{}

func (m *MockWafRepository) CreateAccessLog(log *interface{}) error {
	return nil
}

func TestWafMiddleware_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: false,
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_Whitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Whitelist: []string{"192.168.1.100"},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_Blacklist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Blacklist: []string{"192.168.1.100"},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Access denied")
}

func TestWafMiddleware_GeoIP_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN", "RU"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "CN",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Country is blocked")
}

func TestWafMiddleware_GeoIP_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN", "RU"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "US",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_GeoIP_AllowList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				AllowList: []string{"US", "CA"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "CN",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Country not in allow list")
}

func TestWafMiddleware_GeoIP_InAllowList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				AllowList: []string{"US", "CA"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "US",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_GeoIP_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		err: assert.AnError,
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// GeoIP 查询出错时应该允许通过
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_GeoIP_EmptyCountryCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 空国家码应该允许通过
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_NilGeoIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// nil GeoIP 时应该允许通过
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_NilWafRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Blacklist: []string{"192.168.1.100"},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// nil WafRepo 时应该仍然能返回 403
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWafMiddleware_NilRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			RateLimitConfig: config.RateLimitConfig{
				Enabled:  true,
				Requests: 100,
				Window:   60,
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// nil Redis 时应该允许通过（速率限制跳过）
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_Whitelist_Before_Blacklist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Whitelist: []string{"192.168.1.100"},
			Blacklist: []string{"192.168.1.100"},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Access denied",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 白名单优先级高于黑名单
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWafMiddleware_Blacklist_Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled:   true,
			Blacklist: []string{"192.168.1.100"},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Forbidden by WAF",
			},
		},
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Forbidden by WAF")
	assert.Contains(t, body, "IP is in blacklist")
}

func TestWafMiddleware_GeoIP_Blocked_Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				BlockList: []string{"CN"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Forbidden by WAF",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "CN",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Forbidden by WAF")
	assert.Contains(t, body, "Country is blocked")
}

func TestWafMiddleware_GeoIP_AllowList_Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	site := config.SiteConfig{
		ID: "site1",
		Firewall: config.FirewallConfig{
			Enabled: true,
			GeoIPConfig: config.GeoIPConfig{
				Enabled:   true,
				AllowList: []string{"US"},
			},
			ActionConfig: config.ActionConfig{
				BlockMessage: "Forbidden by WAF",
			},
		},
	}

	mockGeoIP := &MockGeoIPResolver{
		countryCode: "CN",
	}

	router := gin.New()
	router.Use(WafMiddleware(site, nil, nil, mockGeoIP))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Forbidden by WAF")
	assert.Contains(t, body, "Country not in allow list")
}
