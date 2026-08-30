package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/logging"

	"github.com/oschwald/geoip2-golang"
)

// GeoLocation 地理位置信息
type GeoLocation struct {
	Country     string  `json:"country"`      // 国家名称
	CountryCode string  `json:"country_code"` // 国家代码 (ISO 3166-1 alpha-2)
	City        string  `json:"city"`         // 城市
	Latitude    float64 `json:"latitude"`     // 纬度
	Longitude   float64 `json:"longitude"`    // 经度
}

// GeoIPResolver 定义GeoIP解析接口，便于测试Mock
type GeoIPResolver interface {
	LookupCountryISO(ip string) (string, error)
}

// GeoIPService IP地理位置解析服务
type GeoIPService struct {
	client         *http.Client
	mu             sync.RWMutex
	serverLocation *GeoLocation   // 本机地理位置（用于内网IP回退）
	cache          sync.Map       // 内存缓存 map[string]*GeoLocation (IP -> Location)
	localDB        *geoip2.Reader // MaxMind 本地数据库（可选）
}

// NewGeoIPService 创建新的GeoIP服务
// dbPath 为可选的 MaxMind GeoLite2/GeoIP2 数据库路径，若提供则优先使用本地数据库
func NewGeoIPService(dbPath string) *GeoIPService {
	service := &GeoIPService{
		client: &http.Client{
			Timeout: 5 * time.Second, // 缩短超时时间
		},
	}

	// 尝试加载本地 MaxMind 数据库
	if dbPath != "" {
		db, err := geoip2.Open(dbPath)
		if err != nil {
			logging.DefaultLogger.Warn("Failed to open MaxMind DB at %s: %v, falling back to HTTP API", dbPath, err)
		} else {
			service.localDB = db
			logging.DefaultLogger.Info("MaxMind GeoIP database loaded: %s", dbPath)
		}
	}

	// 异步初始化本机地理位置信息
	go service.initServerLocation()

	return service
}

// initServerLocation 初始化本机地理位置
func (s *GeoIPService) initServerLocation() {
	// 尝试多次获取，直到成功
	for i := 0; i < 5; i++ {
		location, err := s.fetchServerLocation()
		if err == nil && location != nil {
			s.mu.Lock()
			s.serverLocation = location
			s.mu.Unlock()
			logging.DefaultLogger.Info("Server location initialized: %s, %s", location.City, location.Country)
			return
		}
		time.Sleep(2 * time.Second)
	}
	logging.DefaultLogger.Warn("Failed to initialize server location after retries")
}

// fetchServerLocation 调用API获取本机IP位置
func (s *GeoIPService) fetchServerLocation() (*GeoLocation, error) {
	// 使用不带IP参数的API来获取请求者的IP（即本机IP）
	// ip-api.com
	url := "http://ip-api.com/json/"
	resp, err := s.client.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var result struct {
			Status      string  `json:"status"`
			Country     string  `json:"country"`
			CountryCode string  `json:"countryCode"`
			City        string  `json:"city"`
			Lat         float64 `json:"lat"`
			Lon         float64 `json:"lon"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil && result.Status == "success" {
			return &GeoLocation{
				Country:     result.Country,
				CountryCode: result.CountryCode,
				City:        result.City,
				Latitude:    result.Lat,
				Longitude:   result.Lon,
			}, nil
		}
	}

	// 备选：geojs.io
	url2 := "https://get.geojs.io/v1/ip/geo.json"
	resp2, err2 := s.client.Get(url2)
	if err2 == nil && resp2.StatusCode == http.StatusOK {
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		var result struct {
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
			City        string `json:"city"`
			Latitude    string `json:"latitude"`
			Longitude   string `json:"longitude"`
		}
		if json.Unmarshal(body, &result) == nil {
			var lat, lon float64
			fmt.Sscanf(result.Latitude, "%f", &lat)
			fmt.Sscanf(result.Longitude, "%f", &lon)
			return &GeoLocation{
				Country:     result.Country,
				CountryCode: result.CountryCode,
				City:        result.City,
				Latitude:    lat,
				Longitude:   lon,
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to fetch server location")
}

// Close 关闭GeoIP服务（清理资源）
func (s *GeoIPService) Close() error {
	if s.localDB != nil {
		return s.localDB.Close()
	}
	return nil
}

// LookupCountryISO 快速查询IP所属国家代码 (用于WAF)
func (s *GeoIPService) LookupCountryISO(ip string) (string, error) {
	// 1. 检查是否为内网IP
	if isPrivateIP(ip) {
		s.mu.RLock()
		serverLoc := s.serverLocation
		s.mu.RUnlock()
		if serverLoc != nil {
			return serverLoc.CountryCode, nil
		}
		// 如果还没初始化完，返回"Local"
		return "Local", nil
	}

	// 2. 检查缓存
	if val, ok := s.cache.Load(ip); ok {
		if loc, ok := val.(*GeoLocation); ok {
			return loc.CountryCode, nil
		}
	}

	// 3. 尝试本地 MaxMind 数据库
	if s.localDB != nil {
		if loc, err := s.queryLocalDB(ip); err == nil && loc != nil {
			s.cache.Store(ip, loc)
			return loc.CountryCode, nil
		}
	}

	// 4. 调用远程 API
	location, err := s.queryAPIWithFallback(ip)

	// 1. 检查是否为内网IP或获取位置失败的IP
	if isPrivateIP(ip) || err != nil || location == nil {
		s.mu.RLock()
		serverLoc := s.serverLocation
		s.mu.RUnlock()
		if serverLoc != nil {
			return serverLoc.CountryCode, nil
		}
		// 如果还没初始化完，返回"Local"
		return "Local", nil
	}

	// 写入缓存
	s.cache.Store(ip, location)
	return location.CountryCode, nil
}

// queryLocalDB 查询本地 MaxMind 数据库
func (s *GeoIPService) queryLocalDB(ip string) (*GeoLocation, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP: %s", ip)
	}

	record, err := s.localDB.City(parsedIP)
	if err != nil {
		return nil, fmt.Errorf("local DB lookup failed: %w", err)
	}

	if record.Country.IsoCode == "" {
		return nil, fmt.Errorf("no country data for IP: %s", ip)
	}

	return &GeoLocation{
		Country:     record.Country.Names["en"],
		CountryCode: record.Country.IsoCode,
		City:        record.City.Names["en"],
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
	}, nil
}

// GetLocation 解析IP地理位置（带重试机制，用于日志处理）
func (s *GeoIPService) GetLocation(ip string) (*GeoLocation, error) {
	// 1. 检查是否为内网IP
	if isPrivateIP(ip) {
		s.mu.RLock()
		serverLoc := s.serverLocation
		s.mu.RUnlock()
		if serverLoc != nil {
			return serverLoc, nil
		}
		return &GeoLocation{
			Country:     "Local",
			CountryCode: "Local",
			City:        "Local",
		}, nil
	}

	// 2. 检查缓存
	if val, ok := s.cache.Load(ip); ok {
		if loc, ok := val.(*GeoLocation); ok {
			return loc, nil
		}
	}

	// 3. 尝试本地 MaxMind 数据库
	if s.localDB != nil {
		if loc, err := s.queryLocalDB(ip); err == nil && loc != nil {
			s.cache.Store(ip, loc)
			return loc, nil
		}
	}

	// 4. 调用远程 API (作为回退)
	location, err := s.queryAPIWithFallback(ip)

	// 如果API查询失败，或者返回空，也回退到本机位置
	if err != nil || location == nil {
		s.mu.RLock()
		serverLoc := s.serverLocation
		s.mu.RUnlock()
		if serverLoc != nil {
			return serverLoc, nil
		}
		// queryAPIWithFallback 成功时 location 必非 nil，失败时 err 必非 nil，
		// 走到这里必然 err != nil（原「返回默认 Local」分支不可达，已删除）
		return nil, err
	}

	// 写入缓存
	s.cache.Store(ip, location)
	return location, nil
}

// queryAPIWithFallback 轮询API获取地理位置
func (s *GeoIPService) queryAPIWithFallback(ip string) (*GeoLocation, error) {
	providers := []func(string) (*GeoLocation, error){
		s.queryIPAPI,
		s.queryIPAPIco,
		s.queryGeoJS,
	}

	var lastErr error
	for _, provider := range providers {
		location, err := provider(ip)
		if err == nil {
			return location, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("all providers failed, last error: %v", lastErr)
}

// queryIPAPI 查询 ip-api.com
func (s *GeoIPService) queryIPAPI(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s", ip)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var result struct {
		Status      string  `json:"status"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		City        string  `json:"city"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("api returned fail status")
	}

	return &GeoLocation{
		Country:     result.Country,
		CountryCode: result.CountryCode,
		City:        result.City,
		Latitude:    result.Lat,
		Longitude:   result.Lon,
	}, nil
}

// queryIPAPIco 查询 ipapi.co
func (s *GeoIPService) queryIPAPIco(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "PrerenderShield/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var result struct {
		CountryName string  `json:"country_name"`
		CountryCode string  `json:"country_code"`
		City        string  `json:"city"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Error       bool    `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error {
		return nil, fmt.Errorf("api returned error")
	}

	return &GeoLocation{
		Country:     result.CountryName,
		CountryCode: result.CountryCode,
		City:        result.City,
		Latitude:    result.Latitude,
		Longitude:   result.Longitude,
	}, nil
}

// queryGeoJS 查询 get.geojs.io
func (s *GeoIPService) queryGeoJS(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("https://get.geojs.io/v1/ip/geo/%s.json", ip)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		City        string `json:"city"`
		Latitude    string `json:"latitude"`
		Longitude   string `json:"longitude"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var lat, lon float64
	fmt.Sscanf(result.Latitude, "%f", &lat)
	fmt.Sscanf(result.Longitude, "%f", &lon)

	return &GeoLocation{
		Country:     result.Country,
		CountryCode: result.CountryCode,
		City:        result.City,
		Latitude:    lat,
		Longitude:   lon,
	}, nil
}

func isPrivateIP(ipStr string) bool {
	if ipStr == "localhost" {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	privateRanges := []string{
		// IPv4 私用地址
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",     // 回环地址
		"169.254.0.0/16",  // 链路本地地址
		"100.64.0.0/10",   // CGNAT 共享地址
		"192.0.0.0/24",    // IETF 协议分配
		"198.18.0.0/15",   // 基准测试地址
		"198.51.100.0/24", // 文档地址 (TEST-NET-2)
		"203.0.113.0/24",  // 文档地址 (TEST-NET-3)
		// IPv6 私用/本地地址
		"::1/128",       // 回环地址
		"fc00::/7",      // 唯一本地地址 (ULA)
		"fe80::/10",     // 链路本地地址
		"2001:db8::/32", // 文档地址
	}
	for _, cidr := range privateRanges {
		_, ipNet, _ := net.ParseCIDR(cidr)
		if ipNet != nil && ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
