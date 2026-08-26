package detectors

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/geoip"
	"prerender-shield/internal/logging"

	"github.com/oschwald/geoip2-golang"
)

// GeoIPDetector 地理位置访问控制检测器
type GeoIPDetector struct {
	reader      *geoip2.Reader
	apiProvider geoip.Provider
	geoIPConfig *config.GeoIPConfig
}

// NewGeoIPDetector 创建新的地理位置访问控制检测器
func NewGeoIPDetector(geoIPConfig *config.GeoIPConfig) *GeoIPDetector {
	// 处理 nil 配置，避免空指针异常
	if geoIPConfig == nil {
		geoIPConfig = &config.GeoIPConfig{Enabled: false}
	}

	var reader *geoip2.Reader
	dbPath := geoIPConfig.DatabasePath
	if dbPath == "" {
		dbPath = "./rules/GeoLite2-Country.mmdb"
	}

	r, err := geoip2.Open(dbPath)
	if err != nil {
		logging.DefaultLogger.Warn("Failed to open GeoIP database at %s: %v. Using API fallback.", dbPath, err)
	} else {
		reader = r
		logging.DefaultLogger.Info("GeoIP database loaded successfully from %s", dbPath)
	}

	// Initialize API fallback provider
	var apiProvider geoip.Provider
	provider := geoIPConfig.APIProvider
	if provider == "" {
		provider = "ip-api"
	}

	switch provider {
	case "ip-api":
		apiProvider = geoip.NewIPAPIProvider()
	case "ipinfo":
		apiProvider = geoip.NewIPInfoProvider(geoIPConfig.APIKey)
	case "ipapi-co":
		apiProvider = geoip.NewIPAPIProviderCO()
	default:
		apiProvider = geoip.NewIPAPIProvider()
	}

	return &GeoIPDetector{
		reader:      reader,
		apiProvider: apiProvider,
		geoIPConfig: geoIPConfig,
	}
}

// warnOnce 确保无本地数据库的告警只打一次（Detect 会被每请求并发调用，必须同步）
var geoIPNoDBWarned sync.Once

// Detect 检测请求的地理位置是否在允许列表中
func (d *GeoIPDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 如果地理位置访问控制未启用，直接返回
	if d.geoIPConfig == nil || !d.geoIPConfig.Enabled {
		return threats, nil
	}

	// 无本地数据库时显式告警一次：外部 API 有速率限制（如 ip-api 免费 45 次/分钟），
	// 高流量站点必须配置 MaxMind MMDB 本地库
	if d.reader == nil {
		geoIPNoDBWarned.Do(func() {
			logging.DefaultLogger.Warn("GeoIP enabled WITHOUT local MMDB — falling back to external API with strict rate limits. Configure geoip.database_path (GeoLite2-Country.mmdb) for production use")
		})
	}

	// 获取请求IP地址
	ip := getClientIP(req)
	if ip == "" {
		return threats, nil
	}

	// 获取国家/地区代码
	countryCode := "UNKNOWN"

	// 本地/私有 IP 使用 mock 模式（API 和数据库都无法查询私有 IP）
	if isLocalIP(ip) {
		countryCode = "CN"
	} else if d.reader != nil {
		ipAddr := net.ParseIP(ip)
		if ipAddr == nil {
			countryCode = "UNKNOWN"
		} else {
			record, err := d.reader.Country(ipAddr)
			if err != nil {
				logging.DefaultLogger.Warn("GeoIP lookup failed for IP %s: %v", ip, err)
				countryCode = "UNKNOWN"
			} else if record != nil && record.Country.IsoCode != "" {
				countryCode = record.Country.IsoCode
			} else {
				countryCode = "UNKNOWN"
			}
		}
	} else if d.apiProvider != nil {
		// Fallback to API provider
		result, err := d.apiProvider.Lookup(ip)
		if err != nil {
			logging.DefaultLogger.Warn("GeoIP API lookup failed for IP %s: %v", ip, err)
			countryCode = "UNKNOWN"
		} else if result != nil && result.CountryCode != "" {
			countryCode = result.CountryCode
		} else {
			countryCode = "UNKNOWN"
		}
	} else {
		// 无任何数据源可用：标记为 UNKNOWN（fail-safe）
		// BlockList 模式下 UNKNOWN 放行，AllowList 模式下 UNKNOWN 被拒
		logging.DefaultLogger.Error("GeoIP enabled but no data source available (no MMDB, no API provider) for IP %s", ip)
		countryCode = "UNKNOWN"
	}

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
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// isLocalIP 检查是否为本地/回环 IP
func isLocalIP(ip string) bool {
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP != nil && parsedIP.IsLoopback() {
		return true
	}
	return false
}
