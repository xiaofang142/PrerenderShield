// Package threatintel provides automated threat intelligence IP blacklist fetching.
// Supports multiple free threat intelligence sources with configurable update intervals.
package threatintel

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
)

// Source represents a threat intelligence data source
type Source struct {
	Name           string        `json:"name" yaml:"name"`
	URL            string        `json:"url" yaml:"url"`
	Format         string        `json:"format" yaml:"format"` // csv, text, json
	UpdateInterval time.Duration `json:"update_interval" yaml:"update_interval"`
	Enabled        bool          `json:"enabled" yaml:"enabled"`
	IPField        string        `json:"ip_field" yaml:"ip_field"` // for CSV/JSON: field name containing IP
}

// Config holds the threat intelligence configuration
type Config struct {
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Sources     []Source `json:"sources" yaml:"sources"`
	GlobalKey   string   `json:"global_key" yaml:"global_key"`   // Redis key for global blacklist
	MaxIPs      int      `json:"max_ips" yaml:"max_ips"`         // max IPs per source
	Concurrency int      `json:"concurrency" yaml:"concurrency"` // fetch concurrency
}

// DefaultConfig returns the default threat intelligence configuration
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		GlobalKey:   "threatintel:global:blacklist",
		MaxIPs:      50000,
		Concurrency: 3,
		Sources: []Source{
			{
				Name:           "Abuse.ch Feodo Tracker",
				URL:            "https://feodotracker.abuse.ch/downloads/ipblocklist.csv",
				Format:         "csv",
				UpdateInterval: 1 * time.Hour,
				Enabled:        false,
				IPField:        "dst_ip",
			},
			{
				Name:           "Blocklist.de All",
				URL:            "https://lists.blocklist.de/lists/all.txt",
				Format:         "text",
				UpdateInterval: 6 * time.Hour,
				Enabled:        false,
			},
			{
				Name:           "Emerging Threats Compromised IPs",
				URL:            "https://rules.emergingthreats.net/fwrules/emerging-Block-IPs.txt",
				Format:         "text",
				UpdateInterval: 12 * time.Hour,
				Enabled:        false,
			},
			{
				Name:           "Spamhaus DROP List",
				URL:            "https://www.spamhaus.org/drop/drop.txt",
				Format:         "text",
				UpdateInterval: 12 * time.Hour,
				Enabled:        false,
			},
			{
				Name:           "CINS Army Threat List",
				URL:            "https://cinsscore.com/list/ci-badguys.txt",
				Format:         "text",
				UpdateInterval: 6 * time.Hour,
				Enabled:        false,
			},
		},
	}
}

// Fetcher handles threat intelligence IP fetching and storage
type Fetcher struct {
	config      Config
	redisClient *redis.Client
	httpClient  *http.Client
	stopChan    chan struct{}
	mu          sync.Mutex
	stats       map[string]*SourceStats
}

// SourceStats tracks statistics per source
type SourceStats struct {
	Name        string    `json:"name"`
	LastFetch   time.Time `json:"last_fetch"`
	LastSuccess time.Time `json:"last_success"`
	IPCount     int       `json:"ip_count"`
	FetchCount  int64     `json:"fetch_count"`
	ErrorCount  int64     `json:"error_count"`
	LastError   string    `json:"last_error,omitempty"`
}

// NewFetcher creates a new threat intelligence fetcher
func NewFetcher(config Config, redisClient *redis.Client) *Fetcher {
	return &Fetcher{
		config:      config,
		redisClient: redisClient,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		stopChan:    make(chan struct{}),
		stats:       make(map[string]*SourceStats),
	}
}

// MergeConfig 将另一份配置的来源并集合并进当前 Fetcher。
// 用于多站点场景：威胁情报是全局数据源，任一站点启用即应拉取其选择的源
func (f *Fetcher) MergeConfig(other Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing := make(map[string]bool, len(f.config.Sources))
	for _, src := range f.config.Sources {
		existing[src.Name] = true
	}
	for _, src := range other.Sources {
		if !existing[src.Name] {
			f.config.Sources = append(f.config.Sources, src)
		}
	}
	if other.MaxIPs > f.config.MaxIPs {
		f.config.MaxIPs = other.MaxIPs
	}
	if other.Concurrency > f.config.Concurrency {
		f.config.Concurrency = other.Concurrency
	}
}

// Start begins periodic threat intelligence fetching
func (f *Fetcher) Start() {
	if !f.config.Enabled {
		logging.DefaultLogger.Info("Threat intelligence fetching is disabled")
		return
	}

	logging.DefaultLogger.Info("Threat intelligence fetcher started with %d sources", len(f.config.Sources))

	// Initial fetch for all enabled sources
	go f.fetchAll()

	// Schedule periodic updates per source
	for _, source := range f.config.Sources {
		if !source.Enabled {
			continue
		}
		go f.scheduleSource(source)
	}
}

// Stop stops the fetcher
func (f *Fetcher) Stop() {
	close(f.stopChan)
	logging.DefaultLogger.Info("Threat intelligence fetcher stopped")
}

func (f *Fetcher) scheduleSource(source Source) {
	ticker := time.NewTicker(source.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopChan:
			return
		case <-ticker.C:
			f.fetchSource(source)
		}
	}
}

func (f *Fetcher) fetchAll() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.config.Concurrency)

	for _, source := range f.config.Sources {
		if !source.Enabled {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(s Source) {
			defer wg.Done()
			defer func() { <-sem }()
			f.fetchSource(s)
		}(source)
	}
	wg.Wait()
}

func (f *Fetcher) fetchSource(source Source) {
	f.mu.Lock()
	if _, ok := f.stats[source.Name]; !ok {
		f.stats[source.Name] = &SourceStats{Name: source.Name}
	}
	stats := f.stats[source.Name]
	stats.FetchCount++
	stats.LastFetch = time.Now()
	f.mu.Unlock()

	logging.DefaultLogger.Info("Fetching threat intelligence from %s: %s", source.Name, source.URL)

	ips, err := f.downloadAndParse(source)
	if err != nil {
		f.mu.Lock()
		stats.ErrorCount++
		stats.LastError = err.Error()
		f.mu.Unlock()
		logging.DefaultLogger.Error("Failed to fetch from %s: %v", source.Name, err)
		return
	}

	if len(ips) == 0 {
		logging.DefaultLogger.Warn("No IPs found from source: %s", source.Name)
		return
	}

	// Store IPs in Redis
	sourceKey := fmt.Sprintf("threatintel:source:%s", f.sanitizeKey(source.Name))
	if err := f.storeIPs(sourceKey, ips); err != nil {
		f.mu.Lock()
		stats.ErrorCount++
		stats.LastError = err.Error()
		f.mu.Unlock()
		logging.DefaultLogger.Error("Failed to store IPs from %s: %v", source.Name, err)
		return
	}

	// Also add to global blacklist
	if err := f.storeIPs(f.config.GlobalKey, ips); err != nil {
		logging.DefaultLogger.Error("Failed to store IPs to global blacklist: %v", err)
	}

	f.mu.Lock()
	stats.LastSuccess = time.Now()
	stats.IPCount = len(ips)
	stats.LastError = ""
	f.mu.Unlock()

	logging.DefaultLogger.Info("Fetched %d IPs from %s", len(ips), source.Name)
}

func (f *Fetcher) downloadAndParse(source Source) ([]string, error) {
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "PrerenderShield-ThreatIntel/1.0")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	switch source.Format {
	case "csv":
		return f.parseCSV(resp.Body, source.IPField)
	case "text":
		return f.parseText(resp.Body)
	case "json":
		return f.parseJSON(resp.Body, source.IPField)
	default:
		return f.parseText(resp.Body)
	}
}

func (f *Fetcher) parseCSV(r io.Reader, ipField string) ([]string, error) {
	reader := csv.NewReader(r)
	reader.Comment = '#'
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	ipColIdx := -1
	for i, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(ipField)) {
			ipColIdx = i
			break
		}
	}
	if ipColIdx < 0 {
		// Try first column as fallback
		ipColIdx = 0
	}

	ips := make(map[string]bool)
	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if ipColIdx >= len(record) {
			continue
		}
		ip := strings.TrimSpace(record[ipColIdx])
		if isValidIP(ip) {
			ips[ip] = true
			count++
			if count >= f.config.MaxIPs {
				break
			}
		}
	}

	return mapKeys(ips), nil
}

func (f *Fetcher) parseText(r io.Reader) ([]string, error) {
	ips := make(map[string]bool)
	cidrs := make(map[string]bool)
	scanner := bufio.NewScanner(r)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		raw := parseLine(line)
		if raw == "" {
			continue
		}

		if isCIDR(raw) {
			// Store CIDR for efficient matching
			if _, _, err := net.ParseCIDR(raw); err == nil {
				cidrs[raw] = true
			}
		} else if isValidIP(raw) {
			ips[raw] = true
			count++
			if count >= f.config.MaxIPs {
				break
			}
		}
	}

	result := mapKeys(ips)
	// Append CIDRs as-is for matching
	for cidr := range cidrs {
		result = append(result, cidr)
	}

	return result, scanner.Err()
}

func (f *Fetcher) parseJSON(r io.Reader, ipField string) ([]string, error) {
	// Simple JSON array or object parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	ips := make(map[string]bool)
	text := string(data)
	// Extract IPs using regex from JSON content
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, `"[],{}`)
		if isValidIP(word) {
			ips[word] = true
		}
	}
	return mapKeys(ips), nil
}

func (f *Fetcher) storeIPs(key string, ips []string) error {
	if f.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// Use pipeline for batch insert
	members := make([]interface{}, len(ips))
	for i, ip := range ips {
		members[i] = ip
	}
	// Delete old set and recreate
	f.redisClient.Del(key)
	for _, ip := range ips {
		f.redisClient.SetAdd(key, ip)
	}
	return nil
}

// IsThreatIP checks if an IP is in the threat intelligence blacklist
func (f *Fetcher) IsThreatIP(ip string) bool {
	if f.redisClient == nil || !f.config.Enabled {
		return false
	}
	isMember, err := f.redisClient.SetContains(f.config.GlobalKey, ip)
	if err != nil {
		return false
	}
	return isMember
}

// GetStats returns statistics for all sources
func (f *Fetcher) GetStats() map[string]*SourceStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]*SourceStats)
	for k, v := range f.stats {
		result[k] = v
	}
	return result
}

func (f *Fetcher) sanitizeKey(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, " ", "_"), ".", "_"))
}

func isValidIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "0.0.0.0" || ip == "255.255.255.255" {
		return false
	}
	if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") || ip == "::1" {
		return false
	}
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func extractIP(line string) string {
	// Take first word
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	ip := strings.TrimSpace(fields[0])

	// Remove port notation (e.g., 1.2.3.4:80 -> 1.2.3.4)
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		before := ip[:idx]
		// Only strip port if it looks like IPv4:port (3 dots before last colon)
		if strings.Count(before, ".") == 3 {
			ip = before
		}
	}

	// Remove subnet notation (/24, /32 etc)
	if idx := strings.Index(ip, "/"); idx > 0 {
		ip = ip[:idx]
	}

	return ip
}

// expandCIDR expands a CIDR notation (e.g., 1.2.3.0/24) into individual IPs
// For large ranges, stores the CIDR notation for efficient matching
func expandCIDR(cidr string) []string {
	cidr = strings.TrimSpace(cidr)
	if !strings.Contains(cidr, "/") {
		return nil
	}

	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}

	var ips []string
	// For /24 and larger, just store the CIDR for efficient matching
	ones, bits := ipnet.Mask.Size()
	if bits == 32 && ones <= 24 {
		// Store CIDR notation for efficient matching
		return []string{cidr}
	}

	// For smaller ranges, expand to individual IPs
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
		if len(ips) >= 65536 { // Safety limit
			break
		}
	}

	// Remove network and broadcast addresses
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// parseLine extracts IP or CIDR from a line
func parseLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(fields[0])
}

// isCIDR checks if a string is CIDR notation
func isCIDR(s string) bool {
	return strings.Contains(s, "/")
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
