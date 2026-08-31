package detectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
)

// CustomRulesRedisClient 自定义规则检测器所需的 Redis 最小接口
type CustomRulesRedisClient interface {
	Get(key string) (string, error)
}

// UICustomRule 控制台「WAF Rules」页保存的规则形态（firewall:rules:<siteID>）
type UICustomRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Field    string `json:"field"`    // query/path/header/body/ip/user_agent/country
	Operator string `json:"operator"` // contains/equals/matches/gt/lt/in
	Value    string `json:"value"`
	Action   string `json:"action"` // block/allow/log
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

type customRulePayload struct {
	Rules []UICustomRule `json:"rules"`
}

// CustomRuleDetector 执行控制台可视化编辑的自定义规则。
// 修复（R12-BUG-1）：此前 UI 规则仅持久化到 firewall:rules:<siteID> 并被 API 回显，
// 无任何引擎组件消费——规则对实际流量零效力（假性完成）。
// 本检测器挂入引擎检测器链，按 5s 缓存热读规则，实现文档宣称的"规则热加载"。
type CustomRuleDetector struct {
	siteID   string
	redis    CustomRulesRedisClient
	mu       sync.RWMutex
	rules    []UICustomRule
	lastLoad time.Time
	compiled map[string]*regexp.Regexp // 规则ID -> 编译后的 matches 正则
}

// NewCustomRuleDetector 创建站点自定义规则检测器
func NewCustomRuleDetector(siteID string, redis CustomRulesRedisClient) *CustomRuleDetector {
	return &CustomRuleDetector{
		siteID:   siteID,
		redis:    redis,
		compiled: make(map[string]*regexp.Regexp),
	}
}

func (d *CustomRuleDetector) Name() string { return "custom_rule" }

const customRulesCacheTTL = 5 * time.Second

// loadRules 热读站点规则（5s 缓存；读失败沿用旧规则，保证 Redis 抖动不致放行态漂移）
func (d *CustomRuleDetector) loadRules() {
	d.mu.RLock()
	fresh := time.Since(d.lastLoad) < customRulesCacheTTL && d.rules != nil
	d.mu.RUnlock()
	if fresh {
		return
	}

	var payload customRulePayload
	if d.redis != nil {
		if raw, err := d.redis.Get("firewall:rules:" + d.siteID); err == nil && raw != "" {
			if json.Unmarshal([]byte(raw), &payload) == nil {
				compiled := make(map[string]*regexp.Regexp, len(payload.Rules))
				for _, r := range payload.Rules {
					if r.Operator == "matches" && r.Value != "" {
						if re, err := regexp.Compile(r.Value); err == nil {
							compiled[r.ID] = re
						}
					}
				}
				d.mu.Lock()
				d.rules = payload.Rules
				d.compiled = compiled
				d.lastLoad = time.Now()
				d.mu.Unlock()
				return
			}
		}
	}
	// 读取失败/为空：置空但刷新时间，避免每请求重复打 Redis
	d.mu.Lock()
	d.rules = []UICustomRule{}
	d.lastLoad = time.Now()
	d.mu.Unlock()
}

// Detect 对请求执行站点自定义规则
func (d *CustomRuleDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.loadRules()

	d.mu.RLock()
	rules := make([]UICustomRule, len(d.rules))
	copy(rules, d.rules)
	compiled := d.compiled
	d.mu.RUnlock()

	var threats []types.Threat
	for _, r := range rules {
		if !r.Enabled || r.Action == "allow" {
			continue // allow 在当前引擎语义下为 no-op（无高危威胁即放行）
		}
		if !d.matchRule(r, req, compiled) {
			continue
		}
		severity := "low" // log：记录不拦截
		if r.Action == "block" {
			severity = "high" // 引擎对 high/critical 阻断
		}
		threats = append(threats, types.Threat{
			Type:     "custom_rule",
			SubType:  r.Action,
			Severity: severity,
			Message:  fmt.Sprintf("custom rule '%s' matched (%s %s %s)", r.Name, r.Field, r.Operator, r.Value),
			RuleID:   r.ID,
			RuleName: r.Name,
			SourceIP: getClientIP(req),
			Details:  map[string]interface{}{"field": r.Field, "operator": r.Operator, "value": r.Value, "action": r.Action},
		})
	}
	return threats, nil
}

// matchRule 提取字段值并应用算子。
// joined: 字段整体串（contains/equals/matches 用）；values: 结构化候选值（in/gt/lt 用，如 query 各参数解码值）
func (d *CustomRuleDetector) matchRule(r UICustomRule, req *http.Request, compiled map[string]*regexp.Regexp) bool {
	joined, values := d.extractField(r.Field, req)
	if r.Field == "country" {
		// GeoIP 归属判定由引擎 GeoIP 检测器负责；自定义规则 v1 不重复实现
		return false
	}

	switch r.Operator {
	case "contains":
		return joined != "" && strings.Contains(joined, r.Value)
	case "equals":
		return joined == r.Value
	case "matches", "regex":
		if re := compiled[r.ID]; re != nil {
			return re.MatchString(joined)
		}
		return false
	case "gt", "lt":
		t, err2 := strconv.ParseFloat(r.Value, 64)
		if err2 != nil {
			return false
		}
		for _, v := range values {
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				continue
			}
			if (r.Operator == "gt" && n > t) || (r.Operator == "lt" && n < t) {
				return true
			}
		}
		return false
	case "in":
		if joined == "" {
			return false
		}
		candidates := values
		if len(candidates) == 0 {
			candidates = []string{joined}
		}
		for _, item := range strings.Split(r.Value, ",") {
			item = strings.TrimSpace(item)
			for _, c := range candidates {
				if item == strings.TrimSpace(c) && item != "" {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// extractField 从请求中提取规则字段的匹配对象。
// 返回：joined 整体串；values 结构化候选值（可为空）。
func (d *CustomRuleDetector) extractField(field string, req *http.Request) (string, []string) {
	switch field {
	case "user_agent":
		return req.UserAgent(), nil
	case "query":
		// 解码后的查询串 + 各参数值（模板正则如 "UNION SELECT" 依赖解码语义）
		q := req.URL.Query()
		var pairs []string
		var vals []string
		for k, vs := range q {
			for _, v := range vs {
				pairs = append(pairs, k+"="+v)
				vals = append(vals, v)
			}
		}
		return strings.Join(pairs, "&"), vals
	case "path":
		return req.URL.Path, nil
	case "ip":
		return getClientIP(req), nil
	case "header":
		// 语义：任意请求头（含名称）整体子串匹配，如模板中的 "onerror="
		var sb strings.Builder
		for name, vals := range req.Header {
			sb.WriteString(name)
			sb.WriteString(":")
			sb.WriteString(strings.Join(vals, ","))
			sb.WriteString("\n")
		}
		return sb.String(), nil
	case "body":
		if req.Body == nil {
			return "", nil
		}
		// 引擎对 POST body 有大小上限保护（与其它 detector 的读取约定一致）
		buf := make([]byte, 8192)
		n, _ := req.Body.Read(buf)
		return string(buf[:n]), nil
	case "country":
		return "", nil
	default:
		return "", nil
	}
}
