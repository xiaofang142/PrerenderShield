package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSystemRedis 可编程 SystemRedisClient mock（仅覆盖现有 mock 未触及的分支）
type fakeSystemRedis struct {
	getRawErr     bool
	setMembersErr bool
	sysConfigErr  bool
	saveErr       bool
	getErr        bool
	setErr        bool
	keysErr       bool
	keys          []string
	values        map[string]string
}

func (f *fakeSystemRedis) Context() context.Context { return context.Background() }

func (f *fakeSystemRedis) GetRawClient() (RawRedisClient, error) {
	if f.getRawErr {
		return nil, errors.New("raw unavailable")
	}
	return nil, nil
}

func (f *fakeSystemRedis) SetMembers(key string) ([]string, error) {
	if f.setMembersErr {
		return nil, errors.New("smembers failed")
	}
	return []string{}, nil
}

func (f *fakeSystemRedis) GetJSON(key string, value interface{}) error {
	return errors.New("getjson failed")
}

func (f *fakeSystemRedis) GetSystemConfig() (map[string]string, error) {
	if f.sysConfigErr {
		return nil, errors.New("hgetall failed")
	}
	return map[string]string{}, nil
}

func (f *fakeSystemRedis) SaveSystemConfig(config map[string]interface{}) error {
	if f.saveErr {
		return errors.New("save failed")
	}
	return nil
}

func (f *fakeSystemRedis) Get(key string) (string, error) {
	if f.getErr {
		return "", errors.New("get failed")
	}
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func (f *fakeSystemRedis) Set(key string, value interface{}, expiration time.Duration) error {
	if f.setErr {
		return errors.New("set failed")
	}
	return nil
}

func (f *fakeSystemRedis) Keys(pattern string) ([]string, error) {
	if f.keysErr {
		return nil, errors.New("keys failed")
	}
	return f.keys, nil
}

func setupFakeSystem(fake *fakeSystemRedis) (*SystemController, *gin.Engine) {
	controller := &SystemController{redisClient: fake}
	router := ginNewRouter()
	router.GET("/health", controller.Health)
	router.POST("/system/config", controller.UpdateSystemConfig)
	router.POST("/system/backup", controller.BackupConfig)
	router.POST("/system/restore", controller.RestoreConfig)
	router.GET("/system/backups", controller.ListBackups)
	return controller, router
}

// TestSystemController_Health_SSLNotConfigured SetMembers 失败 → not_configured
func TestSystemController_Health_SSLNotConfigured(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{setMembersErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ssl_status":"not_configured"`)
}

// TestSystemController_Health_RawUnavailable GetRawClient 失败 → redis unknown
func TestSystemController_Health_RawUnavailable(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{getRawErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"redis_status":"unknown"`)
}

// TestSystemController_Health_ChromiumNotFound Chromium 解析失败 → degraded/not_found
func TestSystemController_Health_ChromiumNotFound(t *testing.T) {
	t.Setenv("PRERENDER_CHROMIUM_PATH", "/nonexistent/chromium-binary")
	t.Setenv("CHROME_PATH", "")

	_, router := setupFakeSystem(&fakeSystemRedis{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"chromium":"not_found"`)
	assert.Contains(t, body, `"status":"degraded"`)
}

func TestSystemController_BackupConfig_NilRedis(t *testing.T) {
	controller := NewSystemController(nil)
	router := ginNewRouter()
	router.POST("/system/backup", controller.BackupConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/system/backup", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_BackupConfig_ReadError(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{sysConfigErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/system/backup", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_BackupConfig_SetError(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{setErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/system/backup", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_BackupConfig_Success(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/system/backup", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "system:backup:")
}

// TestSystemController_RestoreConfig 成功恢复 + 缺 key + 备份不存在
func TestSystemController_RestoreConfig(t *testing.T) {
	backupPayload, _ := json.Marshal(map[string]interface{}{
		"config":    map[string]interface{}{"log_days": "14"},
		"timestamp": "2026-01-01T00:00:00Z",
	})

	_, router := setupFakeSystem(&fakeSystemRedis{
		values: map[string]string{"system:backup:20260101": string(backupPayload)},
	})

	// 缺 backup_key → 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 备份不存在 → 404
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "ghost"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 成功恢复
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "system:backup:20260101"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "restored")
}

// TestSystemController_RestoreConfig_InvalidPayload 备份内容损坏 → 500
func TestSystemController_RestoreConfig_InvalidPayload(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{
		values: map[string]string{"system:backup:bad": "not-json"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "system:backup:bad"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSystemController_RestoreConfig_MissingConfigField 备份缺 config 字段 → 500
func TestSystemController_RestoreConfig_MissingConfigField(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{"timestamp": "x"})
	_, router := setupFakeSystem(&fakeSystemRedis{
		values: map[string]string{"system:backup:noconfig": string(payload)},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "system:backup:noconfig"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_ListBackups(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{"timestamp": "2026-01-01T00:00:00Z"})

	_, router := setupFakeSystem(&fakeSystemRedis{
		keys:   []string{"system:backup:a", "system:backup:b"},
		values: map[string]string{"system:backup:a": string(payload), "system:backup:b": "not-json"},
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/system/backups", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// 只有合法 JSON 的备份 a 出现
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "system:backup:a", resp.Data[0]["key"])
}

func TestSystemController_ListBackups_NilRedis(t *testing.T) {
	controller := NewSystemController(nil)
	router := ginNewRouter()
	router.GET("/system/backups", controller.ListBackups)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/system/backups", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":[]`)
}

func TestSystemController_ListBackups_KeysError(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{keysErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/system/backups", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":[]`)
}

// TestSystemController_Wrappers systemRedisWrapper 全部委托方法直连 redis.Client
func TestSystemController_Wrappers(t *testing.T) {
	client := newTestRedisDB15(t)
	controller := NewSystemController(client)
	require.NotNil(t, controller.redisClient)
	w := &systemRedisWrapper{client: client}
	ctx := w.Context()
	require.NotNil(t, ctx)

	raw, err := w.GetRawClient()
	require.NoError(t, err)
	require.NotNil(t, raw)
	assert.NoError(t, raw.Ping(ctx).Err())

	members, err := w.SetMembers("test:wrappers:set")
	require.NoError(t, err)
	assert.NotNil(t, members)

	var dest map[string]interface{}
	// 不存在的键返回空值不报错（GetJSON 语义），已保存的 JSON 应能读回
	assert.NoError(t, w.GetJSON("test:wrappers:json", &dest))

	cfg, err := w.GetSystemConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// SaveSystemConfig 写入固定键 system:config（redis 包测试共用 DB15），用后立即清理避免跨包干扰
	require.NoError(t, w.SaveSystemConfig(map[string]interface{}{"k": "v"}))
	client.Del("system:config")

	require.NoError(t, w.Set("test:wrappers:key", "v", time.Minute))
	val, err := w.Get("test:wrappers:key")
	require.NoError(t, err)
	assert.Equal(t, "v", val)

	keys, err := w.Keys("test:wrappers:*")
	require.NoError(t, err)
	assert.NotNil(t, keys)
}

// TestSystemController_WrapperGetError closed client 触发包装层错误路径
func TestSystemController_WrapperGetError(t *testing.T) {
	client := closedTestRedisDB15(t)
	w := &systemRedisWrapper{client: client}

	_, err := w.Get("anything")
	assert.Error(t, err)

	assert.Error(t, w.Set("anything", "v", time.Minute))

	_, err = w.SetMembers("anything")
	assert.Error(t, err)

	_, err = w.GetSystemConfig()
	assert.Error(t, err)

	assert.Error(t, w.SaveSystemConfig(map[string]interface{}{}))

	_, err = w.Keys("anything*")
	assert.Error(t, err)

	var dest map[string]interface{}
	assert.Error(t, w.GetJSON("anything", &dest))
}
