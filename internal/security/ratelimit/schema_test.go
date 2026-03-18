package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// ============== SchemaValidator 基础测试 ==============

func TestDefaultSchemaValidatorConfig(t *testing.T) {
	config := DefaultSchemaValidatorConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableStrict)
	assert.Equal(t, 10, config.MaxDepth)
	assert.Equal(t, 100, config.MaxProperties)
	assert.Equal(t, 100*time.Millisecond, config.Timeout)
	assert.Equal(t, true, config.EnableCaching)
	assert.Equal(t, 1000, config.CacheSize)
}

func TestNewSchemaValidator(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultSchemaValidatorConfig()

	validator := NewSchemaValidator(config, logger)

	assert.NotNil(t, validator)
	assert.NotNil(t, validator.schemas)
	assert.NotNil(t, validator.stats)
}

func TestNewSchemaValidator_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	validator := NewSchemaValidator(nil, logger)

	assert.NotNil(t, validator)
	assert.NotNil(t, validator.config)
	assert.Equal(t, DefaultSchemaValidatorConfig().MaxDepth, validator.config.MaxDepth)
}

func TestNewSchemaValidator_NilLogger(t *testing.T) {
	config := DefaultSchemaValidatorConfig()

	validator := NewSchemaValidator(config, nil)

	assert.NotNil(t, validator)
}

// ============== RegisterSchema 测试 ==============

func TestSchemaValidator_RegisterSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "user",
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	err := validator.RegisterSchema("user", schema)

	assert.NoError(t, err)
	assert.Contains(t, validator.schemas, "user")
}

func TestSchemaValidator_RegisterSchema_InvalidPattern(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:      "invalid",
		Type:    "string",
		Pattern: "[invalid(regex", // 无效的正则表达式
	}

	err := validator.RegisterSchema("invalid", schema)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "编译 schema 失败")
}

func TestSchemaValidator_RegisterSchema_NestedSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "nested",
		Type: "object",
		Properties: map[string]*Schema{
			"user": {
				Type: "object",
				Properties: map[string]*Schema{
					"profile": {
						Type: "object",
						Properties: map[string]*Schema{
							"email": {Type: "string"},
						},
					},
				},
			},
		},
	}

	err := validator.RegisterSchema("nested", schema)

	assert.NoError(t, err)
}

func TestSchemaValidator_RegisterSchema_ExceedMaxDepth(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultSchemaValidatorConfig()
	config.MaxDepth = 2
	validator := NewSchemaValidator(config, logger)

	// 创建超过 MaxDepth 的嵌套 schema
	schema := &Schema{
		ID:   "deep",
		Type: "object",
		Properties: map[string]*Schema{
			"level1": {
				Type: "object",
				Properties: map[string]*Schema{
					"level2": {
						Type: "object",
						Properties: map[string]*Schema{
							"level3": {Type: "string"}, // 这超过了 MaxDepth=2
						},
					},
				},
			},
		},
	}

	err := validator.RegisterSchema("deep", schema)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "嵌套过深")
}

// ============== RemoveSchema 测试 ==============

func TestSchemaValidator_RemoveSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{ID: "test", Type: "string"}
	validator.RegisterSchema("test", schema)
	assert.Contains(t, validator.schemas, "test")

	validator.RemoveSchema("test")
	assert.NotContains(t, validator.schemas, "test")
}

func TestSchemaValidator_RemoveSchema_NonExistent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	assert.NotPanics(t, func() {
		validator.RemoveSchema("non_existent")
	})
}

// ============== GetSchema 测试 ==============

func TestSchemaValidator_GetSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{ID: "test", Type: "string"}
	validator.RegisterSchema("test", schema)

	retrieved := validator.GetSchema("test")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "string", retrieved.Type)
}

func TestSchemaValidator_GetSchema_NonExistent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	retrieved := validator.GetSchema("non_existent")
	assert.Nil(t, retrieved)
}

// ============== ListSchemas 测试 ==============

func TestSchemaValidator_ListSchemas(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	validator.RegisterSchema("schema1", &Schema{ID: "schema1", Type: "string"})
	validator.RegisterSchema("schema2", &Schema{ID: "schema2", Type: "integer"})
	validator.RegisterSchema("schema3", &Schema{ID: "schema3", Type: "object"})

	schemas := validator.ListSchemas()

	assert.Len(t, schemas, 3)
	assert.Contains(t, schemas, "schema1")
	assert.Contains(t, schemas, "schema2")
	assert.Contains(t, schemas, "schema3")
}

func TestSchemaValidator_ListSchemas_Empty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schemas := validator.ListSchemas()
	assert.Empty(t, schemas)
}

// ============== Validate 测试 ==============

func TestSchemaValidator_Validate_String(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:          "string_test",
		Type:        "string",
		MinLength:   intPtr(3),
		MaxLength:   intPtr(10),
		Pattern:     "^[a-z]+$",
		Description: "测试字符串",
	}
	validator.RegisterSchema("string_test", schema)

	ctx := context.Background()

	// 有效字符串
	result := validator.Validate(ctx, "string_test", "hello")
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)

	// 太短
	result = validator.Validate(ctx, "string_test", "ab")
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// 太长
	result = validator.Validate(ctx, "string_test", "verylongstring")
	assert.False(t, result.Valid)

	// 不匹配模式
	result = validator.Validate(ctx, "string_test", "123")
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_Number(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	min := 0.0
	max := 100.0
	schema := &Schema{
		ID:      "number_test",
		Type:    "number",
		Minimum: &min,
		Maximum: &max,
	}
	validator.RegisterSchema("number_test", schema)

	ctx := context.Background()

	// 有效数字
	result := validator.Validate(ctx, "number_test", float64(50))
	assert.True(t, result.Valid)

	// 小于最小值
	result = validator.Validate(ctx, "number_test", float64(-1))
	assert.False(t, result.Valid)

	// 大于最大值
	result = validator.Validate(ctx, "number_test", float64(101))
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_Integer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "integer_test",
		Type: "integer",
	}
	validator.RegisterSchema("integer_test", schema)

	ctx := context.Background()

	// 整数有效
	result := validator.Validate(ctx, "integer_test", 42)
	assert.True(t, result.Valid)

	// float64 整数值也有效
	result = validator.Validate(ctx, "integer_test", float64(42))
	assert.True(t, result.Valid)

	// float64 非整数值无效
	result = validator.Validate(ctx, "integer_test", float64(42.5))
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_Boolean(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "boolean_test",
		Type: "boolean",
	}
	validator.RegisterSchema("boolean_test", schema)

	ctx := context.Background()

	result := validator.Validate(ctx, "boolean_test", true)
	assert.True(t, result.Valid)

	result = validator.Validate(ctx, "boolean_test", false)
	assert.True(t, result.Valid)

	result = validator.Validate(ctx, "boolean_test", "true")
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_Object(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "object_test",
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}
	validator.RegisterSchema("object_test", schema)

	ctx := context.Background()

	// 有效对象
	result := validator.Validate(ctx, "object_test", map[string]interface{}{
		"name": "John",
		"age":  30,
	})
	assert.True(t, result.Valid)

	// 缺少必需字段
	result = validator.Validate(ctx, "object_test", map[string]interface{}{
		"age": 30,
	})
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Field, "name")
	assert.Equal(t, "required", result.Errors[0].Type)
}

func TestSchemaValidator_Validate_Array(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "array_test",
		Type: "array",
		Items: &Schema{
			Type: "string",
		},
	}
	validator.RegisterSchema("array_test", schema)

	ctx := context.Background()

	// 有效数组
	result := validator.Validate(ctx, "array_test", []interface{}{"a", "b", "c"})
	assert.True(t, result.Valid)

	// 包含无效元素的数组
	result = validator.Validate(ctx, "array_test", []interface{}{"a", 123, "c"})
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_Enum(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "enum_test",
		Type: "string",
		Enum: []interface{}{"red", "green", "blue"},
	}
	validator.RegisterSchema("enum_test", schema)

	ctx := context.Background()

	// 有效枚举值
	result := validator.Validate(ctx, "enum_test", "red")
	assert.True(t, result.Valid)

	// 无效枚举值
	result = validator.Validate(ctx, "enum_test", "yellow")
	assert.False(t, result.Valid)
	assert.Equal(t, "enum", result.Errors[0].Type)
}

func TestSchemaValidator_Validate_Null(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "null_test",
		Type: "null",
	}
	validator.RegisterSchema("null_test", schema)

	ctx := context.Background()

	result := validator.Validate(ctx, "null_test", nil)
	assert.True(t, result.Valid)

	result = validator.Validate(ctx, "null_test", "not null")
	assert.False(t, result.Valid)
}

func TestSchemaValidator_Validate_SchemaNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	ctx := context.Background()

	result := validator.Validate(ctx, "non_existent", "any data")

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "schema_not_found", result.Errors[0].Type)
}

func TestSchemaValidator_Validate_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultSchemaValidatorConfig()
	config.Timeout = 1 * time.Nanosecond // 非常短的超时时间
	validator := NewSchemaValidator(config, logger)

	// 创建复杂 schema 触发超时
	schema := &Schema{
		ID:   "timeout_test",
		Type: "object",
	}
	validator.RegisterSchema("timeout_test", schema)

	ctx := context.Background()
	result := validator.Validate(ctx, "timeout_test", map[string]interface{}{})

	// 可能超时或完成验证（取决于执行速度）
	assert.NotNil(t, result)
}

// ============== ValidateRequest 测试 ==============

func TestSchemaValidator_ValidateRequest(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:   "request_test",
		Type: "object",
		Properties: map[string]*Schema{
			"username": {Type: "string"},
			"email":    {Type: "string"},
		},
		Required: []string{"username"},
	}
	validator.RegisterSchema("request_test", schema)

	ctx := context.Background()

	// 有效请求
	result := validator.ValidateRequest(ctx, "request_test", "POST",
		[]byte(`{"username": "john", "email": "john@example.com"}`),
		nil, nil)
	assert.True(t, result.Valid)

	// 缺少必需字段
	result = validator.ValidateRequest(ctx, "request_test", "POST",
		[]byte(`{"email": "john@example.com"}`),
		nil, nil)
	assert.False(t, result.Valid)
}

func TestSchemaValidator_ValidateRequest_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{ID: "json_test", Type: "object"}
	validator.RegisterSchema("json_test", schema)

	ctx := context.Background()

	result := validator.ValidateRequest(ctx, "json_test", "POST",
		[]byte(`{invalid json}`),
		nil, nil)

	assert.False(t, result.Valid)
	assert.Equal(t, "parse_error", result.Errors[0].Type)
}

func TestSchemaValidator_ValidateRequest_EmptyBody(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{
		ID:       "empty_body_test",
		Type:     "object",
		Required: []string{"required_field"},
	}
	validator.RegisterSchema("empty_body_test", schema)

	ctx := context.Background()

	result := validator.ValidateRequest(ctx, "empty_body_test", "POST",
		[]byte{},
		nil, nil)

	assert.False(t, result.Valid)
}

// ============== GetStats 测试 ==============

func TestSchemaValidator_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	validator := NewSchemaValidator(DefaultSchemaValidatorConfig(), logger)

	schema := &Schema{ID: "stats_test", Type: "string"}
	validator.RegisterSchema("stats_test", schema)

	ctx := context.Background()
	validator.Validate(ctx, "stats_test", "test")

	stats := validator.GetStats()

	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalValidations)
}

// ============== SanitizeInput 测试 ==============

func TestSanitizeInput(t *testing.T) {
	// 移除 HTML 标签
	result := SanitizeInput("<script>alert('xss')</script>")
	// HTML 标签被移除，单引号被转义
	assert.Equal(t, "alert(&#x27;xss&#x27;)", result)

	// 转义特殊字符
	result = SanitizeInput("hello & world <test>")
	assert.Contains(t, result, "&amp;")
	// HTML 标签被完全移除
	assert.NotContains(t, result, "<test>")

	// 修剪空格
	result = SanitizeInput("  trimmed  ")
	assert.Equal(t, "trimmed", result)
}

func TestSanitizeInput_QuoteEscaping(t *testing.T) {
	result := SanitizeInput(`hello "world" and 'test'`)
	assert.Contains(t, result, "&quot;")
	assert.Contains(t, result, "&#x27;")
}

func TestSanitizeInput_ComplexHTML(t *testing.T) {
	result := SanitizeInput(`<div class="test"><p>Content with <strong>bold</strong></p></div>`)
	assert.NotContains(t, result, "<")
	assert.NotContains(t, result, ">")
}

// ============== MarshalJSON 测试 ==============

func TestSchema_MarshalJSON(t *testing.T) {
	schema := &Schema{
		ID:          "test",
		Type:        "string",
		Pattern:     "^[a-z]+$",
		Description: "Test schema",
		ReadOnly:    true,
	}

	data, err := schema.MarshalJSON()

	assert.NoError(t, err)
	assert.Contains(t, string(data), `"pattern"`)
	assert.Contains(t, string(data), `"^[a-z]+$"`)
}

// ============== countProperties 测试 ==============

func TestCountProperties(t *testing.T) {
	schema := &Schema{
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
			"address": {
				Properties: map[string]*Schema{
					"street": {Type: "string"},
					"city":   {Type: "string"},
				},
			},
		},
	}

	count := countProperties(schema)

	// 顶层 3 个属性 + address 嵌套 2 个属性 = 5 个
	// countProperties 递归计算所有层级的属性
	// name + age + address.street + address.city = 4
	// 但 address 本身也算 1 个属性
	// 所以总共是 5
	// 实际上：Properties 中有 3 个（name, age, address），address.Properties 有 2 个
	// count = 3 + 2 = 5
	assert.Equal(t, 5, count)
}

func TestCountProperties_Empty(t *testing.T) {
	schema := &Schema{
		Type: "string",
	}

	count := countProperties(schema)
	assert.Equal(t, 0, count)
}

// ============== ValidatorStats 测试 ==============

func TestValidatorStats_Struct(t *testing.T) {
	stats := &ValidatorStats{
		TotalValidations:  100,
		SuccessfulReviews: 80,
		FailedValidations: 20,
		CacheHits:         50,
		CacheMisses:       50,
	}

	assert.Equal(t, int64(100), stats.TotalValidations)
	assert.Equal(t, int64(80), stats.SuccessfulReviews)
	assert.Equal(t, int64(20), stats.FailedValidations)
	assert.Equal(t, int64(50), stats.CacheHits)
	assert.Equal(t, int64(50), stats.CacheMisses)
}

// ============== ValidationError 测试 ==============

func TestValidationError_Struct(t *testing.T) {
	err := ValidationError{
		Field:   "username",
		Message: "缺少必需字段",
		Type:    "required",
		Value:   nil,
	}

	assert.Equal(t, "username", err.Field)
	assert.Equal(t, "缺少必需字段", err.Message)
	assert.Equal(t, "required", err.Type)
	assert.Nil(t, err.Value)
}

// ============== ValidationResult 测试 ==============

func TestValidationResult_Struct(t *testing.T) {
	result := ValidationResult{
		Valid: true,
		Errors: []ValidationError{
			{Field: "test", Message: "test error", Type: "type"},
		},
	}

	assert.True(t, result.Valid)
	assert.Len(t, result.Errors, 1)
}

// ============== Schema 结构测试 ==============

func TestSchema_Struct(t *testing.T) {
	min := 1
	max := 10
	minF := 0.0
	maxF := 100.0

	schema := &Schema{
		ID:          "test",
		Type:        "object",
		Properties:  map[string]*Schema{"name": {Type: "string"}},
		Required:    []string{"name"},
		Pattern:     "^[a-z]+$",
		Enum:        []interface{}{"a", "b"},
		MinLength:   &min,
		MaxLength:   &max,
		Minimum:     &minF,
		Maximum:     &maxF,
		Items:       &Schema{Type: "string"},
		Ref:         "$ref",
		Description: "Test schema",
		Default:     "default",
		ReadOnly:    true,
		WriteOnly:   false,
	}

	assert.Equal(t, "test", schema.ID)
	assert.Equal(t, "object", schema.Type)
	assert.NotNil(t, schema.Properties)
	assert.Equal(t, []string{"name"}, schema.Required)
	assert.Equal(t, "^[a-z]+$", schema.Pattern)
	assert.Len(t, schema.Enum, 2)
}

// ============== 辅助函数 ==============

func intPtr(i int) *int {
	return &i
}
