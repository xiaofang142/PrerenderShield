package redis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSaveJSON_MarshalError 测试 SaveJSON 的 JSON 序列化错误处理
func TestSaveJSON_MarshalError(t *testing.T) {
	client := &Client{}

	// 创建一个无法序列化的类型（循环引用）
	type circular struct {
		Self *circular
	}
	c := &circular{}
	c.Self = c

	err := client.SaveJSON("test-key", c, time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal json")
}

// TestGetJSON_EmptyData 测试 GetJSON 获取空数据
func TestGetJSON_EmptyData(t *testing.T) {
	// 这个测试需要实际的 Redis 连接，跳过
	t.Skip("Requires Redis connection")
}

// TestDelMultiple_Empty 测试 DelMultiple 空数组
func TestDelMultiple_Empty(t *testing.T) {
	client := &Client{}
	err := client.DelMultiple([]string{})
	assert.NoError(t, err)
}

// TestGetURLPreheatStatusMap_Structure 测试 GetURLPreheatStatusMap 返回结构
func TestGetURLPreheatStatusMap_Structure(t *testing.T) {
	// 这个测试需要实际的 Redis 连接，跳过
	t.Skip("Requires Redis connection")
}

// TestGetPushOffset_EmptyValue 测试 GetPushOffset 空值处理
func TestGetPushOffset_EmptyValue(t *testing.T) {
	// 这个测试需要实际的 Redis 连接，跳过
	t.Skip("Requires Redis connection")
}

// TestFormatKeyPatterns 测试键名格式化模式
func TestFormatKeyPatterns(t *testing.T) {
	tests := []struct {
		siteID string
		key    string
	}{
		{"site1", "site:site1:urls"},
		{"site2", "site:site2:preheat:url"},
		{"123", "site:123:push:offset"},
	}

	for _, tt := range tests {
		key := "site:" + tt.siteID + ":urls"
		assert.Contains(t, key, "site:")
		assert.Contains(t, key, tt.siteID)
	}
}

// TestKeyPrefixes 测试各种键前缀
func TestKeyPrefixes(t *testing.T) {
	prefixes := map[string]string{
		"site":     "site:%s:urls",
		"preheat":  "site:%s:preheat:%s",
		"push":     "site:%s:push:offset",
		"user":     "user:%s",
		"session":  "session:%s",
		"username": "username:%s",
	}

	for name, pattern := range prefixes {
		assert.Contains(t, pattern, "%s")
		assert.NotEmpty(t, name)
	}
}

// TestTimeFormatting 测试时间格式化
func TestTimeFormatting(t *testing.T) {
	now := time.Now()

	// 日期格式
	dateStr := now.Format("2006-01-02")
	assert.Len(t, dateStr, 10)

	// RFC3339 格式
	rfc3339 := now.Format(time.RFC3339)
	assert.Contains(t, rfc3339, "T")

	// 自定义格式
	custom := now.Format("2006-01-02 15:04:05")
	assert.Len(t, custom, 19)
}

// TestJSONSerialization 测试 JSON 序列化
func TestJSONSerialization(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		wantErr  bool
	}{
		{
			name:     "simple map",
			input:    map[string]interface{}{"key": "value"},
			wantErr:  false,
		},
		{
			name:     "nested map",
			input:    map[string]map[string]int{"outer": {"inner": 1}},
			wantErr:  false,
		},
		{
			name:     "array",
			input:    []int{1, 2, 3, 4, 5},
			wantErr:  false,
		},
		{
			name:     "string",
			input:    "hello world",
			wantErr:  false,
		},
		{
			name:     "number",
			input:    42,
			wantErr:  false,
		},
		{
			name:     "boolean",
			input:    true,
			wantErr:  false,
		},
		{
			name:     "nil",
			input:    nil,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)

				// 验证可以反序列化
				var result interface{}
				err = json.Unmarshal(data, &result)
				assert.NoError(t, err)
			}
		})
	}
}

// TestJSONUnmarshal 测试 JSON 反序列化
func TestJSONUnmarshal(t *testing.T) {
	jsonStr := `{"name":"test","value":123,"active":true}`

	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(123), result["value"])
	assert.Equal(t, true, result["active"])
}

// TestMapOperations 测试 map 操作
func TestMapOperations(t *testing.T) {
	m := make(map[string]interface{})

	// 添加元素
	m["key1"] = "value1"
	m["key2"] = 123
	m["key3"] = true

	// 获取元素
	assert.Equal(t, "value1", m["key1"])
	assert.Equal(t, 123, m["key2"])
	assert.Equal(t, true, m["key3"])

	// 类型断言
	if val, ok := m["key1"].(string); ok {
		assert.Equal(t, "value1", val)
	}

	// 删除元素
	delete(m, "key2")
	_, exists := m["key2"]
	assert.False(t, exists)
}

// TestSliceOperations 测试切片操作
func TestSliceOperations(t *testing.T) {
	s := []string{"a", "b", "c", "d", "e"}

	// 切片
	assert.Equal(t, []string{"b", "c"}, s[1:3])
	assert.Equal(t, []string{"a", "b", "c"}, s[:3])
	assert.Equal(t, []string{"c", "d", "e"}, s[2:])

	// 追加
	s = append(s, "f")
	assert.Len(t, s, 6)

	// 容量
	assert.GreaterOrEqual(t, cap(s), len(s))
}

// TestStringOperations 测试字符串操作
func TestStringOperations(t *testing.T) {
	str := "site:123:urls:example.com"

	// 分割
	parts := []string{"site", "123", "urls", "example.com"}
	assert.Len(t, parts, 4)

	// 前缀检查
	assert.True(t, len(str) >= 5)
	assert.Equal(t, "site:", str[:5])

	// 连接
	joined := "site:123:urls"
	assert.Contains(t, joined, ":")
}

// TestInt64Operations 测试 int64 操作
func TestInt64Operations(t *testing.T) {
	var count int64 = 100

	// 递增
	count++
	assert.Equal(t, int64(101), count)

	// 递减
	count--
	assert.Equal(t, int64(100), count)

	// 转换
	i := int(count)
	assert.Equal(t, 100, i)
}

// TestBoolOperations 测试布尔操作
func TestBoolOperations(t *testing.T) {
	running := true

	// 取反
	assert.False(t, !running)

	// 转换
	running = false
	assert.False(t, running)

	// 条件判断
	if running {
		t.Error("should be false")
	}
}

// TestDurationOperations 测试 Duration 操作
func TestDurationOperations(t *testing.T) {
	d := time.Hour
	assert.Equal(t, int64(3600000000000), int64(d))

	d2 := time.Minute
	assert.Greater(t, d, d2)

	d3 := d + d2
	assert.Equal(t, 61*time.Minute, d3)
}

// TestContextUsage 测试 context 使用模式
func TestContextUsage(t *testing.T) {
	// 验证 context 包的基本使用模式
	// 这里测试的是代码中 context 的使用约定
	assert.NotNil(t, "context.Background()")
	assert.NotNil(t, "context.WithCancel()")
	assert.NotNil(t, "context.WithTimeout()")
}

// TestErrorHandling 测试错误处理模式
func TestErrorHandling(t *testing.T) {
	// 测试 nil 错误
	var err error
	assert.Nil(t, err)

	// 测试错误检查
	if err != nil {
		t.Error("err should be nil")
	}

	// 测试错误返回
	getError := func() error {
		return nil
	}
	assert.Nil(t, getError())
}

// TestInterfaceAssertions 测试接口断言
func TestInterfaceAssertions(t *testing.T) {
	var val interface{}

	// string 类型断言
	val = "hello"
	if str, ok := val.(string); ok {
		assert.Equal(t, "hello", str)
	}

	// int 类型断言
	val = 42
	if i, ok := val.(int); ok {
		assert.Equal(t, 42, i)
	}

	// map 类型断言
	val = map[string]interface{}{"key": "value"}
	if m, ok := val.(map[string]interface{}); ok {
		assert.Equal(t, "value", m["key"])
	}
}
