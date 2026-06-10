package constants

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisKeys(t *testing.T) {
	assert.Equal(t, "prerender:firewall:rules", RedisKeyFirewallRules)
	assert.Equal(t, "prerender:config:sites", RedisKeyConfigSites)
	assert.Equal(t, "prerender:cache:", RedisKeyCachePrefix)
}

func TestDefaultTimings(t *testing.T) {
	assert.Equal(t, 5*time.Second, DefaultConfigCheckInterval)
	assert.Equal(t, 30*time.Second, DefaultPrerenderTimeout)
	assert.Equal(t, 3600*time.Second, DefaultCacheTTL)
	assert.Equal(t, 24*time.Hour, DefaultRuleUpdateInterval)
}

func TestDefaultPorts(t *testing.T) {
	assert.Equal(t, "0.0.0.0", DefaultServerAddress)
	assert.Equal(t, 9598, DefaultAPIPort)
	assert.Equal(t, 9597, DefaultConsolePort)
}

func TestDefaultCache(t *testing.T) {
	assert.Equal(t, "memory", DefaultCacheType)
	assert.Equal(t, 1000, DefaultCacheMemorySize)
}

func TestDefaultFirewall(t *testing.T) {
	assert.Equal(t, "block", DefaultFirewallAction)
	assert.Equal(t, 100, DefaultRateLimitRequests)
}
