package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/config"
)

func setupSEOController(cfg *config.Config) (*SEOController, *gin.Engine) {
	controller := NewSEOController(cfg)
	router := ginNewRouter()
	router.POST("/seo/sitemap/generate", controller.GenerateSitemap)
	router.GET("/seo/sitemap", controller.GetSitemap)
	router.POST("/seo/robots/generate", controller.GenerateRobotsTxt)
	router.GET("/seo/robots", controller.GetRobotsTxt)
	router.GET("/seo/config", controller.GetSEOConfig)
	router.PUT("/seo/config", controller.UpdateSEOConfig)
	return controller, router
}

// TestSEOController_GenerateSitemap_Disabled 站点地图生成关闭 → 400
func TestSEOController_GenerateSitemap_Disabled(t *testing.T) {
	cfg := &config.Config{}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/sitemap/generate", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "disabled")
}

// TestSEOController_GenerateSitemap_Success 存在静态站点目录时生成成功
func TestSEOController_GenerateSitemap_Success(t *testing.T) {
	staticRoot := t.TempDir()
	siteDir := filepath.Join(staticRoot, "seo-site")
	require.NoError(t, os.MkdirAll(siteDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<html></html>"), 0o644))
	defer os.RemoveAll(filepath.Join(staticRoot, "seo-site", "sitemap.xml"))

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "seo-site", Name: "SEO", Domains: []string{"seo.example"}}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	cfg.SEO.Sitemap = config.SitemapSEOConfig{Enabled: true, BaseURL: "http://seo.example"}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/sitemap/generate", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Sites   int      `json:"sites"`
			Outputs []string `json:"outputs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Sites)
	assert.NotEmpty(t, resp.Data.Outputs)
}

// TestSEOController_GenerateSitemap_NoDirs 站点目录全部缺失 → 500
func TestSEOController_GenerateSitemap_NoDirs(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ghost-site", Domains: []string{"ghost.example"}}},
		Dirs:  config.DirsConfig{StaticDir: filepath.Join(t.TempDir(), "not-exist")},
	}
	cfg.SEO.Sitemap = config.SitemapSEOConfig{Enabled: true}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/sitemap/generate", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSEOController_GetSitemap 从静态目录生成 sitemap XML
func TestSEOController_GetSitemap(t *testing.T) {
	staticRoot := t.TempDir()
	defaultDir := filepath.Join(staticRoot, "default")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "page.html"), []byte("<html></html>"), 0o644))

	cfg := &config.Config{
		Dirs: config.DirsConfig{StaticDir: staticRoot},
	}
	cfg.SEO.Sitemap = config.SitemapSEOConfig{Enabled: true, BaseURL: "http://seo.example"}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/seo/sitemap", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/xml", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "<urlset")
	assert.Contains(t, w.Body.String(), "page.html")
}

// TestSEOController_GenerateRobotsTxt_Disabled robots 生成关闭 → 400
func TestSEOController_GenerateRobotsTxt_Disabled(t *testing.T) {
	cfg := &config.Config{}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/robots/generate", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSEOController_GenerateRobotsTxt_Success 按站点生成 robots.txt
func TestSEOController_GenerateRobotsTxt_Success(t *testing.T) {
	staticRoot := t.TempDir()
	siteDir := filepath.Join(staticRoot, "robots-site")
	require.NoError(t, os.MkdirAll(siteDir, 0o755))

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "robots-site", Domains: []string{"robots.example"}}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	cfg.SEO.Robots = config.RobotsSEOConfig{
		Enabled:    true,
		SitemapURL: "http://robots.example/sitemap.xml",
		Rules: []config.RobotsRuleSEO{
			{UserAgent: "*", Disallow: []string{"/admin"}},
		},
	}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/robots/generate", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "output_paths")
}

// TestSEOController_GenerateRobotsTxt_NoDirs 目录缺失 → 500
func TestSEOController_GenerateRobotsTxt_NoDirs(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ghost-robots"}},
		Dirs:  config.DirsConfig{StaticDir: filepath.Join(t.TempDir(), "nope")},
	}
	cfg.SEO.Robots = config.RobotsSEOConfig{Enabled: true}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seo/robots/generate", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSEOController_GetRobotsTxt 按配置渲染 robots.txt 内容
func TestSEOController_GetRobotsTxt(t *testing.T) {
	cfg := &config.Config{}
	cfg.SEO.Robots = config.RobotsSEOConfig{
		Enabled:    true,
		SitemapURL: "http://x.example/sitemap.xml",
		Rules: []config.RobotsRuleSEO{
			{UserAgent: "Baiduspider", Allow: []string{"/"}, Disallow: []string{"/private"}, CrawlDelay: 2},
		},
	}
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/seo/robots", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	body := w.Body.String()
	assert.Contains(t, body, "User-agent: Baiduspider")
	assert.Contains(t, body, "Disallow: /private")
	assert.Contains(t, body, "Crawl-delay: 2")
	assert.Contains(t, body, "Sitemap: http://x.example/sitemap.xml")
}

// TestSEOController_GetSEOConfig 返回 SEO 配置段
func TestSEOController_GetSEOConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.SEO.Sitemap.BaseURL = "http://cfg.example"
	_, router := setupSEOController(cfg)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/seo/config", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http://cfg.example")
}

// TestSEOController_UpdateSEOConfig_InvalidBody 非法请求体 → 400
func TestSEOController_UpdateSEOConfig_InvalidBody(t *testing.T) {
	_, router := setupSEOController(&config.Config{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/seo/config", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSEOController_UpdateSEOConfig_Success 默认全局配置无 configPath，SaveConfig 直接成功
func TestSEOController_UpdateSEOConfig_Success(t *testing.T) {
	config.ResetInstance()
	defer config.ResetInstance()

	_, router := setupSEOController(&config.Config{})

	body := `{"sitemap":{"enabled":true,"base_url":"http://updated.example"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/seo/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "updated successfully")
}
