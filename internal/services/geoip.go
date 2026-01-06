package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"prerender-shield/internal/logging"
)

// GeoLocation 地理位置信息
type GeoLocation struct {
	Country     string  `json:"country"`      // 国家名称
	CountryCode string  `json:"country_code"` // 国家代码 (ISO 3166-1 alpha-2)
	City        string  `json:"city"`         // 城市
	Latitude    float64 `json:"latitude"`     // 纬度
	Longitude   float64 `json:"longitude"`    // 经度
}

// GeoIPService IP地理位置解析服务
type GeoIPService struct {
	client *http.Client
}

// NewGeoIPService 创建新的GeoIP服务
func NewGeoIPService() *GeoIPService {
	return &GeoIPService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetLocation 解析IP地理位置（带重试机制）
func (s *GeoIPService) GetLocation(ip string) (*GeoLocation, error) {
	// 如果是内网IP，直接返回
	if isPrivateIP(ip) {
		return &GeoLocation{
			Country:     "Local",
			CountryCode: "Local",
			City:        "Local",
		}, nil
	}

	// 尝试多个API提供商
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
		logging.DefaultLogger.Warn("GeoIP provider failed for IP %s: %v", ip, err)
		// 短暂休眠避免并发过快
		time.Sleep(500 * time.Millisecond)
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
	req.Header.Set("User-Agent", "PrerenderShield/1.0") // ipapi.co requires UA

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
	// geojs returns string latitude/longitude sometimes? Check docs.
	// Assuming standard JSON.
	
	// Create a custom struct to handle string/float parsing if needed, 
	// but standard unmarshal handles string numbers to float if using json.Number or string tag? No.
	// Let's assume it returns strings for lat/lon as it often does.
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

// isPrivateIP 简单判断是否为内网IP
func isPrivateIP(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost" || 
		(len(ip) >= 3 && ip[:3] == "10.") ||
		(len(ip) >= 7 && ip[:7] == "192.168") ||
		(len(ip) >= 4 && ip[:4] == "172.")
}