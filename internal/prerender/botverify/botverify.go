package botverify

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"prerender-shield/internal/logging"
)

// 验证结果常量（与 CrawlerLog.Verified 语义一致）
const (
	ResultVerified   = "verified"
	ResultUnverified = "unverified"
	ResultUnknown    = "" // DNS 失败等不确定态：不打标不拦截（fail-open）
)

// DNS 验证超时硬限制：验证发生在爬虫请求旁路，超时上限决定最坏时延
const dnsTimeout = 1500 * time.Millisecond

// 缓存 TTL：正向（verified）可信期长；负向（unverified）短，防止爬虫出口 IP 变更后误标
const (
	positiveTTL = 7 * 24 * time.Hour
	negativeTTL = 1 * time.Hour
)

// diskCacheMaxEntries 磁盘缓存容量上限（超限按最早解析时间 LRU 淘汰）
const diskCacheMaxEntries = 10000

// verifiedHostnameSuffixes Google 官方 rDNS 验证流程的主机名后缀白名单
var verifiedHostnameSuffixes = []string{
	"googlebot.com",
	"google.com",
	"googleusercontent.com",
}

// resolver DNS 解析接口（net.Resolver 满足；测试注入假实现避免触外网）
type resolver interface {
	LookupAddr(ctx context.Context, addr string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// Verifier 爬虫真实性验证器（Google 官方双向 rDNS 流程）：
//  1. LookupAddr 反向解析 IP → 主机名后缀必须命中白名单
//  2. LookupHost 正向解析该主机名 → 结果必须包含原 IP（防伪造 PTR 记录）
//
// 结果内存+磁盘双层缓存；singleflight 按 IP 去重并发验证。
type Verifier struct {
	resolver resolver

	mu       sync.RWMutex
	cache    map[string]cacheEntry
	diskPath string

	group singleflight.Group

	saveMu    sync.Mutex
	saveTimer *time.Timer
	dirty     bool
	flushNack int
}

// cacheEntry 单条验证缓存
type cacheEntry struct {
	Result   string    `json:"result"` // verified / unverified
	Host     string    `json:"host,omitempty"`
	CachedAt time.Time `json:"cached_at"`
}

type diskFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

// diskSaveDelay 防抖落盘延迟
const diskSaveDelay = 3 * time.Second

// NewVerifier 创建验证器；diskPath 为空时使用默认路径（env PRERENDER_BOTVERIFY_CACHE 可覆盖）
func NewVerifier(diskPath string) *Verifier {
	if diskPath == "" {
		diskPath = defaultDiskCachePath()
	}
	v := &Verifier{
		resolver: net.DefaultResolver,
		cache:    make(map[string]cacheEntry),
		diskPath: diskPath,
	}
	v.loadDisk()
	return v
}

func defaultDiskCachePath() string {
	if v := os.Getenv("PRERENDER_BOTVERIFY_CACHE"); v != "" {
		return v
	}
	return filepath.Join("data", "botverify_cache.json")
}

// Peek 仅读缓存（不触 DNS）：命中返回结果与 true；未缓存返回 ResultUnknown。
// 请求路径用它保证零阻塞：首访未缓存时由 VerifyAsync 异步回填，后续请求即时命中。
func (v *Verifier) Peek(ip string) string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	e, ok := v.cache[ip]
	if !ok {
		return ResultUnknown
	}
	ttl := negativeTTL
	if e.Result == ResultVerified {
		ttl = positiveTTL
	}
	if time.Since(e.CachedAt) >= ttl {
		return ResultUnknown
	}
	return e.Result
}

// Verify 同步执行完整验证（缓存优先）。DNS 故障 fail-open 返回 ResultUnknown 且不入缓存。
func (v *Verifier) Verify(ip string) string {
	if ip == "" {
		return ResultUnknown
	}
	if r := v.Peek(ip); r != ResultUnknown {
		return r
	}

	// singleflight：同一出口 IP 的并发爬虫请求只做一次 DNS 双向验证
	fv, err, _ := v.group.Do(ip, func() (interface{}, error) {
		return v.verifyOnce(ip), nil
	})
	if err != nil {
		return ResultUnknown
	}
	s, _ := fv.(string)
	return s
}

// VerifyAsync 异步验证并回填缓存（请求路径零阻塞）
func (v *Verifier) VerifyAsync(ip string) {
	if ip == "" {
		return
	}
	go func() {
		defer func() { _ = recover() }()
		_ = v.Verify(ip)
	}()
}

// verifyOnce 执行 DNS 双向验证，结果写入缓存
func (v *Verifier) verifyOnce(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	// 步骤1：反向解析，主机名后缀必须在白名单
	names, err := v.resolver.LookupAddr(ctx, ip)
	if err != nil {
		// 确认性"无 PTR 记录"= unverified；传输故障/超时 = 不确定态 fail-open 不缓存
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			v.setCache(ip, cacheEntry{Result: ResultUnverified, CachedAt: time.Now()})
			return ResultUnverified
		}
		return ResultUnknown
	}
	if len(names) == 0 {
		v.setCache(ip, cacheEntry{Result: ResultUnverified, CachedAt: time.Now()})
		return ResultUnverified
	}
	verifiedHost := ""
	for _, name := range names {
		host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
		if host == "" {
			continue
		}
		for _, suffix := range verifiedHostnameSuffixes {
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				verifiedHost = host
				break
			}
		}
		if verifiedHost != "" {
			break
		}
	}
	if verifiedHost == "" {
		v.setCache(ip, cacheEntry{Result: ResultUnverified, CachedAt: time.Now()})
		return ResultUnverified
	}

	// 步骤2：正向回验，解析出的 IP 列表必须包含原 IP
	addrs, err := v.resolver.LookupHost(ctx, verifiedHost)
	if err != nil {
		return ResultUnknown
	}
	forwardOK := false
	for _, a := range addrs {
		if a == ip {
			forwardOK = true
			break
		}
	}
	if !forwardOK {
		v.setCache(ip, cacheEntry{Result: ResultUnverified, Host: verifiedHost, CachedAt: time.Now()})
		return ResultUnverified
	}

	v.setCache(ip, cacheEntry{Result: ResultVerified, Host: verifiedHost, CachedAt: time.Now()})
	return ResultVerified
}

func (v *Verifier) setCache(ip string, e cacheEntry) {
	v.mu.Lock()
	v.cache[ip] = e
	v.evictLRULocked()
	v.mu.Unlock()
	v.scheduleFlush()
}

// evictLRULocked 超限按最早缓存时间淘汰（须持有 mu 写锁）
func (v *Verifier) evictLRULocked() {
	if len(v.cache) <= diskCacheMaxEntries {
		return
	}
	entries := make([]struct {
		ip string
		t  time.Time
	}, 0, len(v.cache))
	for ip, e := range v.cache {
		entries = append(entries, struct {
			ip string
			t  time.Time
		}{ip, e.CachedAt})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].t.Before(entries[j].t) })
	excess := len(v.cache) - diskCacheMaxEntries
	for _, e := range entries[:excess] {
		delete(v.cache, e.ip)
	}
}

// loadDisk 启动时加载持久缓存（文件不存在/损坏静默降级为空缓存）
func (v *Verifier) loadDisk() {
	if v.diskPath == "" {
		return
	}
	raw, err := os.ReadFile(v.diskPath)
	if err != nil {
		return
	}
	var file diskFile
	if err := json.Unmarshal(raw, &file); err != nil {
		logging.DefaultLogger.Warn("botverify disk cache corrupted at %s: %v — starting with empty cache", v.diskPath, err)
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	for ip, e := range file.Entries {
		ttl := negativeTTL
		if e.Result == ResultVerified {
			ttl = positiveTTL
		}
		if now.Sub(e.CachedAt) >= ttl {
			continue
		}
		v.cache[ip] = e
	}
	v.evictLRULocked()
}

// scheduleFlush 防抖调度异步落盘（原子 rename 写入）
func (v *Verifier) scheduleFlush() {
	v.saveMu.Lock()
	defer v.saveMu.Unlock()
	v.dirty = true
	if v.saveTimer == nil {
		v.saveTimer = time.AfterFunc(diskSaveDelay, func() {
			v.saveMu.Lock()
			v.saveTimer = nil
			v.dirty = false
			v.saveMu.Unlock()
			if err := v.Flush(); err != nil {
				logging.DefaultLogger.Warn("botverify disk cache flush failed: %v", err)
			}
		})
	}
}

// Flush 将内存缓存写入磁盘（临时文件 + 原子 rename，避免半写损坏）
func (v *Verifier) Flush() error {
	if v.diskPath == "" {
		return nil
	}
	v.mu.RLock()
	file := diskFile{Entries: make(map[string]cacheEntry, len(v.cache))}
	for ip, e := range v.cache {
		file.Entries[ip] = e
	}
	v.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(v.diskPath), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	tmp := v.diskPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, v.diskPath)
}

// NewVerifierWithResolver 测试专用构造：注入假 resolver，不触外网
func NewVerifierWithResolver(r resolver, diskPath string) *Verifier {
	v := NewVerifier(diskPath)
	v.resolver = r
	return v
}
