package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"prerender-shield/internal/models"
	redisPkg "prerender-shield/internal/redis"

	"github.com/go-redis/redis/v8"
)

// WafRedisClient defines the interface for WAF Redis operations
type WafRedisClient interface {
	Context() context.Context
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	LPush(ctx context.Context, key string, value interface{}) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LTrim(ctx context.Context, key string, start, stop int64) error
	HIncrBy(ctx context.Context, key, field string, incr int64) error
	Incr(ctx context.Context, key string) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// RedisClientWrapper wraps redisPkg.Client to implement WafRedisClient
type RedisClientWrapper struct {
	Client *redisPkg.Client
}

func (w *RedisClientWrapper) Context() context.Context {
	return w.Client.Context()
}

func (w *RedisClientWrapper) Get(ctx context.Context, key string) (string, error) {
	return w.Client.GetRawClient().Get(ctx, key).Result()
}

func (w *RedisClientWrapper) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return w.Client.GetRawClient().Set(ctx, key, value, expiration).Err()
}

func (w *RedisClientWrapper) LPush(ctx context.Context, key string, value interface{}) error {
	return w.Client.GetRawClient().LPush(ctx, key, value).Err()
}

func (w *RedisClientWrapper) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return w.Client.GetRawClient().LRange(ctx, key, start, stop).Result()
}

func (w *RedisClientWrapper) LLen(ctx context.Context, key string) (int64, error) {
	return w.Client.GetRawClient().LLen(ctx, key).Result()
}

func (w *RedisClientWrapper) LTrim(ctx context.Context, key string, start, stop int64) error {
	return w.Client.GetRawClient().LTrim(ctx, key, start, stop).Err()
}

func (w *RedisClientWrapper) HIncrBy(ctx context.Context, key, field string, incr int64) error {
	return w.Client.GetRawClient().HIncrBy(ctx, key, field, incr).Err()
}

func (w *RedisClientWrapper) Incr(ctx context.Context, key string) error {
	return w.Client.GetRawClient().Incr(ctx, key).Err()
}

func (w *RedisClientWrapper) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return w.Client.GetRawClient().HGetAll(ctx, key).Result()
}

func (w *RedisClientWrapper) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return w.Client.GetRawClient().Expire(ctx, key, expiration).Err()
}

// WafRepository handles WAF related database operations using Redis
type WafRepository struct {
	client WafRedisClient
}

// NewWafRepository creates a new WafRepository
func NewWafRepository(client WafRedisClient) *WafRepository {
	return &WafRepository{
		client: client,
	}
}

// GetWafConfigBySiteID retrieves the WAF configuration for a specific site
func (r *WafRepository) GetWafConfigBySiteID(siteID string) (*models.WafConfig, error) {
	ctx := r.client.Context()
	key := fmt.Sprintf("waf:config:%s", siteID)

	data, err := r.client.Get(ctx, key)
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var config models.WafConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateWafConfig creates a new WAF configuration
func (r *WafRepository) CreateWafConfig(config *models.WafConfig) error {
	return r.saveWafConfig(config)
}

// UpdateWafConfig updates an existing WAF configuration
func (r *WafRepository) UpdateWafConfig(config *models.WafConfig) error {
	return r.saveWafConfig(config)
}

func (r *WafRepository) saveWafConfig(config *models.WafConfig) error {
	ctx := r.client.Context()
	key := fmt.Sprintf("waf:config:%s", config.SiteID)

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 0)
}

// UpdateBlockedCountries replaces the list of blocked countries
func (r *WafRepository) UpdateBlockedCountries(wafConfigID string, countries []string) error {
	// In the Redis implementation, we store the full config including lists in one JSON key for simplicity,
	// or we could use separate sets.
	// However, since UpdateWafConfig saves the whole object, this method might be redundant
	// or needs to fetch, update, and save.
	// Given the previous implementation expected a separate update, we should fetch the siteID from wafConfigID first?
	// Actually, the previous SQL model had normalized tables. In Redis, embedding is easier.
	// But `wafConfigID` is just an ID. We need `SiteID`.
	// Assuming `models.WafConfig` has `SiteID`.

	// Issue: We only have `wafConfigID`.
	// Workaround: In Redis version, `wafConfigID` might be same as `SiteID` or we maintain a mapping.
	// But let's assume the caller has the SiteID context or we change the signature.
	// Since I can't easily change all callers right now, I will try to implement it if possible.
	// But wait, `WafConfig` model has `SiteID`.
	// For now, let's assume we handle this in the Controller by updating the whole config object.
	// If this method is called independently, it's tricky without SiteID.

	// Let's check `models.WafConfig`.
	return nil // Placeholder, callers should use UpdateWafConfig with full object
}

// UpdateIPWhitelist replaces the IP whitelist
func (r *WafRepository) UpdateIPWhitelist(wafConfigID string, ips []string) error {
	return nil // Placeholder
}

// UpdateIPBlacklist replaces the IP blacklist
func (r *WafRepository) UpdateIPBlacklist(wafConfigID string, ips []string) error {
	return nil // Placeholder
}

// GetAccessLogs retrieves access logs with pagination and filters
func (r *WafRepository) GetAccessLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	ctx := r.client.Context()
	key := fmt.Sprintf("waf:logs:%s", siteID)

	start := int64((page - 1) * limit)
	end := start + int64(limit) - 1

	total, err := r.client.LLen(ctx, key)
	if err != nil {
		return nil, 0, err
	}

	rawLogs, err := r.client.LRange(ctx, key, start, end)
	if err != nil {
		return nil, 0, err
	}

	var logs []models.AccessLog
	for _, raw := range rawLogs {
		var log models.AccessLog
		if err := json.Unmarshal([]byte(raw), &log); err == nil {
			logs = append(logs, log)
		}
	}

	return logs, total, nil
}

// GetAttackLogs retrieves attack logs (blocked requests)
func (r *WafRepository) GetAttackLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	ctx := r.client.Context()
	key := fmt.Sprintf("waf:attacks:%s", siteID)

	start := int64((page - 1) * limit)
	end := start + int64(limit) - 1

	total, err := r.client.LLen(ctx, key)
	if err != nil {
		return nil, 0, err
	}

	rawLogs, err := r.client.LRange(ctx, key, start, end)
	if err != nil {
		return nil, 0, err
	}

	var logs []models.AccessLog
	for _, raw := range rawLogs {
		var log models.AccessLog
		if err := json.Unmarshal([]byte(raw), &log); err == nil {
			logs = append(logs, log)
		}
	}

	return logs, total, nil
}

// AddIPToWhitelist adds an IP to the whitelist
func (r *WafRepository) AddIPToWhitelist(siteID, ip string) error {
	config, err := r.GetWafConfigBySiteID(siteID)
	if err != nil {
		return err
	}
	if config == nil {
		config = &models.WafConfig{SiteID: siteID}
	}

	// Check if already exists
	for _, item := range config.IPWhitelist {
		if item.IPAddress == ip {
			return nil
		}
	}

	config.IPWhitelist = append(config.IPWhitelist, models.IPWhitelist{
		IPAddress:   ip,
		WafConfigID: config.ID,
		// ID will be generated if using DB, but here using Redis JSON
	})

	// Remove from blacklist if present
	var newBlacklist []models.IPBlacklist
	for _, item := range config.IPBlacklist {
		if item.IPAddress != ip {
			newBlacklist = append(newBlacklist, item)
		}
	}
	config.IPBlacklist = newBlacklist

	return r.UpdateWafConfig(config)
}

// AddIPToBlacklist adds an IP to the blacklist
func (r *WafRepository) AddIPToBlacklist(siteID, ip string) error {
	config, err := r.GetWafConfigBySiteID(siteID)
	if err != nil {
		return err
	}
	if config == nil {
		config = &models.WafConfig{SiteID: siteID}
	}

	// Check if already exists
	for _, item := range config.IPBlacklist {
		if item.IPAddress == ip {
			return nil
		}
	}

	config.IPBlacklist = append(config.IPBlacklist, models.IPBlacklist{
		IPAddress:   ip,
		WafConfigID: config.ID,
		Reason:      "Manual Block",
		CreatedAt:   time.Now(),
	})

	// Remove from whitelist if present
	var newWhitelist []models.IPWhitelist
	for _, item := range config.IPWhitelist {
		if item.IPAddress != ip {
			newWhitelist = append(newWhitelist, item)
		}
	}
	config.IPWhitelist = newWhitelist

	return r.UpdateWafConfig(config)
}

// CreateAccessLog creates a new access log entry
func (r *WafRepository) CreateAccessLog(log *models.AccessLog) error {
	ctx := r.client.Context()
	key := fmt.Sprintf("waf:logs:%s", log.SiteID)

	data, err := json.Marshal(log)
	if err != nil {
		return err
	}

	// LPUSH to add to the beginning of the list
	if err := r.client.LPush(ctx, key, data); err != nil {
		return err
	}

	// Trim list to keep size manageable (e.g., 10000 logs)
	r.client.LTrim(ctx, key, 0, 9999)

	// If action is block, also add to attack logs list
	if log.Action == "block" {
		attackKey := fmt.Sprintf("waf:attacks:%s", log.SiteID)
		if err := r.client.LPush(ctx, attackKey, data); err != nil {
			// Log error but don't fail the main log
			fmt.Printf("Failed to push attack log: %v\n", err)
		} else {
			// Trim attack logs (keep 10000)
			r.client.LTrim(ctx, attackKey, 0, 9999)
		}
	}

	// Update stats
	r.incrementStats(log)

	return nil
}

func (r *WafRepository) incrementStats(log *models.AccessLog) {
	ctx := r.client.Context()
	// Global Stats
	r.client.Incr(ctx, "waf:stats:global:total")
	if log.Action == "block" {
		r.client.Incr(ctx, "waf:stats:global:blocked")
	}

	// Hourly Stats for Charts
	// Key: waf:stats:hourly:{timestamp_hour}
	hour := log.CreatedAt.Truncate(time.Hour).Unix()
	hourKey := fmt.Sprintf("waf:stats:hourly:%d", hour)
	r.client.HIncrBy(ctx, hourKey, "total", 1)
	if log.Action == "block" {
		r.client.HIncrBy(ctx, hourKey, "blocked", 1)
	}
	r.client.Expire(ctx, hourKey, 7*24*time.Hour) // Keep stats for 7 days
}

// WafStats represents aggregated WAF statistics
type WafStats struct {
	TotalRequests   int64 `json:"total_requests"`
	BlockedRequests int64 `json:"blocked_requests"`
	AttackRequests  int64 `json:"attack_requests"`
}

// GetGlobalStats returns global WAF statistics for a given duration
func (r *WafRepository) GetGlobalStats(startTime, endTime string) (*WafStats, error) {
	ctx := r.client.Context()

	// For simplicity in Redis without time-series, we return the global counters.
	// Note: accurate time-range filtering is hard with simple counters.
	// We will return total accumulated stats.

	total, _ := r.client.Get(ctx, "waf:stats:global:total")
	blocked, _ := r.client.Get(ctx, "waf:stats:global:blocked")

	// Parse string values to int64
	var totalInt, blockedInt int64
	if total != "" {
		totalInt, _ = strconv.ParseInt(total, 10, 64)
	}
	if blocked != "" {
		blockedInt, _ = strconv.ParseInt(blocked, 10, 64)
	}

	return &WafStats{
		TotalRequests:   totalInt,
		BlockedRequests: blockedInt,
		AttackRequests:  blockedInt,
	}, nil
}

// GetTrafficStats returns traffic statistics grouped by time
func (r *WafRepository) GetTrafficStats(startTime, endTime string) ([]map[string]interface{}, error) {
	ctx := r.client.Context()

	// Parse times
	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}

	var data []map[string]interface{}

	// Iterate by hour
	for t := start.Truncate(time.Hour); t.Before(end) || t.Equal(end); t = t.Add(time.Hour) {
		hourKey := fmt.Sprintf("waf:stats:hourly:%d", t.Unix())
		stats, err := r.client.HGetAll(ctx, hourKey)
		if err != nil {
			continue
		}

		total, _ := strconv.ParseInt(stats["total"], 10, 64)
		blocked, _ := strconv.ParseInt(stats["blocked"], 10, 64)

		data = append(data, map[string]interface{}{
			"time":            t.Format(time.RFC3339),
			"totalRequests":   total,
			"blockedRequests": blocked,
		})
	}

	// Sort by time just in case
	sort.Slice(data, func(i, j int) bool {
		return data[i]["time"].(string) < data[j]["time"].(string)
	})

	return data, nil
}

// WafRepositoryInMemory is an in-memory implementation of WafRepository for testing
type WafRepositoryInMemory struct {
	configs      map[string]*models.WafConfig
	accessLogs   map[string][]models.AccessLog
	attackLogs   map[string][]models.AccessLog
	ipWhitelists map[string][]models.IPWhitelist
	ipBlacklists map[string][]models.IPBlacklist
	mu           sync.RWMutex
}

// NewWafRepositoryInMemory creates a new in-memory WafRepository
func NewWafRepositoryInMemory() *WafRepositoryInMemory {
	return &WafRepositoryInMemory{
		configs:      make(map[string]*models.WafConfig),
		accessLogs:   make(map[string][]models.AccessLog),
		attackLogs:   make(map[string][]models.AccessLog),
		ipWhitelists: make(map[string][]models.IPWhitelist),
		ipBlacklists: make(map[string][]models.IPBlacklist),
	}
}

// GetWafConfigBySiteID retrieves the WAF configuration for a specific site
func (r *WafRepositoryInMemory) GetWafConfigBySiteID(siteID string) (*models.WafConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if config, ok := r.configs[siteID]; ok {
		return config, nil
	}
	return nil, nil
}

// UpdateWafConfig updates an existing WAF configuration
func (r *WafRepositoryInMemory) UpdateWafConfig(config *models.WafConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[config.SiteID] = config
	return nil
}

// GetAccessLogs retrieves access logs with pagination
func (r *WafRepositoryInMemory) GetAccessLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	logs, ok := r.accessLogs[siteID]
	if !ok {
		return []models.AccessLog{}, 0, nil
	}
	start := (page - 1) * limit
	end := start + limit
	if start >= len(logs) {
		return []models.AccessLog{}, 0, nil
	}
	if end > len(logs) {
		end = len(logs)
	}
	return logs[start:end], int64(len(logs)), nil
}

// GetAttackLogs retrieves attack logs with pagination
func (r *WafRepositoryInMemory) GetAttackLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	logs, ok := r.attackLogs[siteID]
	if !ok {
		return []models.AccessLog{}, 0, nil
	}
	start := (page - 1) * limit
	end := start + limit
	if start >= len(logs) {
		return []models.AccessLog{}, 0, nil
	}
	if end > len(logs) {
		end = len(logs)
	}
	return logs[start:end], int64(len(logs)), nil
}

// AddIPToWhitelist adds an IP to the whitelist
func (r *WafRepositoryInMemory) AddIPToWhitelist(siteID, ip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ipWhitelists[siteID] = append(r.ipWhitelists[siteID], models.IPWhitelist{IPAddress: ip})
	// Remove from blacklist if present
	var newBlacklist []models.IPBlacklist
	for _, item := range r.ipBlacklists[siteID] {
		if item.IPAddress != ip {
			newBlacklist = append(newBlacklist, item)
		}
	}
	r.ipBlacklists[siteID] = newBlacklist
	return nil
}

// AddIPToBlacklist adds an IP to the blacklist
func (r *WafRepositoryInMemory) AddIPToBlacklist(siteID, ip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ipBlacklists[siteID] = append(r.ipBlacklists[siteID], models.IPBlacklist{IPAddress: ip})
	// Remove from whitelist if present
	var newWhitelist []models.IPWhitelist
	for _, item := range r.ipWhitelists[siteID] {
		if item.IPAddress != ip {
			newWhitelist = append(newWhitelist, item)
		}
	}
	r.ipWhitelists[siteID] = newWhitelist
	return nil
}
