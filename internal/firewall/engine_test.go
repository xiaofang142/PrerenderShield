package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEngine_NewEngine(t *testing.T) {
	// 测试创建新引擎
	config := Config{
		RulesPath: "/tmp/rules",
		ActionConfig: ActionConfig{
			DefaultAction: "block",
			BlockMessage:  "Request blocked",
		},
	}

	engine, err := NewEngine(config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestEngine_UpdateRules(t *testing.T) {
	// 测试更新规则
	config := Config{
		RulesPath: "/tmp/rules",
	}

	engine, err := NewEngine(config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	// 更新规则，应该不返回错误
	err = engine.UpdateRules()
	assert.NoError(t, err)
}

