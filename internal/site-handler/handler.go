package sitehandler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/firewall/detectors"
	"prerender-shield/internal/i18n"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/middleware"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/botverify"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/services"
)

// Handler 站点处理器，负责处理站点的HTTP请求
type Handler struct {
	prerenderManager *prerender.EngineManager
	wafRepo          *repository.WafRepository
	redisClient      *redis.Client
	geoIP            services.GeoIPResolver
	firewallManager  *firewall.EngineManager
	logWriter        *middleware.WafLogWriter
	monitor          *monitoring.Monitor

	// refreshInflight 软过期异步重渲的去重表（key -> 到期时间戳，进程内去重）
	refreshInflight sync.Map

	// botVerifier 爬虫真实性验证器（懒加载进程内单例；nil 时 bot_verify 未启用）
	botVerifier     *botverify.Verifier
	botVerifierOnce sync.Once
}

// NewHandler 创建站点处理器实例
func NewHandler(prerenderManager *prerender.EngineManager, wafRepo *repository.WafRepository, redisClient *redis.Client, geoIP services.GeoIPResolver, firewallManager *firewall.EngineManager, logWriter *middleware.WafLogWriter, monitor *monitoring.Monitor) *Handler {
	return &Handler{
		prerenderManager: prerenderManager,
		wafRepo:          wafRepo,
		redisClient:      redisClient,
		geoIP:            geoIP,
		firewallManager:  firewallManager,
		logWriter:        logWriter,
		monitor:          monitor,
	}
}

// getLanguageFromRequest 从请求中获取语言偏好
func getLanguageFromRequest(c *gin.Context) string {
	// 从Accept-Language头获取语言
	acceptLanguage := c.GetHeader("Accept-Language")
	if acceptLanguage != "" {
		// 简单处理，获取第一个语言标签
		parts := strings.Split(acceptLanguage, ",")
		if len(parts) > 0 {
			lang := strings.Split(parts[0], ";")[0]
			// 只取语言代码部分（如zh-CN -> zh）
			langParts := strings.Split(lang, "-")
			return langParts[0]
		}
	}
	// 默认返回英文
	return "en"
}

// ---- 预渲染决策辅助（2026-08 升级：对标 Prerender.io/Rendertron 缓存与名单体系） ----

// compilePrerenderFilters 编译站点渲染 URL 名单；非法正则跳过并记日志（保存入口已强校验）
func compilePrerenderFilters(site config.SiteConfig) (include, exclude []*regexp.Regexp) {
	for _, p := range site.Prerender.IncludePatterns {
		if re, err := regexp.Compile(p); err == nil {
			include = append(include, re)
		} else {
			logging.DefaultLogger.Error("site %s include_pattern 非法已忽略 %q: %v", site.ID, p, err)
		}
	}
	for _, p := range site.Prerender.ExcludePatterns {
		if re, err := regexp.Compile(p); err == nil {
			exclude = append(exclude, re)
		} else {
			logging.DefaultLogger.Error("site %s exclude_pattern 非法已忽略 %q: %v", site.ID, p, err)
		}
	}
	return include, exclude
}

// shouldPrerenderURL 判定 URI 是否在渲染范围内：exclude 优先；include 空=全放行
func shouldPrerenderURL(uri string, include, exclude []*regexp.Regexp) bool {
	for _, re := range exclude {
		if re.MatchString(uri) {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, re := range include {
		if re.MatchString(uri) {
			return true
		}
	}
	return false
}

// prerenderPolicyFor 解析分类策略（默认：search/social/generic=render，ai=cache_only 防渲染洪水）
var defaultCategoryPolicies = map[string]string{
	prerender.CatSearch:  "render",
	prerender.CatSocial:  "render",
	prerender.CatAI:      "cache_only",
	prerender.CatGeneric: "render",
}

const (
	policyRender      = "render"
	policyCacheOnly   = "cache_only"
	policyPassthrough = "passthrough"
)

// siteBaseURL 站点公开基址，规则与 sitemap 生成一致（SSL.Enabled 决定 scheme）
func siteBaseURL(site config.SiteConfig) string {
	if len(site.Domains) == 0 {
		return ""
	}
	scheme := "https"
	if !site.SSL.Enabled {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, site.Domains[0])
}

// prerenderPolicyFor 解析分类策略：站点配置优先，未配置的类用默认表
func prerenderPolicyFor(site config.SiteConfig, category string) string {
	if category == "" {
		return policyRender
	}
	if p, ok := site.Prerender.CategoryPolicy[category]; ok && p != "" {
		return p
	}
	if p, ok := defaultCategoryPolicies[category]; ok {
		return p
	}
	return policyRender
}

// staleEnabled 软过期降级开关（nil 视为开启）
func staleEnabled(site config.SiteConfig) bool {
	return site.Prerender.StaleWhileRevalidate == nil || *site.Prerender.StaleWhileRevalidate
}

// serveCrawlerPage 统一爬虫响应出口：真实状态码 + 命中标记头 + 监控同步
// serveCrawlerPage 爬虫响应汇聚点（fresh/stale/miss 三路径统一出口）。
// 条件请求支持：200 响应带弱 ETag（W/"sha256前16hex"，gzip 变体安全）与 Last-Modified；
// 命中 If-None-Match / If-Modified-Since 返回 304 无 body（调研：Googlebot 实际发送并接受 304）。
// gzip：200 且 >1KB 且客户端接受时压缩（stdlib，零新依赖）。返回实际响应状态码供日志如实记录。
func serveCrawlerPage(c *gin.Context, mon *monitoring.Monitor, status int, html []byte, hit string, createdAt int64, renderTime float64) int {
	c.Header("X-Prerender-Hit", hit)
	c.Header("Vary", "Accept-Encoding")
	served := status
	if status == http.StatusOK {
		etag := weakETag(html)
		c.Header("ETag", etag)
		if createdAt > 0 {
			c.Header("Last-Modified", time.Unix(createdAt, 0).UTC().Format(http.TimeFormat))
		}
		if ifNoneMatchHit(c.Request.Header.Get("If-None-Match"), etag) {
			served = http.StatusNotModified
		} else if ims := c.Request.Header.Get("If-Modified-Since"); ims != "" && createdAt > 0 {
			if t, err := time.Parse(http.TimeFormat, ims); err == nil {
				// 秒级粒度比较：Last-Modified ≤ IMS 即未修改
				if !time.Unix(createdAt, 0).Truncate(time.Second).After(t) {
					served = http.StatusNotModified
				}
			}
		}
	}
	if served == http.StatusNotModified {
		c.AbortWithStatus(http.StatusNotModified)
	} else if status == http.StatusOK && len(html) > gzipMinSize && strings.Contains(strings.ToLower(c.GetHeader("Accept-Encoding")), "gzip") {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(html)
		_ = zw.Close()
		c.Header("Content-Encoding", "gzip")
		c.Data(status, "text/html; charset=utf-8", buf.Bytes())
	} else {
		c.Data(status, "text/html; charset=utf-8", html)
	}

	if mon != nil {
		mon.RecordCrawlerRequest()
		mon.RecordRequest(c.Request.Method, c.Request.URL.Path, served, time.Duration(renderTime*float64(time.Second)))
	}
	c.Abort()
	return served
}

// gzipMinSize 小于该值的响应不压缩（压缩头部开销超过收益）
const gzipMinSize = 1024

// weakETag 内容哈希弱校验器
func weakETag(b []byte) string {
	sum := sha256.Sum256(b)
	return `W/"` + hex.EncodeToString(sum[:8]) + `"`
}

// ifNoneMatchHit 解析 If-None-Match 列表（容忍 W/ 前缀与 *）
func ifNoneMatchHit(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	raw := strings.TrimPrefix(etag, "W/")
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		if p == "*" || strings.TrimPrefix(p, "W/") == raw {
			return true
		}
	}
	return false
}

type crawlerPageMeta struct {
	Site       string
	Route      string
	UA         string
	HitCache   string // fresh/stale/miss
	Status     int
	CacheTTL   int
	RenderTime float64
	Quality    string // ""/"thin"
	Verified   string // ""/"verified"/"unverified"（bot_verify 开启时的 rDNS 验证结果）
}

// recordCrawlerLog 附加字段版日志记录
func recordCrawlerLog(crawlerLogManager *logging.CrawlerLogManager, c *gin.Context, m crawlerPageMeta) {
	if crawlerLogManager == nil {
		return
	}
	crawlerLogManager.RecordCrawlerLog(logging.CrawlerLog{
		Site:       m.Site,
		IP:         logging.GetClientIP(c.Request),
		Time:       time.Now(),
		HitCache:   m.HitCache == "fresh" || m.HitCache == "stale",
		Route:      m.Route,
		UA:         m.UA,
		Status:     m.Status,
		Method:     c.Request.Method,
		CacheTTL:   m.CacheTTL,
		RenderTime: float64(int(m.RenderTime*100)) / 100,
		Quality:    m.Quality,
		Verified:   m.Verified,
	})
}

// botVerifyFor 爬虫真实性验证。
// mode=log（默认）：零阻塞——缓存命中即时取结果，未缓存时本次不打标、异步验证回填。
// mode=block：同步验证（缓存+singleflight 兜底成本），
// 仅当结果为确定性 unverified（自称搜索爬虫但 PTR 缺失/解析到非官方域）时调用方应拒绝；
// unknown（DNS 传输故障/超时）一律放行——宁漏勿杀（tasks/disputes-c10.md 裁决）。
// 返回值：""（未验证/未启用）/ verified / unverified。
func (h *Handler) botVerifyFor(site config.SiteConfig, category, userAgent, ip string) string {
	if !site.Firewall.BotVerify.Enabled || category != prerender.CatSearch {
		return ""
	}
	h.botVerifierOnce.Do(func() {
		if h.botVerifier == nil {
			h.botVerifier = botverify.NewVerifier("")
		}
	})
	if site.Firewall.BotVerify.Mode == "block" {
		return h.botVerifier.Verify(ip)
	}
	if r := h.botVerifier.Peek(ip); r != botverify.ResultUnknown {
		return r
	}
	h.botVerifier.VerifyAsync(ip)
	return ""
}

// renderErrorHTML 渲染失败时的极简 HTML 兜底页（爬虫可读，禁止收录）
func renderErrorHTML(lang, detail string) []byte {
	msg := i18n.T(lang, "error.render_timeout")
	html := "<!DOCTYPE html><html><head><meta charset=\"utf-8\">" +
		"<meta name=\"robots\" content=\"noindex\"><title>Service Unavailable</title></head><body><h1>" +
		msg + "</h1>" + detail + "</body></html>"
	return []byte(html)
}

// triggerAsyncRefresh 软过期命中后的后台重渲（窗口内去重，失败静默）
func (h *Handler) triggerAsyncRefresh(eng prerender.Engine, req prerender.RenderRequest) {
	const window = 2 * time.Minute
	now := time.Now().Unix()
	if v, loaded := h.refreshInflight.LoadOrStore(req.URL, now); loaded {
		if exp, ok := v.(int64); ok && now < exp {
			return
		}
		h.refreshInflight.Store(req.URL, now+int64(window/time.Second))
	} else {
		h.refreshInflight.Store(req.URL, now+int64(window/time.Second))
	}
	go func() {
		defer func() { _ = recover() }()
		time.Sleep(200 * time.Millisecond) // 让出请求路径
		_, err := eng.RenderAndCache(req)
		if err != nil {
			logging.DefaultLogger.Info("async refresh failed for %s: %v", req.URL, err)
		}
	}()
}

// serveCrawlerFromCacheOrRender 爬虫响应主编排（G3/G5/H-A 降级供数三件套）：
//  1. 缓存 fresh 命中 → 立即回真实状态码
//  2. 缓存 stale 命中（软过期）→ 立即回旧值 + 后台异步重渲（SWR，可站点级关闭）
//  3. miss → per-site 并发预算内渲染；cache_only 类到此为止透传上游
//  4. 渲染失败 → 有任意旧值回旧值；全无返回错误由上层出 503 noindex HTML
func (h *Handler) serveCrawlerFromCacheOrRender(
	site config.SiteConfig, eng prerender.Engine, c *gin.Context,
	fullURL, userAgent string, startTime time.Time, cacheOnly bool,
	crawlerLogManager *logging.CrawlerLogManager, mon *monitoring.Monitor,
) (bool, error) {
	siteID := site.ID
	path := c.Request.URL.Path
	effectiveTTL := site.EffectiveCacheTTL(fullURL)
	meta := crawlerPageMeta{Site: siteID, Route: path, UA: userAgent, CacheTTL: effectiveTTL}
	if v := h.botVerifyFor(site, prerender.ClassifyUserAgent(userAgent), userAgent, logging.GetClientIP(c.Request)); v != "" {
		meta.Verified = v
		// block 模式：确认伪造（自称搜索爬虫但 rDNS 确定性排除）→ 403；unknown/verified 放行
		if v == botverify.ResultUnverified && site.Firewall.BotVerify.Mode == "block" {
			recordCrawlerLog(crawlerLogManager, c, crawlerPageMeta{
				Site: siteID, Route: c.Request.URL.Path, UA: userAgent,
				HitCache: "blocked", Status: http.StatusForbidden, Verified: v,
			})
			if mon != nil {
				mon.RecordCrawlerRequest()
			}
			c.Data(http.StatusForbidden, "text/plain; charset=utf-8", []byte("Forbidden: crawler identity verification failed"))
			c.Abort()
			return true, nil
		}
	}

	if env, ok := eng.GetCachedPage(siteID, fullURL, userAgent); ok {
		if env.Fresh(time.Now()) {
			meta.HitCache = "fresh"
			meta.RenderTime = time.Since(startTime).Seconds()
			meta.Status = serveCrawlerPage(c, mon, env.Status, []byte(env.HTML), "fresh", env.CreatedAt, meta.RenderTime)
			recordCrawlerLog(crawlerLogManager, c, meta)
			return true, nil
		}
		// 软过期：立即回旧值 + 异步重渲（降级供数：宁给旧 HTML 不给失败）
		if staleEnabled(site) {
			meta.HitCache = "stale"
			meta.RenderTime = time.Since(startTime).Seconds()
			meta.Status = serveCrawlerPage(c, mon, env.Status, []byte(env.HTML), "stale", env.CreatedAt, meta.RenderTime)
			recordCrawlerLog(crawlerLogManager, c, meta)
			h.triggerAsyncRefresh(eng, prerender.RenderRequest{
				SiteID:    siteID,
				URL:       fullURL,
				Opts:      prerender.RenderOptions{Timeout: time.Duration(site.Prerender.Timeout) * time.Second, CacheTTL: effectiveTTL},
				UserAgent: userAgent,
			})
			return true, nil
		}
		// SWR 关闭：过期值仅作渲染失败的兜底，不直接供数
		req := prerender.RenderRequest{
			SiteID:    siteID,
			URL:       fullURL,
			Opts:      prerender.RenderOptions{Timeout: time.Duration(site.Prerender.Timeout) * time.Second, CacheTTL: effectiveTTL, MaxConcurrency: site.Prerender.MaxConcurrency},
			UserAgent: userAgent,
		}
		res, err := eng.RenderAndCache(req)
		if err != nil || !res.Success {
			if env.HTML != "" {
				meta.HitCache = "stale"
				meta.Status = serveCrawlerPage(c, mon, env.Status, []byte(env.HTML), "stale", env.CreatedAt, time.Since(startTime).Seconds())
				recordCrawlerLog(crawlerLogManager, c, meta)
				return true, nil
			}
			return false, fmt.Errorf("render failed: no fallback available")
		}
		meta.HitCache = "miss"
		meta.Status = res.Status
		if res.Thin {
			meta.Quality = "thin"
		}
		meta.RenderTime = time.Since(startTime).Seconds()
		meta.Status = serveCrawlerPage(c, mon, res.Status, []byte(res.HTML), "miss", time.Now().Unix(), meta.RenderTime)
		recordCrawlerLog(crawlerLogManager, c, meta)
		return true, nil
	}

	// 完全无缓存
	if cacheOnly {
		// cache_only 类（默认 AI 爬虫）：无缓存不现场渲染，透传上游
		c.Next()
		return true, nil
	}

	req := prerender.RenderRequest{
		SiteID:    siteID,
		URL:       fullURL,
		Opts:      prerender.RenderOptions{Timeout: time.Duration(site.Prerender.Timeout) * time.Second, CacheTTL: effectiveTTL, MaxConcurrency: site.Prerender.MaxConcurrency},
		UserAgent: userAgent,
	}
	res, err := eng.RenderAndCache(req)
	if err != nil {
		if errors.Is(err, prerender.ErrSiteBusy) {
			// 站点并发预算耗尽：透传上游（爬虫拿到真实页面而非失败）
			c.Next()
			return true, nil
		}
		return false, err
	}
	if !res.Success {
		return false, fmt.Errorf("render not successful: %s", res.Error)
	}
	meta.HitCache = "miss"
	meta.Status = res.Status
	if res.Thin {
		meta.Quality = "thin"
	}
	meta.RenderTime = time.Since(startTime).Seconds()
	meta.Status = serveCrawlerPage(c, mon, res.Status, []byte(res.HTML), "miss", time.Now().Unix(), meta.RenderTime)
	recordCrawlerLog(crawlerLogManager, c, meta)
	return true, nil
}

// CreateSiteHandler 创建基于站点配置的HTTP处理器
// 根据站点配置创建对应的HTTP处理器，支持proxy、static和redirect三种模式
//
// 参数:
//
//	site: 站点配置，包含站点的基本信息、运行模式、路由规则等
//	crawlerLogManager: 爬虫日志管理器，用于记录爬虫访问日志
//	monitor: 监控管理器，用于记录请求指标
//	staticDir: 静态文件目录，用于static模式下的文件服务
//
// 返回值:
//
//	http.Handler: 创建的HTTP处理器，可直接用于HTTP服务器
//
// 示例:
//
//	httpHandler := handler.CreateSiteHandler(siteConfig, crawlerLogManager, visitLogManager, monitor, "/static")
//	http.ListenAndServe(":8080", httpHandler)
func (h *Handler) CreateSiteHandler(site config.SiteConfig, crawlerLogManager *logging.CrawlerLogManager, visitLogManager *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler {
	// 创建站点级别的Gin路由器
	siteRouter := gin.Default()

	// 创建 WAF 防火墙引擎
	var siteWafEngine *firewall.Engine
	if h.firewallManager != nil && site.Firewall.Enabled {
		geoIPCfg := site.Firewall.GeoIPConfig
		rateLimitCfg := site.Firewall.RateLimitConfig

		// 构建 CC 防护配置
		var ccCfg *detectors.CCProtectionConfig
		if site.Firewall.CCProtection.Enabled {
			rules := make([]detectors.CCProtectionRule, len(site.Firewall.CCProtection.Rules))
			for i, r := range site.Firewall.CCProtection.Rules {
				rules[i] = detectors.CCProtectionRule{
					Name:       r.Name,
					Path:       r.Path,
					Method:     r.Method,
					Dimensions: r.Dimensions,
					Requests:   r.Requests,
					Window:     r.Window,
					BanTime:    r.BanTime,
					Enabled:    r.Enabled,
				}
			}
			ccCfg = &detectors.CCProtectionConfig{
				Enabled: site.Firewall.CCProtection.Enabled,
				Rules:   rules,
			}
		}

		// 构建威胁情报配置
		var tiCfg *detectors.ThreatIntelConfig
		if site.Firewall.ThreatIntel.Enabled {
			tiCfg = &detectors.ThreatIntelConfig{
				Enabled:   true,
				GlobalKey: site.Firewall.ThreatIntel.GlobalKey,
			}
			if tiCfg.GlobalKey == "" {
				tiCfg.GlobalKey = "threatintel:global:blacklist"
			}
		}

		wafConfig := firewall.Config{
			RulesPath:          site.Firewall.RulesPath,
			ActionConfig:       firewall.ActionConfig{DefaultAction: site.Firewall.ActionConfig.DefaultAction, BlockMessage: site.Firewall.ActionConfig.BlockMessage},
			GeoIPConfig:        &geoIPCfg,
			RateLimitConfig:    &rateLimitCfg,
			CCProtectionConfig: ccCfg,
			ThreatIntelConfig:  tiCfg,
			RedisClient:        h.redisClient.GetRawClient(),
			FailStrategy:       firewall.FailClosed,
		}
		engine, err := firewall.NewEngine(site.ID, wafConfig)
		if err == nil {
			siteWafEngine = engine
		}
	}

	// IndexNow key file 托管（必须先于 WAF 中间件：搜索引擎验证器不带任何凭证，
	// 若被 WAF 误拦将导致所有权验证失败，IndexNow 配置形同虚设）
	if idxKey := site.Prerender.Push.IndexNowKey; idxKey != "" {
		keyPath := "/" + idxKey + ".txt"
		siteRouter.GET(keyPath, func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(idxKey))
			c.Abort()
		})
	}

	// WAF中间件 - 最先执行，保护后续处理
	siteRouter.Use(middleware.WafMiddleware(site, h.wafRepo, h.redisClient, h.geoIP, siteWafEngine, h.logWriter))

	// 爬虫检测中间件 - 第一个执行，确保爬虫请求得到正确处理
	siteRouter.Use(func(c *gin.Context) {
		// 将站点ID注入上下文，供渲染引擎生成站点隔离的缓存键（避免多站点缓存互相覆盖）
		c.Set("site_id", site.ID)

		// 获取请求的User-Agent
		userAgent := c.Request.UserAgent()

		// 检测爬虫
		isCrawler := false

		// 只有当prerenderManager不为nil时才使用引擎的检测方法
		if h.prerenderManager != nil {
			prerenderEngine, _ := h.prerenderManager.GetEngine(site.ID)
			if prerenderEngine != nil {
				isCrawler = prerenderEngine.IsCrawlerRequest(userAgent)
			} else {
				// 降级方案：使用默认的爬虫UA检测
				lowerUA := strings.ToLower(userAgent)
				isCrawler = strings.Contains(lowerUA, "baiduspider") ||
					strings.Contains(lowerUA, "googlebot") ||
					strings.Contains(lowerUA, "bingbot") ||
					strings.Contains(lowerUA, "yandexbot") ||
					strings.Contains(lowerUA, "sogou")
			}
		}

		if isCrawler {
			// 如果prerenderManager为nil，无法处理爬虫请求，返回500错误
			if h.prerenderManager == nil {
				lang := getLanguageFromRequest(c)
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": i18n.T(lang, "error.internal_server_error")})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 记录爬虫请求开始时间
			startTime := time.Now()

			// 记录爬虫请求
			monitor.RecordCrawlerRequest()

			// 构建完整的URL
			var scheme string
			if c.Request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
			fullURL := fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, c.Request.URL.Path)
			if c.Request.URL.RawQuery != "" {
				fullURL += "?" + c.Request.URL.RawQuery
			}

			// 获取当前站点的渲染预热引擎实例
			prerenderEngine, exists := h.prerenderManager.GetEngine(site.ID)
			if !exists {
				lang := getLanguageFromRequest(c)
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": i18n.T(lang, "error.internal_server_error")})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 获取User-Agent信息
			userAgent := c.Request.UserAgent()

			// G2 渲染范围名单过滤：exclude 命中（或 include 存在但未命中）直接透传上游不渲染
			include, exclude := compilePrerenderFilters(site)
			if !shouldPrerenderURL(c.Request.URL.Path, include, exclude) {
				c.Next()
				return
			}

			// 分类策略过滤：cache_only 类（默认 AI 爬虫）仅回缓存，未命中透传上游防渲染洪水
			category := prerender.ClassifyUserAgent(userAgent)
			policy := prerenderPolicyFor(site, category)
			if policy == policyPassthrough {
				c.Next()
				return
			}

			// 缓存优先（含软过期降级供数），未命中/按策略阻塞渲染时才走渲染管线
			cacheOnly := policy == policyCacheOnly
			served, serveErr := h.serveCrawlerFromCacheOrRender(site, prerenderEngine, c, fullURL, userAgent, startTime, cacheOnly, crawlerLogManager, monitor)
			if serveErr != nil {
				// 渲染失败且无旧值兜底：极简 HTML 503（爬虫可读、noindex），不再返回 JSON
				logging.DefaultLogger.Error("render failed for %s: %v", fullURL, serveErr)
				html := renderErrorHTML(getLanguageFromRequest(c), "")
				served := serveCrawlerPage(c, monitor, http.StatusServiceUnavailable, html, "miss", 0, time.Since(startTime).Seconds())
				recordCrawlerLog(crawlerLogManager, c, crawlerPageMeta{
					Site: site.ID, Route: c.Request.URL.Path, UA: userAgent,
					HitCache: "miss", Status: served, RenderTime: time.Since(startTime).Seconds(),
				})
				return
			}
			_ = served
			return
		}

		// 非爬虫请求，继续处理
		c.Next()
	})

	// 非爬虫请求处理中间件
	siteRouter.Use(func(c *gin.Context) {
		startTime := time.Now()

		// 记录正常访问日志
		defer func() {
			visitLog := logging.VisitLog{
				Site:     site.ID,
				IP:       logging.GetClientIP(c.Request),
				Time:     startTime,
				Method:   c.Request.Method,
				URL:      c.Request.URL.String(),
				Status:   c.Writer.Status(),
				UA:       c.Request.UserAgent(),
				Duration: time.Since(startTime).Seconds(),
				Referer:  c.Request.Referer(),
				Washed:   false,
			}
			visitLogManager.RecordVisitLog(visitLog)
		}()

		// 根据站点模式处理请求
		switch site.Mode {
		case "proxy":
			// 代理已有应用模式：将请求转发到上游服务
			proxyURL, err := url.Parse(site.Proxy.TargetURL)
			if err != nil {
				lang := getLanguageFromRequest(c)
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": i18n.T(lang, "error.invalid_request")})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, time.Since(startTime))
				c.Abort()
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(proxyURL)
			proxy.ServeHTTP(c.Writer, c.Request)
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
			c.Abort()
			return

		case "static":
			// 静态资源站模式：提供静态文件服务
			// 静态文件目录：{staticDir}/{site.ID}
			siteStaticDir := filepath.Join(staticDir, site.ID)

			// 确保静态文件目录存在
			if _, err := os.Stat(siteStaticDir); os.IsNotExist(err) {
				os.MkdirAll(siteStaticDir, 0755)
			}

			// 处理URL，移除hash部分并获取实际路径
			getActualPath := func(urlPath string) string {
				// 移除URL中的hash部分，因为hash不会发送到服务器
				if hashIndex := strings.Index(urlPath, "#"); hashIndex != -1 {
					return urlPath[:hashIndex]
				}
				return urlPath
			}

			// 检查请求的路径是否为静态资源
			isStaticResource := func(path string) bool {
				staticExtensions := []string{
					".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
					".css", ".less", ".sass", ".scss",
					".js", ".jsx", ".ts", ".tsx",
					".woff", ".woff2", ".ttf", ".eot",
					".ico", ".txt", ".json", ".xml", ".pdf", ".zip", ".rar",
					".mp4", ".mp3", ".avi", ".mov", ".wmv",
					".csv", ".xls", ".xlsx", ".doc", ".docx",
				}
				for _, ext := range staticExtensions {
					if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
						return true
					}
				}
				return false
			}

			// 获取实际路径（移除hash部分）
			actualPath := getActualPath(c.Request.URL.Path)

			// 对于静态资源，尝试直接提供文件
			if isStaticResource(actualPath) {
				filePath := filepath.Join(siteStaticDir, actualPath)
				if _, err := os.Stat(filePath); err == nil {
					c.File(filePath)
					monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
					return
				}
			}

			// 对于非静态资源，返回index.html（SPA路由处理）
			indexPath := filepath.Join(siteStaticDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				c.File(indexPath)
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
				return
			}

			// 文件不存在，返回404
			lang := getLanguageFromRequest(c)
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": i18n.T(lang, "error.not_found"),
				"data": gin.H{
					"site":    site.Name,
					"domains": site.Domains,
					"port":    site.Port,
					"path":    c.Request.URL.Path,
				},
			})
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusNotFound, time.Since(startTime))
			c.Abort()
			return

		case "redirect":
			// 重定向模式：返回重定向响应
			c.Redirect(site.Redirect.StatusCode, site.Redirect.TargetURL)
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, site.Redirect.StatusCode, time.Since(startTime))
			c.Abort()
			return

		default:
			// 未知模式，返回500错误
			lang := getLanguageFromRequest(c)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": i18n.T(lang, "error.invalid_request"),
			})
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, time.Since(startTime))
			c.Abort()
			return
		}
	})

	// 返回站点路由器作为HTTP处理器
	return siteRouter
}
