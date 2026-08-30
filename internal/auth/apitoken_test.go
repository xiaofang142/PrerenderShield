package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	raw, hash, err := GenerateToken()
	require.NoError(t, err)
	assert.True(t, len(raw) > len(apiTokenPrefix), "raw token must have prefix + payload")
	assert.Equal(t, apiTokenPrefix, raw[:len(apiTokenPrefix)])
	assert.Len(t, hash, 64, "sha256 hex")
	// hash 必须与 raw 对得上
	assert.Equal(t, HashToken(raw), hash)
	// 两次生成不重复
	raw2, hash2, _ := GenerateToken()
	assert.NotEqual(t, raw, raw2)
	assert.NotEqual(t, hash, hash2)
}

func TestVerifyToken(t *testing.T) {
	raw, hash, _ := GenerateToken()

	assert.True(t, VerifyToken(raw, []string{hash}))
	assert.True(t, VerifyToken(raw, []string{"deadbeef", hash}))
	assert.False(t, VerifyToken("pst_wrong", []string{hash}))
	assert.False(t, VerifyToken(raw, nil), "nil hashes = disabled")
	assert.False(t, VerifyToken("", []string{hash}))
}

func TestJWTAuthMiddleware_APITokenFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &JWTConfig{SecretKey: "test-secret", ExpireTime: time.Hour}
	manager := NewJWTManager(config, nil)
	raw, hash, _ := GenerateToken()
	middleware := JWTAuthMiddleware(manager, func() []string { return []string{hash} })

	// 命中：preheat 前缀 + 有效 API Token → 放行
	t.Run("PreheatPathWithValidToken", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/preheat/invalidate", nil)
		c.Request.Header.Set("Authorization", "Bearer "+raw)

		middleware(c)
		assert.False(t, c.IsAborted())
		assert.Equal(t, "api_token", c.GetString("auth_via"))
	})

	// 拒绝：非 preheat 路径即使 Token 有效也不放行
	t.Run("NonPreheatPathRejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/sites", nil)
		c.Request.Header.Set("Authorization", "Bearer "+raw)

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 拒绝：preheat 路径但 Token 不在配置列表
	t.Run("PreheatPathUnknownToken", func(t *testing.T) {
		rawOther, _, _ := GenerateToken()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/preheat/recache", nil)
		c.Request.Header.Set("Authorization", "Bearer "+rawOther)

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 回退禁用：nil 哈希列表时 API Token 一律 401
	t.Run("FallbackDisabled", func(t *testing.T) {
		mw := JWTAuthMiddleware(manager, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/preheat/invalidate", nil)
		c.Request.Header.Set("Authorization", "Bearer "+raw)

		mw(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
