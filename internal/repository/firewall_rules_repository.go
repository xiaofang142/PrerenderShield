package repository

import (
	"encoding/json"
	"fmt"

	"prerender-shield/internal/redis"
)

// FirewallRulesRepository 站点防火墙规则（firewall:rules:<siteID>）的持久化仓储
type FirewallRulesRepository struct {
	client *redis.Client
}

// NewFirewallRulesRepository 创建防火墙规则仓储
func NewFirewallRulesRepository(client *redis.Client) *FirewallRulesRepository {
	return &FirewallRulesRepository{client: client}
}

func firewallRulesKey(siteID string) string {
	return fmt.Sprintf("firewall:rules:%s", siteID)
}

// Get 获取站点防火墙规则原始 JSON；无数据时返回 nil
func (r *FirewallRulesRepository) Get(siteID string) (map[string]interface{}, error) {
	if r.client == nil {
		return nil, nil
	}
	val, err := r.client.Get(firewallRulesKey(siteID))
	if err != nil || val == "" {
		return nil, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, nil
	}
	return data, nil
}

// Save 保存站点防火墙规则（整体覆盖）
func (r *FirewallRulesRepository) Save(siteID string, rulesJSON []byte) error {
	if r.client == nil {
		return fmt.Errorf("firewall rules repository: redis client not available")
	}
	return r.client.Set(firewallRulesKey(siteID), string(rulesJSON), 0)
}

// DeleteRule 按 ID 删除单条规则；规则不存在时不视为错误
func (r *FirewallRulesRepository) DeleteRule(siteID, ruleID string) error {
	data, err := r.Get(siteID)
	if err != nil || data == nil {
		return err
	}
	rules, ok := data["rules"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]interface{}, 0, len(rules))
	for _, rule := range rules {
		m, ok := rule.(map[string]interface{})
		if ok && m["id"] == ruleID {
			continue
		}
		out = append(out, rule)
	}
	data["rules"] = out
	updated, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("serialize firewall rules: %w", err)
	}
	return r.Save(siteID, updated)
}
