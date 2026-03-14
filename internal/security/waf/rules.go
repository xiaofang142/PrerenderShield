package waf

import (
	"encoding/json"
	"os"
	"sync"

	"prerender-shield/internal/security/waf/types"
)

// RuleManager 规则管理器
type RuleManager struct {
	rules      map[string][]types.Rule
	rulesPath  string
	autoUpdate bool
	mu         sync.RWMutex
}

// NewRuleManager 创建规则管理器
func NewRuleManager(rulesPath string) *RuleManager {
	return &RuleManager{
		rules:     make(map[string][]types.Rule),
		rulesPath: rulesPath,
	}
}

// AddRule 添加规则
func (m *RuleManager) AddRule(rule interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r := rule.(type) {
	case types.Rule:
		category := getRuleCategory(r)
		m.rules[category] = append(m.rules[category], r)
	}

	return nil
}

// RemoveRule 删除规则
func (m *RuleManager) RemoveRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for category, rules := range m.rules {
		for i, rule := range rules {
			if rule.ID == id {
				m.rules[category] = append(rules[:i], rules[i+1:]...)
				return nil
			}
		}
	}

	return nil
}

// LoadRules 从文件加载规则
func (m *RuleManager) LoadRules() error {
	if m.rulesPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.rulesPath)
	if err != nil {
		return err
	}

	var rules []types.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rule := range rules {
		category := getRuleCategory(rule)
		m.rules[category] = append(m.rules[category], rule)
	}

	return nil
}

// GetRules 获取所有规则
func (m *RuleManager) GetRules() []types.Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allRules []types.Rule
	for _, rules := range m.rules {
		allRules = append(allRules, rules...)
	}

	return allRules
}

func getRuleCategory(rule types.Rule) string {
	for _, tag := range rule.Tags {
		if tag != "" {
			return tag
		}
	}
	return "default"
}
