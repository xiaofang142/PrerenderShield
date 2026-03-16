package ratelimit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SchemaValidator Schema 验证器
type SchemaValidator struct {
	config  *SchemaValidatorConfig
	schemas map[string]*Schema
	mu      sync.RWMutex
	logger  *zap.Logger
	stats   *ValidatorStats
}

// SchemaValidatorConfig 验证器配置
type SchemaValidatorConfig struct {
	EnableStrict  bool          // 严格模式
	MaxDepth      int           // 最大嵌套深度
	MaxProperties int           // 最大属性数
	Timeout       time.Duration // 验证超时
	EnableCaching bool          // 启用缓存
	CacheSize     int           // 缓存大小
}

// Schema JSON Schema 定义
type Schema struct {
	ID          string                 `json:"$id,omitempty"`
	Type        string                 `json:"type"`
	Properties  map[string]*Schema     `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	Items       *Schema                `json:"items,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Description string                 `json:"description,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	ReadOnly    bool                   `json:"readOnly,omitempty"`
	WriteOnly   bool                   `json:"writeOnly,omitempty"`
	Extra       map[string]interface{} `json:"-"`

	compiledPattern *regexp.Regexp
}

// ValidatorStats 验证统计
type ValidatorStats struct {
	TotalValidations  int64 `json:"total_validations"`
	SuccessfulReviews int64 `json:"successful_validations"`
	FailedValidations int64 `json:"failed_validations"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Type    string      `json:"type"` // required, type, pattern, enum, etc.
	Value   interface{} `json:"value,omitempty"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// DefaultSchemaValidatorConfig 返回默认配置
func DefaultSchemaValidatorConfig() *SchemaValidatorConfig {
	return &SchemaValidatorConfig{
		EnableStrict:  true,
		MaxDepth:      10,
		MaxProperties: 100,
		Timeout:       100 * time.Millisecond,
		EnableCaching: true,
		CacheSize:     1000,
	}
}

// NewSchemaValidator 创建 Schema 验证器
func NewSchemaValidator(config *SchemaValidatorConfig, logger *zap.Logger) *SchemaValidator {
	if config == nil {
		config = DefaultSchemaValidatorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SchemaValidator{
		config:  config,
		schemas: make(map[string]*Schema),
		logger:  logger,
		stats:   &ValidatorStats{},
	}
}

// RegisterSchema 注册 Schema
func (v *SchemaValidator) RegisterSchema(id string, schema *Schema) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.compileSchema(schema, 0); err != nil {
		return fmt.Errorf("编译 schema 失败：%w", err)
	}

	v.schemas[id] = schema
	v.logger.Info("注册 Schema", zap.String("id", id))
	return nil
}

// compileSchema 编译 Schema（预编译正则等）
func (v *SchemaValidator) compileSchema(schema *Schema, depth int) error {
	if depth > v.config.MaxDepth {
		return errors.New("schema 嵌套过深")
	}

	countProperties(schema)

	if schema.Pattern != "" {
		compiled, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return fmt.Errorf("无效的正则表达式：%w", err)
		}
		schema.compiledPattern = compiled
	}

	for _, prop := range schema.Properties {
		if err := v.compileSchema(prop, depth+1); err != nil {
			return err
		}
	}

	if schema.Items != nil {
		if err := v.compileSchema(schema.Items, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// Validate 验证数据
func (v *SchemaValidator) Validate(ctx context.Context, schemaID string, data interface{}) *ValidationResult {
	v.stats.TotalValidations++

	v.mu.RLock()
	schema, exists := v.schemas[schemaID]
	v.mu.RUnlock()

	if !exists {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "$",
				Message: fmt.Sprintf("Schema %s 不存在", schemaID),
				Type:    "schema_not_found",
			}},
		}
	}

	resultCtx, cancel := context.WithTimeout(ctx, v.config.Timeout)
	defer cancel()

	done := make(chan *ValidationResult, 1)
	go func() {
		errors := v.validateSchema(schema, data, "$")
		done <- &ValidationResult{
			Valid:  len(errors) == 0,
			Errors: errors,
		}
	}()

	select {
	case result := <-done:
		if result.Valid {
			v.stats.SuccessfulReviews++
		} else {
			v.stats.FailedValidations++
		}
		return result
	case <-resultCtx.Done():
		v.stats.FailedValidations++
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "$",
				Message: "验证超时",
				Type:    "timeout",
			}},
		}
	}
}

// validateSchema 验证数据
func (v *SchemaValidator) validateSchema(schema *Schema, data interface{}, path string) []ValidationError {
	var errors []ValidationError

	// 类型检查
	if schema.Type != "" {
		if !v.checkType(schema.Type, data) {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("期望类型 %s，实际 %T", schema.Type, data),
				Type:    "type",
				Value:   data,
			})
			return errors
		}
	}

	// Enum 检查
	if len(schema.Enum) > 0 {
		found := false
		for _, e := range schema.Enum {
			if reflect.DeepEqual(e, data) {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("值必须在 %v 中", schema.Enum),
				Type:    "enum",
				Value:   data,
			})
		}
	}

	// 字符串检查
	if str, ok := data.(string); ok {
		if schema.MinLength != nil && len(str) < *schema.MinLength {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("字符串长度不能小于 %d", *schema.MinLength),
				Type:    "minLength",
				Value:   data,
			})
		}
		if schema.MaxLength != nil && len(str) > *schema.MaxLength {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("字符串长度不能大于 %d", *schema.MaxLength),
				Type:    "maxLength",
				Value:   data,
			})
		}
		if schema.compiledPattern != nil && !schema.compiledPattern.MatchString(str) {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("值不匹配模式 %s", schema.Pattern),
				Type:    "pattern",
				Value:   data,
			})
		}
	}

	// 数字检查
	if num, ok := data.(float64); ok {
		if schema.Minimum != nil && num < *schema.Minimum {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("值不能小于 %f", *schema.Minimum),
				Type:    "minimum",
				Value:   data,
			})
		}
		if schema.Maximum != nil && num > *schema.Maximum {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("值不能大于 %f", *schema.Maximum),
				Type:    "maximum",
				Value:   data,
			})
		}
	}

	// 对象检查
	if obj, ok := data.(map[string]interface{}); ok {
		// Required 检查
		for _, req := range schema.Required {
			if _, exists := obj[req]; !exists {
				errors = append(errors, ValidationError{
					Field:   path + "." + req,
					Message: "缺少必需字段",
					Type:    "required",
				})
			}
		}

		// 属性验证
		for key, propSchema := range schema.Properties {
			if val, exists := obj[key]; exists {
				propErrors := v.validateSchema(propSchema, val, path+"."+key)
				errors = append(errors, propErrors...)
			}
		}
	}

	// 数组检查
	if arr, ok := data.([]interface{}); ok && schema.Items != nil {
		for i, item := range arr {
			itemErrors := v.validateSchema(schema.Items, item, fmt.Sprintf("%s[%d]", path, i))
			errors = append(errors, itemErrors...)
		}
	}

	return errors
}

// checkType 检查类型
func (v *SchemaValidator) checkType(expected string, data interface{}) bool {
	switch expected {
	case "string":
		_, ok := data.(string)
		return ok
	case "number":
		_, ok := data.(float64)
		return ok
	case "integer":
		_, ok := data.(int)
		if !ok {
			if f, ok := data.(float64); ok {
				return f == float64(int(f))
			}
		}
		return ok
	case "boolean":
		_, ok := data.(bool)
		return ok
	case "object":
		_, ok := data.(map[string]interface{})
		return ok
	case "array":
		_, ok := data.([]interface{})
		return ok
	case "null":
		return data == nil
	default:
		return true
	}
}

// GetStats 获取统计信息
func (v *SchemaValidator) GetStats() *ValidatorStats {
	return v.stats
}

// RemoveSchema 移除 Schema
func (v *SchemaValidator) RemoveSchema(id string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.schemas, id)
}

// GetSchema 获取 Schema
func (v *SchemaValidator) GetSchema(id string) *Schema {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.schemas[id]
}

// ListSchemas 列出所有 Schema
func (v *SchemaValidator) ListSchemas() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	ids := make([]string, 0, len(v.schemas))
	for id := range v.schemas {
		ids = append(ids, id)
	}
	return ids
}

// Helper functions
func countProperties(schema *Schema) int {
	count := len(schema.Properties)
	for _, prop := range schema.Properties {
		count += countProperties(prop)
	}
	return count
}

// MarshalJSON 序列化 Schema
func (s *Schema) MarshalJSON() ([]byte, error) {
	type Alias Schema
	return json.Marshal(&struct {
		*Alias
		Pattern string `json:"pattern,omitempty"`
	}{
		Alias:   (*Alias)(s),
		Pattern: s.Pattern,
	})
}

// ValidateRequest 验证请求
func (v *SchemaValidator) ValidateRequest(ctx context.Context, schemaID string, method string, body []byte, query map[string]string, headers map[string]string) *ValidationResult {
	v.stats.TotalValidations++

	var data interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			v.stats.FailedValidations++
			return &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Field:   "$body",
					Message: fmt.Sprintf("JSON 解析失败：%v", err),
					Type:    "parse_error",
				}},
			}
		}
	} else {
		data = make(map[string]interface{})
	}

	result := v.Validate(ctx, schemaID, data)

	if !result.Valid {
		v.stats.FailedValidations++
	} else {
		v.stats.SuccessfulReviews++
	}

	return result
}

// SanitizeInput 清理输入
func SanitizeInput(input string) string {
	// 移除 HTML 标签
	htmlTag := regexp.MustCompile(`<[^>]*>`)
	input = htmlTag.ReplaceAllString(input, "")

	// 转义特殊字符
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")

	return strings.TrimSpace(input)
}
