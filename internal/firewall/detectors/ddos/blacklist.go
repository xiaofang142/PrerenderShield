package ddos

import (
	"sync"
	"time"
)

// Blacklist IP 黑名单
type Blacklist struct {
	mu            sync.RWMutex
	blockedIPs    map[string]*BlacklistEntry
	blockDuration time.Duration
}

// BlacklistEntry 黑名单条目
type BlacklistEntry struct {
	IP        string
	Reason    string
	BlockedAt time.Time
	ExpiresAt time.Time
	Hits      int // 命中次数
}

// NewBlacklist 创建黑名单
func NewBlacklist(blockDuration time.Duration) *Blacklist {
	if blockDuration <= 0 {
		blockDuration = 10 * time.Minute
	}
	return &Blacklist{
		blockedIPs:    make(map[string]*BlacklistEntry),
		blockDuration: blockDuration,
	}
}

// Add 添加 IP 到黑名单
func (b *Blacklist) Add(ip, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(b.blockDuration)

	// 如果已存在，更新过期时间和命中次数
	if entry, exists := b.blockedIPs[ip]; exists {
		entry.Hits++
		entry.ExpiresAt = expiresAt
		if reason != "" {
			entry.Reason = reason
		}
		return
	}

	b.blockedIPs[ip] = &BlacklistEntry{
		IP:        ip,
		Reason:    reason,
		BlockedAt: now,
		ExpiresAt: expiresAt,
		Hits:      1,
	}
}

// AddWithDuration 添加 IP 到黑名单（自定义时长）
func (b *Blacklist) AddWithDuration(ip, reason string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(duration)

	b.blockedIPs[ip] = &BlacklistEntry{
		IP:        ip,
		Reason:    reason,
		BlockedAt: now,
		ExpiresAt: expiresAt,
		Hits:      1,
	}
}

// Remove 从黑名单移除 IP
func (b *Blacklist) Remove(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.blockedIPs[ip]; exists {
		delete(b.blockedIPs, ip)
		return true
	}
	return false
}

// IsBlacklisted 检查 IP 是否在黑名单中
func (b *Blacklist) IsBlacklisted(ip string) bool {
	b.mu.RLock()
	entry, exists := b.blockedIPs[ip]
	if !exists {
		b.mu.RUnlock()
		return false
	}
	b.mu.RUnlock()

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		b.mu.Lock()
		delete(b.blockedIPs, ip)
		b.mu.Unlock()
		return false
	}

	return true
}

// IsBlacklistedWithReason 检查 IP 是否在黑名单中并返回原因
func (b *Blacklist) IsBlacklistedWithReason(ip string) (bool, string) {
	b.mu.RLock()
	entry, exists := b.blockedIPs[ip]
	if !exists {
		b.mu.RUnlock()
		return false, ""
	}
	b.mu.RUnlock()

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		b.mu.Lock()
		delete(b.blockedIPs, ip)
		b.mu.Unlock()
		return false, ""
	}

	return true, entry.Reason
}

// GetEntry 获取黑名单条目
func (b *Blacklist) GetEntry(ip string) *BlacklistEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.blockedIPs[ip]
}

// CleanupExpired 清理过期条目
func (b *Blacklist) CleanupExpired() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for ip, entry := range b.blockedIPs {
		if now.After(entry.ExpiresAt) {
			delete(b.blockedIPs, ip)
		}
	}
}

// GetCount 获取黑名单条目数量
func (b *Blacklist) GetCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.blockedIPs)
}

// GetAll 获取所有黑名单条目
func (b *Blacklist) GetAll() []*BlacklistEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	entries := make([]*BlacklistEntry, 0, len(b.blockedIPs))
	for _, entry := range b.blockedIPs {
		entries = append(entries, entry)
	}
	return entries
}

// GetActiveCount 获取未过期的条目数量
func (b *Blacklist) GetActiveCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	count := 0
	for _, entry := range b.blockedIPs {
		if now.Before(entry.ExpiresAt) {
			count++
		}
	}
	return count
}

// Clear 清空黑名单
func (b *Blacklist) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blockedIPs = make(map[string]*BlacklistEntry)
}

// GetStats 获取黑名单统计
func (b *Blacklist) GetStats() *BlacklistStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	total := len(b.blockedIPs)
	active := 0
	expired := 0

	for _, entry := range b.blockedIPs {
		if now.Before(entry.ExpiresAt) {
			active++
		} else {
			expired++
		}
	}

	return &BlacklistStats{
		Total:    total,
		Active:   active,
		Expired:  expired,
		Duration: b.blockDuration,
	}
}

// BlacklistStats 黑名单统计
type BlacklistStats struct {
	Total    int           `json:"total"`
	Active   int           `json:"active"`
	Expired  int           `json:"expired"`
	Duration time.Duration `json:"duration"`
}

// Block 封禁 IP（别名方法）
func (b *Blacklist) Block(ip string, duration time.Duration) {
	b.AddWithDuration(ip, "ddos_attack", duration)
}

// Unblock 解封 IP（别名方法）
func (b *Blacklist) Unblock(ip string) bool {
	return b.Remove(ip)
}

// IsBlocked 检查是否被封禁（别名方法）
func (b *Blacklist) IsBlocked(ip string) bool {
	return b.IsBlacklisted(ip)
}

// IncrementHits 增加命中次数
func (b *Blacklist) IncrementHits(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		entry.Hits++
	}
}

// GetHits 获取命中次数
func (b *Blacklist) GetHits(ip string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		return entry.Hits
	}
	return 0
}

// ExtendBlock 延长封禁时间
func (b *Blacklist) ExtendBlock(ip string, additionalDuration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		entry.ExpiresAt = entry.ExpiresAt.Add(additionalDuration)
	}
}

// GetBlockReason 获取封禁原因
func (b *Blacklist) GetBlockReason(ip string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		return entry.Reason
	}
	return ""
}

// GetBlockedAt 获取封禁时间
func (b *Blacklist) GetBlockedAt(ip string) time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		return entry.BlockedAt
	}
	return time.Time{}
}

// GetExpiresAt 获取过期时间
func (b *Blacklist) GetExpiresAt(ip string) time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		return entry.ExpiresAt
	}
	return time.Time{}
}

// GetRemainingTime 获取剩余封禁时间
func (b *Blacklist) GetRemainingTime(ip string) time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if entry, exists := b.blockedIPs[ip]; exists {
		remaining := time.Until(entry.ExpiresAt)
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return 0
}

// BlacklistMiddleware 创建黑名单中间件（已废弃：DDoS 检测器内部已处理黑名单逻辑）
// 保留此方法仅为向后兼容，建议使用 WafMiddleware 中的黑名单检查
func (b *Blacklist) BlacklistMiddleware(blockedHandler func(ip, reason string) interface{}) func(interface{}) (interface{}, error) {
	return func(ctx interface{}) (interface{}, error) {
		// DDoS 黑名单已在检测器内部处理，此中间件仅为兼容保留
		return ctx, nil
	}
}
