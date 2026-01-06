package detectors

import (
	"context"
	"fmt"
	"net/http"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"

	"github.com/go-redis/redis/v8"
)

// BlacklistDetector 黑白名单检测器
type BlacklistDetector struct {
	redisClient *redis.Client
	siteID      string
	blacklist   []string // 静态黑名单
	whitelist   []string // 静态白名单
}

// NewBlacklistDetector 创建黑白名单检测器
func NewBlacklistDetector(redisClient *redis.Client, siteID string, blacklist, whitelist []string) *BlacklistDetector {
	return &BlacklistDetector{
		redisClient: redisClient,
		siteID:      siteID,
		blacklist:   blacklist,
		whitelist:   whitelist,
	}
}

// Name 返回检测器名称
func (d *BlacklistDetector) Name() string {
	return "blacklist"
}

// Detect 检测请求
func (d *BlacklistDetector) Detect(req *http.Request) ([]types.Threat, error) {
	ip := logging.GetClientIP(req)
	
	// 1. 检查静态白名单
	for _, allowed := range d.whitelist {
		if allowed == ip {
			return nil, nil // 放行
		}
	}
	
	// 2. 检查静态黑名单
	for _, blocked := range d.blacklist {
		if blocked == ip {
			return []types.Threat{{
				Type:     "blacklist",
				Message:  fmt.Sprintf("IP %s matches static blacklist", ip),
				Severity: "critical",
				Details:  map[string]interface{}{"ip": ip, "source": "static"},
			}}, nil
		}
	}
	
	// 3. 检查动态黑名单 (Redis)
	if d.redisClient != nil {
		key := fmt.Sprintf("firewall:%s:blacklist", d.siteID)
		isMember, err := d.redisClient.SIsMember(context.Background(), key, ip).Result()
		if err != nil {
			// Redis错误，记录但不中断（默认放行或阻止？WAF通常fail open）
			return nil, err
		}
		if isMember {
			return []types.Threat{{
				Type:     "blacklist",
				Message:  fmt.Sprintf("IP %s matches dynamic blacklist", ip),
				Severity: "critical",
				Details:  map[string]interface{}{"ip": ip, "source": "dynamic"},
			}}, nil
		}
	}
	
	return nil, nil
}
