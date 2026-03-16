package threatintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// VirusTotalProvider VirusTotal 威胁情报提供者
type VirusTotalProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	enabled    bool
}

// NewVirusTotalProvider 创建 VirusTotal 提供者
func NewVirusTotalProvider(apiKey string, httpClient *http.Client) *VirusTotalProvider {
	return &VirusTotalProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://www.virustotal.com/api/v3",
		enabled:    apiKey != "",
	}
}

// Name 返回提供者名称
func (p *VirusTotalProvider) Name() string {
	return "virustotal"
}

// IsEnabled 返回是否启用
func (p *VirusTotalProvider) IsEnabled() bool {
	return p.enabled
}

// QueryIP 查询 IP 信誉
func (p *VirusTotalProvider) QueryIP(ctx context.Context, ip string) (*ThreatIntelResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("VirusTotal 未启用")
	}

	reqURL := fmt.Sprintf("%s/ip_addresses/%s", p.baseURL, url.PathEscape(ip))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("x-apikey", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 VirusTotal 失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VirusTotal API 返回错误状态：%d", resp.StatusCode)
	}

	return p.parseIPResponse(body, ip)
}

// QueryDomain 查询域名信誉
func (p *VirusTotalProvider) QueryDomain(ctx context.Context, domain string) (*ThreatIntelResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("VirusTotal 未启用")
	}

	reqURL := fmt.Sprintf("%s/domains/%s", p.baseURL, url.PathEscape(domain))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("x-apikey", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 VirusTotal 失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VirusTotal API 返回错误状态：%d", resp.StatusCode)
	}

	return p.parseDomainResponse(body, domain)
}

// parseIPResponse 解析 IP 响应
func (p *VirusTotalProvider) parseIPResponse(body []byte, ip string) (*ThreatIntelResult, error) {
	var response struct {
		Data struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Links struct {
				Self string `json:"self"`
			} `json:"links"`
			Attributes struct {
				LastAnalysisStats struct {
					Harmless   int `json:"harmless"`
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Timeout    int `json:"timeout"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
				LastAnalysisResults map[string]struct {
					Category   string `json:"category"`
					Result     string `json:"result"`
					Method     string `json:"method"`
					EngineName string `json:"engine_name"`
				} `json:"last_analysis_results"`
				Reputation int `json:"reputation"`
				TotalVotes struct {
					Harmless  int `json:"harmless"`
					Malicious int `json:"malicious"`
				} `json:"total_votes"`
				Country string `json:"country"`
				ASOwner string `json:"as_owner"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	stats := response.Data.Attributes.LastAnalysisStats
	total := stats.Harmless + stats.Malicious + stats.Suspicious + stats.Undetected

	var riskScore int
	if total > 0 {
		riskScore = (stats.Malicious*100 + stats.Suspicious*50) / total
	}

	categories := make([]string, 0)
	if stats.Malicious > 0 {
		categories = append(categories, "malicious")
	}
	if stats.Suspicious > 0 {
		categories = append(categories, "suspicious")
	}

	result := &ThreatIntelResult{
		IP:          ip,
		IsMalicious: stats.Malicious > 0,
		Confidence:  float64(stats.Malicious+stats.Suspicious) / float64(total) * 100,
		RiskScore:   riskScore,
		Categories:  categories,
		Provider:    "virustotal",
		RawData: map[string]interface{}{
			"reputation":     response.Data.Attributes.Reputation,
			"country":        response.Data.Attributes.Country,
			"as_owner":       response.Data.Attributes.ASOwner,
			"total_votes":    response.Data.Attributes.TotalVotes,
			"analysis_stats": stats,
		},
	}

	return result, nil
}

// parseDomainResponse 解析域名响应
func (p *VirusTotalProvider) parseDomainResponse(body []byte, domain string) (*ThreatIntelResult, error) {
	var response struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				LastAnalysisStats struct {
					Harmless   int `json:"harmless"`
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Timeout    int `json:"timeout"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
				LastAnalysisResults map[string]struct {
					Category   string `json:"category"`
					Result     string `json:"result"`
					Method     string `json:"method"`
					EngineName string `json:"engine_name"`
				} `json:"last_analysis_results"`
				Reputation int `json:"reputation"`
				TotalVotes struct {
					Harmless  int `json:"harmless"`
					Malicious int `json:"malicious"`
				} `json:"total_votes"`
				CreationDate   int64 `json:"creation_date"`
				LastUpdateDate int64 `json:"last_update_date"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	stats := response.Data.Attributes.LastAnalysisStats
	total := stats.Harmless + stats.Malicious + stats.Suspicious + stats.Undetected

	var riskScore int
	if total > 0 {
		riskScore = (stats.Malicious*100 + stats.Suspicious*50) / total
	}

	categories := make([]string, 0)
	if stats.Malicious > 0 {
		categories = append(categories, "malicious")
	}
	if stats.Suspicious > 0 {
		categories = append(categories, "suspicious")
	}

	result := &ThreatIntelResult{
		Domain:      domain,
		IsMalicious: stats.Malicious > 0,
		Confidence:  float64(stats.Malicious+stats.Suspicious) / float64(total) * 100,
		RiskScore:   riskScore,
		Categories:  categories,
		Provider:    "virustotal",
		RawData: map[string]interface{}{
			"reputation":     response.Data.Attributes.Reputation,
			"creation_date":  response.Data.Attributes.CreationDate,
			"total_votes":    response.Data.Attributes.TotalVotes,
			"analysis_stats": stats,
		},
	}

	return result, nil
}
