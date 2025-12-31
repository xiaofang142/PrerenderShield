package detectors

import (
	"net/http"
	"strings"

	"github.com/oschwald/geoip2-golang"
	"github.com/prerendershield/internal/config"
	"github.com/prerendershield/internal/firewall/types"
)

// GeoIPDetector 地理位置访问控制检测器
type GeoIPDetector struct {
	reader      *geoip2.Reader
	geoIPConfig *config.GeoIPConfig
}

// NewGeoIPDetector 创建新的地理位置访问控制检测器
func NewGeoIPDetector(geoIPConfig *config.GeoIPConfig) *GeoIPDetector {
	// 这里应该加载GeoIP数据库，暂时返回一个空实例
	return &GeoIPDetector{
		geoIPConfig: geoIPConfig,
	}
}

// Detect 检测请求的地理位置是否在允许列表中
func (d *GeoIPDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 如果地理位置访问控制未启用，直接返回
	if d.geoIPConfig == nil || !d.geoIPConfig.Enabled {
		return threats, nil
	}

	// 获取请求IP地址
	ip := getClientIP(req)
	if ip == "" {
		return threats, nil
	}

	// 这里应该查询GeoIP数据库获取国家/地区代码
	// 暂时模拟返回"CN"（中国）
	countryCode := "CN"

	// 检查是否在阻止列表中
	if len(d.geoIPConfig.BlockList) > 0 {
		for _, blockCode := range d.geoIPConfig.BlockList {
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
				return threats, nil
			}
		}
	}

	// 检查是否在允许列表中（如果允许列表不为空）
	if len(d.geoIPConfig.AllowList) > 0 {
		allowFound := false
		for _, allowCode := range d.geoIPConfig.AllowList {
			if countryCode == allowCode {
				allowFound = true
				break
			}
		}
		if !allowFound {
			threats = append(threats, types.Threat{
				Type:     "geoip",
				SubType:  "country_allow",
				Severity: "high",
				Message:  "Request from country not in allow list",
				SourceIP: ip,
				Details: map[string]interface{}{
					"country": countryCode,
				},
			})
			return threats, nil
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
