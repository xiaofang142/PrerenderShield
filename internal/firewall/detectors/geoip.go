package detectors

import (
	"net/http"
	"strings"

	"github.com/oschwald/geoip2-golang"
	"github.com/prerendershield/internal/firewall/types"
)

// GeoIPDetector 地理位置访问控制检测器
type GeoIPDetector struct {
	reader *geoip2.Reader
}

// NewGeoIPDetector 创建新的地理位置访问控制检测器
func NewGeoIPDetector() *GeoIPDetector {
	// 这里应该加载GeoIP数据库，暂时返回一个空实例
	return &GeoIPDetector{}
}

// Detect 检测请求的地理位置是否在允许列表中
func (d *GeoIPDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 获取请求IP地址
	ip := getClientIP(req)
	if ip == "" {
		return threats, nil
	}

	// 这里应该查询GeoIP数据库获取国家/地区代码
	// 暂时模拟返回"CN"（中国）
	countryCode := "CN"

	// 检查是否在阻止列表中
	// 注意：实际实现中，应该从配置中获取allow_list和block_list
	// 这里只是一个示例
	blockList := []string{"US", "JP"} // 模拟阻止美国和日本的请求
	for _, blockCode := range blockList {
		if countryCode == blockCode {
			threats = append(threats, types.Threat{
				Type:     "geoip",
				SubType:  "country_block",
				Severity: "high",
				Message:  "Request from blocked country",
				SourceIP: ip,
				Details: map[string]interface{}{
					"country": countryCode,
				},
			})
			break
		}
	}

	return threats, nil
}

// Name 返回检测器名称
func (d *GeoIPDetector) Name() string {
	return "geoip"
}

// getClientIP 获取客户端真实IP地址
func getClientIP(req *http.Request) string {
	// 首先检查X-Forwarded-For头
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For格式：client, proxy1, proxy2
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 检查X-Real-IP头
	if xrip := req.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// 直接使用RemoteAddr
	remoteAddr := req.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}
