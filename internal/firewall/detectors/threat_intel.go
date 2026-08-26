package detectors

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// ThreatIntelRedisClient defines the interface for Redis operations needed by threat intel
type ThreatIntelRedisClient interface {
	SetContains(key string, member interface{}) (bool, error)
	Members(key string) ([]string, error)
}

// ThreatIntelConfig holds the threat intelligence detector configuration
type ThreatIntelConfig struct {
	Enabled   bool
	GlobalKey string
}

// ThreatIntelDetector checks IPs against threat intelligence blacklists
type ThreatIntelDetector struct {
	config      *ThreatIntelConfig
	redisClient ThreatIntelRedisClient
}

// NewThreatIntelDetector creates a new threat intelligence detector
func NewThreatIntelDetector(config *ThreatIntelConfig, redisClient ThreatIntelRedisClient) *ThreatIntelDetector {
	return &ThreatIntelDetector{
		config:      config,
		redisClient: redisClient,
	}
}

// Name returns the detector name
func (d *ThreatIntelDetector) Name() string {
	return "threat_intel"
}

// Detect checks if the request IP is in the threat intelligence blacklist
func (d *ThreatIntelDetector) Detect(req *http.Request) ([]types.Threat, error) {
	if d.config == nil || !d.config.Enabled || d.redisClient == nil {
		return nil, nil
	}

	ip := getClientIP(req)
	if ip == "" {
		return nil, nil
	}

	globalKey := d.config.GlobalKey
	if globalKey == "" {
		globalKey = "threatintel:global:blacklist"
	}

	// Check exact IP match first
	isMember, err := d.redisClient.SetContains(globalKey, ip)
	if err != nil {
		logging.DefaultLogger.Warn("ThreatIntel: failed to check IP %s: %v", ip, err)
		return nil, nil
	}

	if isMember {
		return []types.Threat{{
			Type:     "threat_intel",
			SubType:  "known_malicious_ip",
			Severity: "critical",
			Message:  fmt.Sprintf("IP %s found in threat intelligence blacklist", ip),
			SourceIP: ip,
			Details: map[string]interface{}{
				"source": "threat_intelligence",
				"ip":     ip,
			},
		}}, nil
	}

	// Check CIDR matching
	if d.matchCIDR(ip, globalKey) {
		return []types.Threat{{
			Type:     "threat_intel",
			SubType:  "known_malicious_cidr",
			Severity: "critical",
			Message:  fmt.Sprintf("IP %s matches CIDR in threat intelligence blacklist", ip),
			SourceIP: ip,
			Details: map[string]interface{}{
				"source": "threat_intelligence",
				"ip":     ip,
			},
		}}, nil
	}

	return nil, nil
}

// matchCIDR checks if an IP matches any CIDR ranges in the blacklist
func (d *ThreatIntelDetector) matchCIDR(ipStr string, key string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Get all members from the blacklist (includes CIDR entries)
	members, err := d.redisClient.Members(key)
	if err != nil {
		return false
	}

	for _, member := range members {
		if strings.Contains(member, "/") {
			_, cidr, err := net.ParseCIDR(member)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return true
			}
		}
	}

	return false
}
