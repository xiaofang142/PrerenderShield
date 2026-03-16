package dashboard

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*.html
var templateFS embed.FS

// Dashboard 仪表板处理器
type Dashboard struct {
	config     *Config
	metricsSrc MetricsSource
	templates  *template.Template
	mu         sync.RWMutex
}

// Config 仪表板配置
type Config struct {
	Enabled         bool          // 是否启用仪表板
	ListenAddr      string        // 监听地址
	RefreshInterval time.Duration // 数据刷新间隔
	MaxHistory      int           // 最大历史记录数
}

// MetricsSource 指标数据源接口
type MetricsSource interface {
	GetOverview(ctx context.Context) (*Overview, error)
	GetSecurityStats(ctx context.Context) (*SecurityStats, error)
	GetPerformanceStats(ctx context.Context) (*PerformanceStats, error)
	GetSystemHealth(ctx context.Context) (*SystemHealth, error)
}

// Overview 概览数据
type Overview struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalRequests   int64     `json:"total_requests"`
	ActiveRequests  int64     `json:"active_requests"`
	CacheHitRate    float64   `json:"cache_hit_rate"`
	AvgResponseTime float64   `json:"avg_response_time"`
	ThreatsBlocked  int64     `json:"threats_blocked"`
	SitesCount      int       `json:"sites_count"`
}

// SecurityStats 安全统计
type SecurityStats struct {
	Timestamp     time.Time         `json:"timestamp"`
	ThreatsByType map[string]int64  `json:"threats_by_type"`
	TopBlockedIPs []IPThreat        `json:"top_blocked_ips"`
	AttackTrend   []TimeSeriesPoint `json:"attack_trend"`
}

// IPThreat IP 威胁信息
type IPThreat struct {
	IP         string    `json:"ip"`
	Count      int64     `json:"count"`
	LastSeen   time.Time `json:"last_seen"`
	ThreatType string    `json:"threat_type"`
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Time  time.Time `json:"time"`
	Value int64     `json:"value"`
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	Timestamp      time.Time         `json:"timestamp"`
	RenderDuration []TimeSeriesPoint `json:"render_duration"`
	CacheHitRate   []TimeSeriesPoint `json:"cache_hit_rate"`
	ResponseTime   []TimeSeriesPoint `json:"response_time"`
	Throughput     []TimeSeriesPoint `json:"throughput"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	Timestamp    time.Time       `json:"timestamp"`
	Status       string          `json:"status"`
	Goroutines   int             `json:"goroutines"`
	MemoryUsage  uint64          `json:"memory_usage"`
	MemoryLimit  uint64          `json:"memory_limit"`
	CPUUsage     float64         `json:"cpu_usage"`
	DiskUsage    *DiskUsage      `json:"disk_usage"`
	ModuleStatus map[string]bool `json:"module_status"`
}

// DiskUsage 磁盘使用情况
type DiskUsage struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		ListenAddr:      ":9090",
		RefreshInterval: 5 * time.Second,
		MaxHistory:      1000,
	}
}

// NewDashboard 创建仪表板
func NewDashboard(config *Config, metricsSrc MetricsSource) (*Dashboard, error) {
	if config == nil {
		config = DefaultConfig()
	}

	d := &Dashboard{
		config:     config,
		metricsSrc: metricsSrc,
	}

	// 解析模板
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		// 如果嵌入的模板不存在，使用默认模板
		tmpl = template.Must(template.New("dashboard").Parse(defaultTemplate))
	}
	d.templates = tmpl

	return d, nil
}

// RegisterRoutes 注册路由
func (d *Dashboard) RegisterRoutes(router *gin.Engine) {
	dashboard := router.Group("/dashboard")
	{
		dashboard.GET("/", d.handleIndex)
		dashboard.GET("/api/overview", d.handleOverview)
		dashboard.GET("/api/security", d.handleSecurity)
		dashboard.GET("/api/performance", d.handlePerformance)
		dashboard.GET("/api/health", d.handleHealth)
		dashboard.GET("/api/metrics", d.handleMetrics)
	}
}

// handleIndex 处理首页请求
func (d *Dashboard) handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":            "Prerender Shield Dashboard",
		"refresh_interval": d.config.RefreshInterval.Seconds(),
	})
}

// handleOverview 处理概览 API
func (d *Dashboard) handleOverview(c *gin.Context) {
	ctx := c.Request.Context()
	overview, err := d.metricsSrc.GetOverview(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// handleSecurity 处理安全统计 API
func (d *Dashboard) handleSecurity(c *gin.Context) {
	ctx := c.Request.Context()
	stats, err := d.metricsSrc.GetSecurityStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// handlePerformance 处理性能统计 API
func (d *Dashboard) handlePerformance(c *gin.Context) {
	ctx := c.Request.Context()
	stats, err := d.metricsSrc.GetPerformanceStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// handleHealth 处理健康状态 API
func (d *Dashboard) handleHealth(c *gin.Context) {
	ctx := c.Request.Context()
	health, err := d.metricsSrc.GetSystemHealth(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, health)
}

// handleMetrics Prometheus 格式指标
func (d *Dashboard) handleMetrics(c *gin.Context) {
	c.Header("Content-Type", "text/plain; version=0.0.4")
	// 从 metricsSrc 获取 Prometheus 格式指标
	c.String(http.StatusOK, "# Metrics endpoint")
}

// Start 启动仪表板服务器（独立端口）
func (d *Dashboard) Start() error {
	if !d.config.Enabled {
		return nil
	}

	router := gin.New()
	d.RegisterRoutes(router)

	return http.ListenAndServe(d.config.ListenAddr, router)
}

// defaultTemplate 默认 HTML 模板
const defaultTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1a1a2e; color: #eee; }
        .header { background: #16213e; padding: 20px; border-bottom: 2px solid #0f3460; }
        .header h1 { font-size: 24px; color: #e94560; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin-bottom: 20px; }
        .card { background: #16213e; border-radius: 8px; padding: 20px; border: 1px solid #0f3460; }
        .card h3 { color: #e94560; margin-bottom: 15px; font-size: 16px; }
        .metric { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #0f3460; }
        .metric:last-child { border-bottom: none; }
        .metric-value { color: #4ecca3; font-weight: bold; }
        .status-ok { color: #4ecca3; }
        .status-warning { color: #ffc107; }
        .status-error { color: #e94560; }
        .refresh-indicator { position: fixed; top: 20px; right: 20px; background: #0f3460; padding: 10px 15px; border-radius: 4px; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Prerender Shield Dashboard</h1>
    </div>
    <div class="refresh-indicator">自动刷新：<span id="countdown">{{.refresh_interval}}</span>s</div>
    <div class="container">
        <div class="grid">
            <div class="card">
                <h3>概览</h3>
                <div class="metric"><span>总请求数</span><span class="metric-value" id="total-requests">-</span></div>
                <div class="metric"><span>活跃请求</span><span class="metric-value" id="active-requests">-</span></div>
                <div class="metric"><span>缓存命中率</span><span class="metric-value" id="cache-hit-rate">-</span></div>
                <div class="metric"><span>平均响应时间</span><span class="metric-value" id="avg-response-time">-</span></div>
            </div>
            <div class="card">
                <h3>安全统计</h3>
                <div class="metric"><span>阻止威胁数</span><span class="metric-value" id="threats-blocked">-</span></div>
                <div class="metric"><span>AI 检测威胁</span><span class="metric-value" id="ai-threats">-</span></div>
                <div class="metric"><span>DDoS 攻击</span><span class="metric-value" id="ddos-attacks">-</span></div>
            </div>
            <div class="card">
                <h3>系统健康</h3>
                <div class="metric"><span>状态</span><span class="metric-value status-ok" id="system-status">正常</span></div>
                <div class="metric"><span>Goroutines</span><span class="metric-value" id="goroutines">-</span></div>
                <div class="metric"><span>内存使用</span><span class="metric-value" id="memory-usage">-</span></div>
            </div>
        </div>
    </div>
    <script>
        let countdown = {{.refresh_interval}};
        setInterval(() => {
            countdown--;
            document.getElementById('countdown').textContent = countdown;
            if (countdown <= 0) {
                countdown = {{.refresh_interval}};
                fetch('/dashboard/api/overview').then(r => r.json()).then(d => {
                    document.getElementById('total-requests').textContent = d.total_requests || 0;
                    document.getElementById('active-requests').textContent = d.active_requests || 0;
                    document.getElementById('cache-hit-rate').textContent = (d.cache_hit_rate * 100).toFixed(1) + '%';
                    document.getElementById('avg-response-time').textContent = (d.avg_response_time || 0).toFixed(2) + 'ms';
                });
                fetch('/dashboard/api/health').then(r => r.json()).then(d => {
                    document.getElementById('goroutines').textContent = d.goroutines || 0;
                    document.getElementById('memory-usage').textContent = (d.memory_usage / 1024 / 1024).toFixed(1) + ' MB';
                });
            }
        }, 1000);
    </script>
</body>
</html>`
