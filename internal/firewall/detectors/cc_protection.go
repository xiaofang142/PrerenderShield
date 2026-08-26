package detectors

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// CCRedisClient defines the interface for Redis operations needed by CC protection
type CCRedisClient interface {
	Get(key string) (string, error)
	Incr(key string) (int64, error)
	Expire(key string, expiration time.Duration) error
	Set(key string, value interface{}, expiration time.Duration) error
}

// CCProtectionRule defines a custom CC protection rule
type CCProtectionRule struct {
	Name       string   `json:"name" yaml:"name"`
	Path       string   `json:"path" yaml:"path"`
	Method     string   `json:"method" yaml:"method"`
	Dimensions []string `json:"dimensions" yaml:"dimensions"`
	Requests   int      `json:"requests" yaml:"requests"`
	Window     int      `json:"window" yaml:"window"`
	BanTime    int      `json:"ban_time" yaml:"ban_time"`
	Enabled    bool     `json:"enabled" yaml:"enabled"`
}

// CCProtectionConfig holds the CC protection configuration
type CCProtectionConfig struct {
	Enabled bool               `json:"enabled" yaml:"enabled"`
	Rules   []CCProtectionRule `json:"rules" yaml:"rules"`
}

// CCProtectionDetector provides custom CC protection with multi-dimensional rate limiting
type CCProtectionDetector struct {
	config      CCProtectionConfig
	redisClient CCRedisClient
	mu          sync.RWMutex
}

// NewCCProtectionDetector creates a new CC protection detector
func NewCCProtectionDetector(config CCProtectionConfig, redisClient CCRedisClient) *CCProtectionDetector {
	return &CCProtectionDetector{
		config:      config,
		redisClient: redisClient,
	}
}

// Name returns the detector name
func (d *CCProtectionDetector) Name() string {
	return "cc_protection"
}

// Detect checks if the request triggers any CC protection rules
func (d *CCProtectionDetector) Detect(req *http.Request) ([]types.Threat, error) {
	if !d.config.Enabled || d.redisClient == nil {
		return nil, nil
	}

	d.mu.RLock()
	rules := make([]CCProtectionRule, len(d.config.Rules))
	copy(rules, d.config.Rules)
	d.mu.RUnlock()

	var threats []types.Threat

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if !matchCCPath(rule.Path, req.URL.Path) {
			continue
		}
		if rule.Method != "" && !strings.EqualFold(rule.Method, req.Method) {
			continue
		}

		key := buildCCKey(req, rule)
		banKey := "cc:ban:" + key
		countKey := "cc:count:" + key

		isBanned, _ := d.redisClient.Get(banKey)
		if isBanned != "" {
			threats = append(threats, types.Threat{
				Type:     "cc_protection",
				SubType:  "banned",
				Severity: "high",
				Message:  fmt.Sprintf("CC protection: IP banned by rule '%s'", rule.Name),
				RuleID:   rule.Name,
				RuleName: rule.Name,
				Details: map[string]interface{}{
					"rule":  rule.Name,
					"key":   key,
					"state": "banned",
				},
			})
			continue
		}

		count, err := d.redisClient.Incr(countKey)
		if err != nil {
			logging.DefaultLogger.Warn("CC protection: failed to increment counter for %s: %v", rule.Name, err)
			continue
		}

		if count == 1 {
			d.redisClient.Expire(countKey, time.Duration(rule.Window)*time.Second)
		}

		if count > int64(rule.Requests) {
			if rule.BanTime > 0 {
				d.redisClient.Set(banKey, "1", time.Duration(rule.BanTime)*time.Second)
			}
			threats = append(threats, types.Threat{
				Type:     "cc_protection",
				SubType:  "rate_exceeded",
				Severity: "high",
				Message:  fmt.Sprintf("CC protection: rate limit exceeded for rule '%s' (%d/%ds)", rule.Name, rule.Requests, rule.Window),
				RuleID:   rule.Name,
				RuleName: rule.Name,
				Details: map[string]interface{}{
					"rule":     rule.Name,
					"key":      key,
					"count":    count,
					"limit":    rule.Requests,
					"window":   rule.Window,
					"ban_time": rule.BanTime,
				},
			})
		}
	}

	return threats, nil
}

// UpdateRules updates the CC protection rules at runtime
func (d *CCProtectionDetector) UpdateRules(rules []CCProtectionRule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config.Rules = rules
}

// GetRules returns the current CC protection rules
func (d *CCProtectionDetector) GetRules() []CCProtectionRule {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rules := make([]CCProtectionRule, len(d.config.Rules))
	copy(rules, d.config.Rules)
	return rules
}

func buildCCKey(req *http.Request, rule CCProtectionRule) string {
	var parts []string
	parts = append(parts, rule.Name)

	for _, dim := range rule.Dimensions {
		switch {
		case dim == "ip":
			parts = append(parts, getClientIP(req))
		case strings.HasPrefix(dim, "header:"):
			headerName := strings.TrimPrefix(dim, "header:")
			parts = append(parts, req.Header.Get(headerName))
		case strings.HasPrefix(dim, "param:"):
			paramName := strings.TrimPrefix(dim, "param:")
			parts = append(parts, req.URL.Query().Get(paramName))
		case strings.HasPrefix(dim, "cookie:"):
			cookieName := strings.TrimPrefix(dim, "cookie:")
			if cookie, err := req.Cookie(cookieName); err == nil {
				parts = append(parts, cookie.Value)
			}
		case strings.HasPrefix(dim, "path"):
			parts = append(parts, req.URL.Path)
		}
	}

	raw := strings.Join(parts, ":")
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:16])
}

func matchCCPath(pattern, path string) bool {
	if pattern == "" || pattern == "/" || pattern == "/*" {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}
	return pattern == path
}
