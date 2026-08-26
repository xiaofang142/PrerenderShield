package repository

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"prerender-shield/internal/constants"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
)

// AlertRecord 告警记录（ZSet member 的 JSON 结构）
type AlertRecord struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`
	Rule      string    `json:"rule"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Status    string    `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AlertRuleData 前端告警规则数据结构
type AlertRuleData struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Metric      string  `json:"metric"`
	Operator    string  `json:"operator"`
	Threshold   float64 `json:"threshold"`
	Severity    string  `json:"severity"`
	Enabled     bool    `json:"enabled"`
	Cooldown    int     `json:"cooldown"`
	Description string  `json:"description"`
}

const (
	alertHistoryKey  = constants.RedisKeyAlertHistory
	alertRulesKey    = "monitoring:alert-rules"
	alertHistoryTTL  = 30 * 24 * time.Hour
	legacyHistoryPfx = "alert:history:"
)

// AlertRepository 告警历史与告警规则的持久化仓储。
// 历史使用 ZSet（score=Unix 秒），规则使用单个 JSON String 键；
// 此处是唯一读写这些键的位置，monitor 写入与 controller 读取均委托本层。
type AlertRepository struct {
	client *redis.Client
}

// NewAlertRepository 创建告警仓储
func NewAlertRepository(client *redis.Client) *AlertRepository {
	return &AlertRepository{client: client}
}

// AppendAlertHistory 追加一条告警记录，并裁剪超出保留窗口的旧记录
func (r *AlertRepository) AppendAlertHistory(record AlertRecord) {
	if r.client == nil {
		return
	}
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	data, err := json.Marshal(record)
	if err != nil {
		logging.DefaultLogger.Error("marshal alert record: %v", err)
		return
	}
	if err := r.client.ZAdd(alertHistoryKey, float64(record.Timestamp.Unix()), string(data)); err != nil {
		logging.DefaultLogger.Error("save alert history: %v", err)
		return
	}
	if err := r.client.Expire(alertHistoryKey, alertHistoryTTL); err != nil {
		logging.DefaultLogger.Warn("refresh alert history TTL: %v", err)
	}
	cutoff := fmt.Sprintf("%d", time.Now().Add(-alertHistoryTTL).Unix())
	if _, err := r.client.ZRemRangeByScore(alertHistoryKey, "-inf", cutoff); err != nil {
		logging.DefaultLogger.Warn("trim alert history: %v", err)
	}
}

// GetAlertHistory 获取告警历史（最新的在前），ZSet 为空时回退旧版散列键格式
func (r *AlertRepository) GetAlertHistory(limit int) []AlertRecord {
	if r.client == nil {
		return []AlertRecord{}
	}
	members, err := r.client.ZRevRange(alertHistoryKey, 0, int64(limit-1))
	if err != nil {
		return []AlertRecord{}
	}
	records := make([]AlertRecord, 0, len(members))
	for _, m := range members {
		var record AlertRecord
		if err := json.Unmarshal([]byte(m), &record); err == nil {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return r.getLegacyAlertHistory(limit)
	}
	return records
}

// getLegacyAlertHistory 读取旧版 alert:history:<uuid> 散列键（30 天 TTL 内自然过渡）。
// redis.Client.Keys 内部实现为 SCAN 游标分批遍历，非 KEYS 阻塞命令。
func (r *AlertRepository) getLegacyAlertHistory(limit int) []AlertRecord {
	keys, err := r.client.Keys(legacyHistoryPfx + "*")
	if err != nil || len(keys) == 0 {
		return []AlertRecord{}
	}
	all := make([]AlertRecord, 0, len(keys))
	for _, k := range keys {
		val, err := r.client.Get(k)
		if err != nil || val == "" {
			continue
		}
		var record AlertRecord
		if err := json.Unmarshal([]byte(val), &record); err == nil {
			all = append(all, record)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})
	if len(all) > limit {
		all = all[:limit]
	}
	return all
}

// GetAlertRules 获取全部告警规则
func (r *AlertRepository) GetAlertRules() []AlertRuleData {
	rules := []AlertRuleData{}
	if r.client == nil {
		return rules
	}
	val, err := r.client.Get(alertRulesKey)
	if err != nil || val == "" {
		return rules
	}
	if err := json.Unmarshal([]byte(val), &rules); err != nil {
		return []AlertRuleData{}
	}
	return rules
}

// SaveAlertRule 更新或新增单条告警规则（按 ID upsert）
func (r *AlertRepository) SaveAlertRule(rule AlertRuleData) error {
	if r.client == nil {
		return fmt.Errorf("alert repository: redis client not available")
	}
	rules := r.GetAlertRules()
	found := false
	for i, existing := range rules {
		if existing.ID == rule.ID {
			rules[i] = rule
			found = true
			break
		}
	}
	if !found {
		if rule.ID == "" {
			rule.ID = uuid.New().String()
		}
		rules = append(rules, rule)
	}
	return r.saveRules(rules)
}

// DeleteAlertRule 按 ID 删除告警规则；不存在时不视为错误
func (r *AlertRepository) DeleteAlertRule(ruleID string) error {
	if r.client == nil {
		return fmt.Errorf("alert repository: redis client not available")
	}
	rules := r.GetAlertRules()
	out := rules[:0]
	deleted := false
	for _, existing := range rules {
		if existing.ID == ruleID {
			deleted = true
			continue
		}
		out = append(out, existing)
	}
	if !deleted {
		return nil
	}
	return r.saveRules(out)
}

func (r *AlertRepository) saveRules(rules []AlertRuleData) error {
	data, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("serialize alert rules: %w", err)
	}
	return r.client.Set(alertRulesKey, string(data), 0)
}
