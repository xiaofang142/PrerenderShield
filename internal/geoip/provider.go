package geoip

import (
	"net"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/logging"
)

// Provider defines the interface for GeoIP lookup
type Provider interface {
	Lookup(ip string) (*CountryResult, error)
	Name() string
}

// CountryResult holds the result of a GeoIP lookup
type CountryResult struct {
	CountryCode string
	CountryName string
	IP          string
}

const (
	// memoryCacheTTL 内存缓存有效期（新鲜数据直接可用）
	memoryCacheTTL = 24 * time.Hour
	// diskCacheTTL 磁盘缓存兜底有效期：API 全部失败（断网/限流）时，
	// 允许回退到最长 7 天内解析过的旧结果，而不是立即降级为 UNKNOWN
	diskCacheTTL = 7 * 24 * time.Hour
	// diskSaveDelay 新结果落盘的防抖延迟，避免高流量下每请求写文件
	diskSaveDelay = 3 * time.Second
	// diskCacheMaxEntries 磁盘缓存条目上限，超出后按 LRU（最早解析时间）淘汰
	diskCacheMaxEntries = 10000
)

// APIProvider uses free HTTP APIs for GeoIP lookup
type APIProvider struct {
	name      string
	apiURL    string
	apiKey    string
	client    *http.Client
	cache     map[string]*CountryResult
	cacheTTL  time.Duration
	cacheMu   sync.RWMutex
	cacheTime map[string]time.Time

	// 本地磁盘持久缓存层：进程重启后仍可复用历史解析结果
	diskPath  string
	saveMu    sync.Mutex
	dirty     bool
	saveTimer *time.Timer
}

// NewAPIProvider creates a new API-based GeoIP provider
func NewAPIProvider(name, apiURL, apiKey string) *APIProvider {
	return NewAPIProviderWithCache(name, apiURL, apiKey, defaultDiskCachePath())
}

// NewAPIProviderWithCache creates a provider with an explicit disk cache path
func NewAPIProviderWithCache(name, apiURL, apiKey, diskPath string) *APIProvider {
	p := &APIProvider{
		name:      name,
		apiURL:    apiURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 5 * time.Second},
		cache:     make(map[string]*CountryResult),
		cacheTTL:  memoryCacheTTL,
		cacheTime: make(map[string]time.Time),
		diskPath:  diskPath,
	}
	p.loadDisk()
	return p
}

// defaultDiskCachePath 返回磁盘缓存默认路径，可通过环境变量覆盖
func defaultDiskCachePath() string {
	if v := os.Getenv("PRERENDER_GEOIP_CACHE"); v != "" {
		return v
	}
	return filepath.Join("data", "geoip_cache.json")
}

func (p *APIProvider) Name() string {
	return p.name
}

func (p *APIProvider) Lookup(ip string) (*CountryResult, error) {
	// Skip private IPs
	if isPrivateIP(ip) {
		return &CountryResult{
			CountryCode: "LOCAL",
			CountryName: "Local",
			IP:          ip,
		}, nil
	}

	// Check cache
	if cached := p.getFromCache(ip); cached != nil {
		return cached, nil
	}

	// Query API
	result, err := p.queryAPI(ip)
	if err != nil {
		// API 失败（断网/限流）时回退到磁盘持久缓存中的历史结果，
		// 避免 WAF 在 AllowList 模式下因 UNKNOWN 而误杀正常流量
		if stale := p.getStaleFromCache(ip); stale != nil {
			logging.DefaultLogger.Warn("GeoIP API failed for %s (%v), using stale cached result %s", ip, err, stale.CountryCode)
			return stale, nil
		}
		return nil, err
	}

	// Store in cache
	p.setCache(ip, result)

	return result, nil
}

func (p *APIProvider) queryAPI(ip string) (*CountryResult, error) {
	url := strings.Replace(p.apiURL, "{ip}", ip, -1)
	if !strings.Contains(p.apiURL, "{ip}") {
		url = p.apiURL + "/" + ip
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return p.parseResponse(body)
}

func (p *APIProvider) parseResponse(body []byte) (*CountryResult, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result := &CountryResult{}

	// Handle ip-api.com format
	if cc, ok := data["countryCode"].(string); ok {
		result.CountryCode = cc
	}
	if cn, ok := data["country"].(string); ok {
		result.CountryName = cn
	}
	if ip, ok := data["query"].(string); ok {
		result.IP = ip
	}

	// Handle ipinfo.io format
	if cc, ok := data["country"].(string); ok && result.CountryCode == "" {
		result.CountryCode = cc
	}

	// Handle ipapi.co format
	if cc, ok := data["country_code"].(string); ok && result.CountryCode == "" {
		result.CountryCode = cc
	}

	if result.CountryCode == "" {
		return nil, fmt.Errorf("could not extract country code from response")
	}

	return result, nil
}

func (p *APIProvider) getFromCache(ip string) *CountryResult {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	if result, ok := p.cache[ip]; ok {
		if t, ok := p.cacheTime[ip]; ok && time.Since(t) < p.cacheTTL {
			return result
		}
	}
	return nil
}

func (p *APIProvider) setCache(ip string, result *CountryResult) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	p.cache[ip] = result
	p.cacheTime[ip] = time.Now()
	p.evictLRULocked()
	p.scheduleFlush()
}

// evictLRULocked 缓存超过容量上限时按最早解析时间淘汰（须持有 cacheMu 写锁）
func (p *APIProvider) evictLRULocked() {
	if len(p.cache) <= diskCacheMaxEntries {
		return
	}

	type ipTime struct {
		ip string
		t  time.Time
	}
	entries := make([]ipTime, 0, len(p.cacheTime))
	for ip, t := range p.cacheTime {
		entries = append(entries, ipTime{ip, t})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].t.Before(entries[j].t) })

	excess := len(p.cache) - diskCacheMaxEntries
	for _, e := range entries[:excess] {
		delete(p.cache, e.ip)
		delete(p.cacheTime, e.ip)
	}
}

// getStaleFromCache 返回超过 memoryTTL 但仍在 diskTTL 内的历史结果（API 失败兜底用）
func (p *APIProvider) getStaleFromCache(ip string) *CountryResult {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	if result, ok := p.cache[ip]; ok {
		if t, ok := p.cacheTime[ip]; ok && time.Since(t) < diskCacheTTL {
			return result
		}
	}
	return nil
}

// diskEntry 磁盘缓存文件中的单条记录
type diskEntry struct {
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name,omitempty"`
	IP          string `json:"ip"`
	CachedAt    string `json:"cached_at"`
}

type diskFile struct {
	Entries map[string]diskEntry `json:"entries"`
}

// loadDisk 启动时加载历史解析结果（文件不存在或损坏时静默降级为空缓存）
func (p *APIProvider) loadDisk() {
	if p.diskPath == "" {
		return
	}
	raw, err := os.ReadFile(p.diskPath)
	if err != nil {
		return
	}
	var file diskFile
	if err := json.Unmarshal(raw, &file); err != nil {
		logging.DefaultLogger.Warn("GeoIP disk cache corrupted at %s: %v — starting with empty cache", p.diskPath, err)
		return
	}

	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	now := time.Now()
	for ip, e := range file.Entries {
		cachedAt, err := time.Parse(time.RFC3339, e.CachedAt)
		if err != nil || now.Sub(cachedAt) >= diskCacheTTL {
			continue
		}
		p.cache[ip] = &CountryResult{CountryCode: e.CountryCode, CountryName: e.CountryName, IP: e.IP}
		p.cacheTime[ip] = cachedAt
	}
	p.evictLRULocked()
}

// scheduleFlush 防抖调度异步落盘
func (p *APIProvider) scheduleFlush() {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()

	p.dirty = true
	if p.saveTimer == nil {
		p.saveTimer = time.AfterFunc(diskSaveDelay, func() {
			p.saveMu.Lock()
			p.saveTimer = nil
			p.dirty = false
			p.saveMu.Unlock()
			if err := p.Flush(); err != nil {
				logging.DefaultLogger.Warn("GeoIP disk cache flush failed: %v", err)
			}
		})
	}
}

// Flush 将内存缓存快照写入磁盘（临时文件+rename 原子替换）
func (p *APIProvider) Flush() error {
	if p.diskPath == "" {
		return nil
	}

	p.cacheMu.RLock()
	file := diskFile{Entries: make(map[string]diskEntry, len(p.cache))}
	for ip, r := range p.cache {
		t := p.cacheTime[ip]
		file.Entries[ip] = diskEntry{
			CountryCode: r.CountryCode,
			CountryName: r.CountryName,
			IP:          r.IP,
			CachedAt:    t.Format(time.RFC3339),
		}
	}
	p.cacheMu.RUnlock()

	if len(file.Entries) == 0 {
		return nil
	}

	raw, err := json.MarshalIndent(&file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal geoip cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.diskPath), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp := p.diskPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write cache temp file: %w", err)
	}
	return os.Rename(tmp, p.diskPath)
}

// ClearCache clears the provider cache
func (p *APIProvider) ClearCache() {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	p.cache = make(map[string]*CountryResult)
	p.cacheTime = make(map[string]time.Time)
}

// isPrivateIP checks if an IP is a private/local address
func isPrivateIP(ip string) bool {
	if ip == "localhost" {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// 环回 / 私网(v4+IPv4映射) / 链路本地 / IPv6 唯一本地地址(ULA fc00::/7)
	return parsed.IsLoopback() ||
		parsed.IsPrivate() ||
		parsed.IsLinkLocalUnicast() ||
		parsed.IsLinkLocalMulticast() ||
		(parsed.To4() == nil && (parsed.IsUnspecified() || isUniqueLocal(parsed)))
}

// isUniqueLocal 判断 IPv6 唯一本地地址 fc00::/7
func isUniqueLocal(ip net.IP) bool {
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

// Pre-configured providers

// NewIPAPIProvider creates a provider using ip-api.com (free, 45 req/min)
func NewIPAPIProvider() *APIProvider {
	return NewAPIProvider("ip-api.com", "http://ip-api.com/json/{ip}?fields=countryCode,country,query", "")
}

// NewIPInfoProvider creates a provider using ipinfo.io (free, 50k req/month)
func NewIPInfoProvider(token string) *APIProvider {
	return NewAPIProvider("ipinfo.io", "https://ipinfo.io/{ip}/country", token)
}

// NewIPAPIProviderCO creates a provider using ipapi.co (free, 1k req/day)
func NewIPAPIProviderCO() *APIProvider {
	return NewAPIProvider("ipapi.co", "https://ipapi.co/{ip}/json/", "")
}
