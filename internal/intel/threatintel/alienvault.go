package threatintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// AlienVaultProvider AlienVault OTX 威胁情报提供者
type AlienVaultProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	enabled    bool
}

// NewAlienVaultProvider 创建 AlienVault 提供者
func NewAlienVaultProvider(apiKey string, httpClient *http.Client) *AlienVaultProvider {
	return &AlienVaultProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://otx.alienvault.com/api/v1",
		enabled:    apiKey != "",
	}
}

// Name 返回提供者名称
func (p *AlienVaultProvider) Name() string {
	return "alienvault"
}

// IsEnabled 返回是否启用
func (p *AlienVaultProvider) IsEnabled() bool {
	return p.enabled
}

// QueryIP 查询 IP 威胁情报
func (p *AlienVaultProvider) QueryIP(ctx context.Context, ip string) (*ThreatIntelResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("AlienVault 未启用")
	}

	reqURL := fmt.Sprintf("%s/indicators/IP/%s/general", p.baseURL, url.PathEscape(ip))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("X-OTX-API-KEY", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 AlienVault 失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AlienVault API 返回错误状态：%d", resp.StatusCode)
	}

	return p.parseIPResponse(body, ip)
}

// QueryDomain 查询域名威胁情报
func (p *AlienVaultProvider) QueryDomain(ctx context.Context, domain string) (*ThreatIntelResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("AlienVault 未启用")
	}

	reqURL := fmt.Sprintf("%s/indicators/domain/%s/general", p.baseURL, url.PathEscape(domain))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("X-OTX-API-KEY", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 AlienVault 失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AlienVault API 返回错误状态：%d", resp.StatusCode)
	}

	return p.parseDomainResponse(body, domain)
}

// parseIPResponse 解析 IP 响应
func (p *AlienVaultProvider) parseIPResponse(body []byte, ip string) (*ThreatIntelResult, error) {
	var response struct {
		Indicator  string `json:"indicator"`
		Name       string `json:"name"`
		TLP        string `json:"tlp"`
		Access     string `json:"access"`
		AuthorName string `json:"author_name"`
		Creator    string `json:"creator"`
		IsPublic   bool   `json:"is_public"`
		Validation struct {
			Access   string `json:"access"`
			IsPublic bool   `json:"is_public"`
		} `json:"validation"`
		PulseInfo struct {
			Count         int     `json:"count"`
			TopPulses     []Pulse `json:"top_pulses"`
			RelatedPulses []Pulse `json:"related_pulses"`
		} `json:"pulse_info"`
		Asn         string  `json:"asn"`
		CountryName string  `json:"country_name"`
		City        string  `json:"city"`
		Region      string  `json:"region"`
		PostalCode  string  `json:"postal_code"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		IsMalicious *bool   `json:"is_malicious"`
		ThreatTypes []struct {
			ThreatType string `json:"threat_type"`
			Count      int    `json:"count"`
		} `json:"threat_types"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	// 计算风险评分
	pulseCount := response.PulseInfo.Count
	var riskScore int
	if pulseCount > 0 {
		if pulseCount > 10 {
			riskScore = 100
		} else {
			riskScore = pulseCount * 10
		}
	}

	// 判断是否恶意
	isMalicious := false
	if response.IsMalicious != nil && *response.IsMalicious {
		isMalicious = true
	}
	if pulseCount >= 5 {
		isMalicious = true
	}

	// 提取威胁类型
	categories := make([]string, 0)
	for _, tt := range response.ThreatTypes {
		if tt.Count > 0 {
			categories = append(categories, tt.ThreatType)
		}
	}

	confidence := 0.0
	if pulseCount > 0 {
		confidence = float64(pulseCount) / 10 * 100
		if confidence > 100 {
			confidence = 100
		}
	}

	result := &ThreatIntelResult{
		IP:          ip,
		IsMalicious: isMalicious,
		Confidence:  confidence,
		RiskScore:   riskScore,
		Categories:  categories,
		Provider:    "alienvault",
		RawData: map[string]interface{}{
			"name":         response.Name,
			"tlp":          response.TLP,
			"author_name":  response.AuthorName,
			"asn":          response.Asn,
			"country_name": response.CountryName,
			"city":         response.City,
			"region":       response.Region,
			"pulse_count":  pulseCount,
			"top_pulses":   response.PulseInfo.TopPulses,
			"latitude":     response.Latitude,
			"longitude":    response.Longitude,
		},
	}

	return result, nil
}

// parseDomainResponse 解析域名响应
func (p *AlienVaultProvider) parseDomainResponse(body []byte, domain string) (*ThreatIntelResult, error) {
	var response struct {
		Indicator string `json:"indicator"`
		Name      string `json:"name"`
		TLP       string `json:"tlp"`
		PulseInfo struct {
			Count     int     `json:"count"`
			TopPulses []Pulse `json:"top_pulses"`
		} `json:"pulse_info"`
		IsMalicious *bool `json:"is_malicious"`
		ThreatTypes []struct {
			ThreatType string `json:"threat_type"`
			Count      int    `json:"count"`
		} `json:"threat_types"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	// 计算风险评分
	pulseCount := response.PulseInfo.Count
	var riskScore int
	if pulseCount > 0 {
		if pulseCount > 10 {
			riskScore = 100
		} else {
			riskScore = pulseCount * 10
		}
	}

	// 判断是否恶意
	isMalicious := false
	if response.IsMalicious != nil && *response.IsMalicious {
		isMalicious = true
	}
	if pulseCount >= 5 {
		isMalicious = true
	}

	// 提取威胁类型
	categories := make([]string, 0)
	for _, tt := range response.ThreatTypes {
		if tt.Count > 0 {
			categories = append(categories, tt.ThreatType)
		}
	}

	confidence := 0.0
	if pulseCount > 0 {
		confidence = float64(pulseCount) / 10 * 100
		if confidence > 100 {
			confidence = 100
		}
	}

	result := &ThreatIntelResult{
		Domain:      domain,
		IsMalicious: isMalicious,
		Confidence:  confidence,
		RiskScore:   riskScore,
		Categories:  categories,
		Provider:    "alienvault",
		RawData: map[string]interface{}{
			"name":        response.Name,
			"tlp":         response.TLP,
			"pulse_count": pulseCount,
			"top_pulses":  response.PulseInfo.TopPulses,
		},
	}

	return result, nil
}

// Pulse OTX Pulse 信息
type Pulse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AuthorName  string   `json:"author_name"`
	Created     string   `json:"created"`
	Modified    string   `json:"modified"`
	TLP         string   `json:"tlp"`
	Tags        []string `json:"tags"`
	Votes       struct {
		Count  int `json:"count"`
		MyVote int `json:"my_vote"`
	} `json:"votes"`
	Indicators []struct {
		Indicator   string `json:"indicator"`
		Type        string `json:"type"`
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"indicators"`
}
