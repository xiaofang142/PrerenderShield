package seo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newChatServer 创建一个模拟 OpenAI chat/completions 协议的服务器
func newChatServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func chatJSON(content string) string {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"role": "assistant", "content": content}},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func newTestOptimizer(serverURL string, mutate func(*LLMConfig)) *LLMOptimizer {
	cfg := LLMConfig{
		Enabled:     true,
		Provider:    "openai",
		APIKey:      "sk-test",
		APIURL:      serverURL,
		Model:       "gpt-test",
		MaxTokens:   100,
		Temperature: 0.2,
		Timeout:     2 * time.Second,
		MaxRetries:  0,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	o := NewLLMOptimizer(cfg)
	// 构造器会把 <=0 的 MaxRetries 归一化为 2，这里还原测试意图（0 = 不重试，避免用例无谓休眠）
	o.config.MaxRetries = cfg.MaxRetries
	return o
}

func TestDefaultLLMConfig(t *testing.T) {
	cfg := DefaultLLMConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "openai", cfg.Provider)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", cfg.APIURL)
	assert.Equal(t, "gpt-4o-mini", cfg.Model)
	assert.Equal(t, 500, cfg.MaxTokens)
	assert.Equal(t, 0.3, cfg.Temperature)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, 2, cfg.MaxRetries)
	assert.Contains(t, cfg.Prompts.TitleOptimization, "%s")
	assert.Contains(t, cfg.Prompts.DescriptionOptimization, "%s")
	assert.Contains(t, cfg.Prompts.KeywordExtraction, "%s")
	assert.Contains(t, cfg.Prompts.StructuredData, "%s")
}

func TestNewLLMOptimizer_Defaults(t *testing.T) {
	// 零值配置应填充默认 Timeout/MaxRetries/MaxTokens
	o := NewLLMOptimizer(LLMConfig{Enabled: true})

	assert.Equal(t, 10*time.Second, o.config.Timeout)
	assert.Equal(t, 2, o.config.MaxRetries)
	assert.Equal(t, 500, o.config.MaxTokens)
	assert.NotNil(t, o.httpClient)
	assert.Equal(t, 10*time.Second, o.httpClient.Timeout)
}

func TestNewLLMOptimizer_CustomValuesPreserved(t *testing.T) {
	cfg := LLMConfig{
		Enabled:    true,
		Timeout:    3 * time.Second,
		MaxRetries: 5,
		MaxTokens:  200,
	}
	o := NewLLMOptimizer(cfg)

	assert.Equal(t, 3*time.Second, o.config.Timeout)
	assert.Equal(t, 5, o.config.MaxRetries)
	assert.Equal(t, 200, o.config.MaxTokens)
}

func TestLLMOptimizer_OptimizeTitle(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func(*LLMConfig)
		handler func(w http.ResponseWriter, r *http.Request)
		want    string
	}{
		{
			name: "disabled returns original",
			cfg:  func(c *LLMConfig) { c.Enabled = false },
			want: "原始标题",
		},
		{
			name: "success returns optimized",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON("优化后的长标题内容"))
			},
			want: "优化后的长标题内容",
		},
		{
			name: "too short result falls back to original",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON("short"))
			},
			want: "原始标题",
		},
		{
			name: "server error falls back to original",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			want: "原始标题",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.handler != nil {
					tt.handler(w, r)
				}
			})
			o := newTestOptimizer(server.URL, tt.cfg)

			got, err := o.OptimizeTitle("原始标题", "关键词")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLLMOptimizer_OptimizeTitle_ResultsTrimmed(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, chatJSON("  两端空白应被裁剪的长标题  "))
	})
	o := newTestOptimizer(server.URL, nil)

	got, err := o.OptimizeTitle("原始标题", "")
	require.NoError(t, err)
	assert.Equal(t, "两端空白应被裁剪的长标题", got)
}

func TestLLMOptimizer_OptimizeDescription(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(*LLMConfig)
		handler   func(w http.ResponseWriter, r *http.Request)
		wantDesc  string
		wantError string
	}{
		{
			name: "disabled returns empty without error",
			cfg:  func(c *LLMConfig) { c.Enabled = false },
		},
		{
			name: "success returns description",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON(strings.Repeat("描", 60)))
			},
			wantDesc: strings.Repeat("描", 60),
		},
		{
			name: "too short description returns error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON("太短的描述"))
			},
			wantError: "description too short",
		},
		{
			name: "server error returns error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantError: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.handler != nil {
					tt.handler(w, r)
				}
			})
			o := newTestOptimizer(server.URL, tt.cfg)

			got, err := o.OptimizeDescription("<p>页面内容</p>", "关键词")
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDesc, got)
		})
	}
}

func TestLLMOptimizer_ExtractKeywords(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(*LLMConfig)
		handler   func(w http.ResponseWriter, r *http.Request)
		want      []string
		wantError string
	}{
		{
			name: "disabled returns nil",
			cfg:  func(c *LLMConfig) { c.Enabled = false },
		},
		{
			name: "success returns keywords",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON(`["seo", "优化", "预渲染"]`))
			},
			want: []string{"seo", "优化", "预渲染"},
		},
		{
			name: "invalid json returns parse error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON("这不是 JSON"))
			},
			wantError: "failed to parse keywords",
		},
		{
			name: "server error returns error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantError: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.handler != nil {
					tt.handler(w, r)
				}
			})
			o := newTestOptimizer(server.URL, tt.cfg)

			got, err := o.ExtractKeywords("<p>页面内容</p>")
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLLMOptimizer_GenerateStructuredData(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(*LLMConfig)
		handler   func(w http.ResponseWriter, r *http.Request)
		want      map[string]interface{}
		wantError string
	}{
		{
			name: "disabled returns nil",
			cfg:  func(c *LLMConfig) { c.Enabled = false },
		},
		{
			name: "success returns schema",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON(`{"@type":"Product","name":"测试产品"}`))
			},
			want: map[string]interface{}{"@type": "Product", "name": "测试产品"},
		},
		{
			name: "invalid json returns parse error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, chatJSON("[1,2,3]"))
			},
			wantError: "failed to parse structured data",
		},
		{
			name: "server error returns error",
			cfg:  nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantError: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.handler != nil {
					tt.handler(w, r)
				}
			})
			o := newTestOptimizer(server.URL, tt.cfg)

			got, err := o.GenerateStructuredData("Product", "<p>内容</p>")
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLLMOptimizer_CallLLM_RetryThenSuccess(t *testing.T) {
	attempts := 0
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, chatJSON("重试成功的结果内容"))
	})

	// MaxRetries=1：第一次失败后休眠 1 秒再重试
	o := newTestOptimizer(server.URL, func(c *LLMConfig) { c.MaxRetries = 1 })

	result, err := o.callLLM("测试提示词")
	require.NoError(t, err)
	assert.Equal(t, "重试成功的结果内容", result)
	assert.Equal(t, 2, attempts)
}

func TestLLMOptimizer_CallLLM_AllAttemptsFail(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	o := newTestOptimizer(server.URL, func(c *LLMConfig) { c.MaxRetries = 1 })

	result, err := o.callLLM("测试提示词")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 2 attempts failed")
	assert.Empty(t, result)
}

func TestLLMOptimizer_DoRequest_APIKeyHeader(t *testing.T) {
	var gotAuth string
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, chatJSON("ok"))
	})
	o := newTestOptimizer(server.URL, nil)

	result, err := o.doRequest([]byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, "Bearer sk-test", gotAuth)
}

func TestLLMOptimizer_DoRequest_InvalidURL(t *testing.T) {
	// URL 中含空格，http.NewRequest 解析失败
	o := newTestOptimizer("http://exa mple.com/api", nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
}

func TestLLMOptimizer_DoRequest_ConnectionRefused(t *testing.T) {
	// 无监听端口，连接被拒绝
	o := newTestOptimizer("http://127.0.0.1:1/v1/chat/completions", nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed")
}

func TestLLMOptimizer_DoRequest_Non200Status(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "forbidden body")
	})
	o := newTestOptimizer(server.URL, nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
	assert.Contains(t, err.Error(), "forbidden body")
}

func TestLLMOptimizer_DoRequest_InvalidJSON(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not json at all")
	})
	o := newTestOptimizer(server.URL, nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestLLMOptimizer_DoRequest_APIErrorField(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"error":{"message":"配额已用尽"}}`)
	})
	o := newTestOptimizer(server.URL, nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
	assert.Contains(t, err.Error(), "配额已用尽")
}

func TestLLMOptimizer_DoRequest_NoChoices(t *testing.T) {
	server := newChatServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices":[]}`)
	})
	o := newTestOptimizer(server.URL, nil)

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices in response")
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes script style nav footer aside",
			input: "<script>evil()</script><style>.x{}</style><nav>导航</nav>正文<footer>页脚</footer><aside>侧栏</aside>",
			want:  "正文",
		},
		{
			name:  "case insensitive tag match",
			input: "<SCRIPT>x</SCRIPT>正文",
			want:  "正文",
		},
		{
			name:  "unclosed tag ignored then stripped",
			input: "<script>x<body>正文",
			want:  "x 正文",
		},
		{
			name:  "remaining tags replaced with space",
			input: "<p>词一</p><p>词二</p>",
			want:  "词一  词二",
		},
		{
			name:  "tag without closing bracket",
			input: "正文 <broken",
			want:  "正文 <broken",
		},
		{
			name:  "plain text unchanged",
			input: "  纯文本  ",
			want:  "纯文本",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripHTMLTags(tt.input))
		})
	}
}

func TestTruncateText(t *testing.T) {
	assert.Equal(t, "短文本", truncateText("短文本", 10))
	assert.Equal(t, "abc", truncateText("abc", 3))

	// 多字节字符按 rune 截断
	long := strings.Repeat("汉", 20)
	got := truncateText(long, 5)
	assert.Equal(t, strings.Repeat("汉", 5)+"...", got)
	assert.Len(t, got, 5*3+3)
}

type errBodyReader struct{}

func (errBodyReader) Read(p []byte) (int, error) { return 0, fmt.Errorf("read failed") }
func (errBodyReader) Close() error               { return nil }

type seoRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f seoRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLLMOptimizer_DoRequest_ReadBodyError(t *testing.T) {
	o := newTestOptimizer("http://any.example.com/api", nil)
	o.httpClient.Transport = seoRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(errBodyReader{}),
		}, nil
	})

	_, err := o.doRequest([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read failed")
}
