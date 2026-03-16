package threatintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// AbuseIPDBProvider AbuseIPDB 威胁情报提供者
type AbuseIPDBProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	enabled    bool
}

// NewAbuseIPDBProvider 创建 AbuseIPDB 提供者
func NewAbuseIPDBProvider(apiKey string, httpClient *http.Client) *AbuseIPDBProvider {
	return &AbuseIPDBProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://api.abuseipdb.com/api/v2",
		enabled:    apiKey != "",
	}
}

// Name 返回提供者名称
func (p *AbuseIPDBProvider) Name() string {
	return "abuseipdb"
}

// IsEnabled 返回是否启用
func (p *AbuseIPDBProvider) IsEnabled() bool {
	return p.enabled
}

// QueryIP 查询 IP 滥用记录
func (p *AbuseIPDBProvider) QueryIP(ctx context.Context, ip string) (*ThreatIntelResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("AbuseIPDB 未启用")
	}

	reqURL := fmt.Sprintf("%s/check?ipAddress=%s&maxAgeInDays=90", p.baseURL, url.PathEscape(ip))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("Key", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 AbuseIPDB 失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AbuseIPDB API 返回错误状态：%d", resp.StatusCode)
	}

	return p.parseResponse(body, ip)
}

// QueryDomain AbuseIPDB 不支持域名查询
func (p *AbuseIPDBProvider) QueryDomain(ctx context.Context, domain string) (*ThreatIntelResult, error) {
	return nil, fmt.Errorf("AbuseIPDB 不支持域名查询")
}

// parseResponse 解析响应
func (p *AbuseIPDBProvider) parseResponse(body []byte, ip string) (*ThreatIntelResult, error) {
	var response struct {
		Data struct {
			IPAddress             string            `json:"ipAddress"`
			IsPublic              bool              `json:"isPublic"`
			IPVersion             int               `json:"ipVersion"`
			IsWhitelisted         bool              `json:"isWhitelisted"`
			AbuseScore            int               `json:"abuseScore"`
			AbuseConfidence       int               `json:"abuseConfidence"`
			CountryCode           string            `json:"countryCode"`
			UsageType             string            `json:"usageType"`
			ISP                   string            `json:"isp"`
			Domain                string            `json:"domain"`
			Hostnames             []string          `json:"hostnames"`
			IsTor                 bool              `json:"isTor"`
			TotalReports          int               `json:"totalReports"`
			LastReportedAt        string            `json:"lastReportedAt"`
			Reports               []Report          `json:"reports"`
			Categories            map[string]string `json:"categories"`
			DistinctUsers         int               `json:"distinctUsers"`
			DistinctUserCountries map[string]int    `json:"distinctUserCountries"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	data := response.Data

	// 根据滥用置信度计算风险评分
	riskScore := data.AbuseConfidence
	if data.IsTor {
		riskScore += 20 // Tor 出口节点额外加分
	}
	if riskScore > 100 {
		riskScore = 100
	}

	// 确定类别
	categories := make([]string, 0)
	if data.AbuseConfidence >= 80 {
		categories = append(categories, "high_abuse")
	} else if data.AbuseConfidence >= 50 {
		categories = append(categories, "medium_abuse")
	} else if data.AbuseConfidence >= 20 {
		categories = append(categories, "low_abuse")
	}

	if data.IsTor {
		categories = append(categories, "tor_exit_node")
	}

	// 根据使用类型添加类别
	switch data.UsageType {
	case "Data Center/Web Hosting/Transit":
		categories = append(categories, "hosting")
	case "Fixed Line ISP":
		categories = append(categories, "isp")
	case "Commercial":
		categories = append(categories, "commercial")
	}

	// 解析类别映射
	categoryLabels := make([]string, 0)
	for _, cat := range data.Reports {
		for _, catID := range cat.Categories {
			if label, ok := data.Categories[strconv.Itoa(catID)]; ok {
				categoryLabels = append(categoryLabels, label)
			}
		}
	}

	result := &ThreatIntelResult{
		IP:          ip,
		IsMalicious: data.AbuseConfidence >= 80 || data.IsTor,
		Confidence:  float64(data.AbuseConfidence),
		RiskScore:   riskScore,
		Categories:  append(categories, categoryLabels...),
		Provider:    "abuseipdb",
		RawData: map[string]interface{}{
			"is_public":        data.IsPublic,
			"is_whitelisted":   data.IsWhitelisted,
			"abuse_score":      data.AbuseScore,
			"abuse_confidence": data.AbuseConfidence,
			"country_code":     data.CountryCode,
			"usage_type":       data.UsageType,
			"isp":              data.ISP,
			"domain":           data.Domain,
			"is_tor":           data.IsTor,
			"total_reports":    data.TotalReports,
			"distinct_users":   data.DistinctUsers,
			"last_reported":    data.LastReportedAt,
			"hostnames":        data.Hostnames,
		},
	}

	return result, nil
}

// Report 滥用报告
type Report struct {
	ReportedAt  string `json:"reportedAt"`
	Categories  []int  `json:"categories"`
	Comment     string `json:"comment"`
	ReporterID  int    `json:"reporterId"`
	IsPublic    bool   `json:"isPublic"`
	IsAutomated bool   `json:"isAutomated"`
}
