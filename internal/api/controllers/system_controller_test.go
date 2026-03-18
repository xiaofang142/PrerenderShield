package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSystemRedisClient is a mock implementation of SystemRedisClient
type MockSystemRedisClient struct {
	mock.Mock
}

func (m *MockSystemRedisClient) Context() context.Context {
	return context.Background()
}

func (m *MockSystemRedisClient) GetRawClient() (RawRedisClient, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(RawRedisClient), args.Error(1)
}

func (m *MockSystemRedisClient) SetMembers(key string) ([]string, error) {
	args := m.Called(key)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSystemRedisClient) GetJSON(key string, value interface{}) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockSystemRedisClient) GetSystemConfig() (map[string]string, error) {
	args := m.Called()
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockSystemRedisClient) SaveSystemConfig(config map[string]interface{}) error {
	args := m.Called(config)
	return args.Error(0)
}

// MockRawRedisClient for redis operations
type MockRawRedisClient struct {
	mock.Mock
}

func (m *MockRawRedisClient) Ping(ctx context.Context) RedisStatus {
	args := m.Called(ctx)
	return args.Get(0).(RedisStatus)
}

// MockRedisStatus for mocking ping results
type MockRedisStatus struct {
	err error
}

func (m *MockRedisStatus) Err() error {
	return m.err
}

func setupSystemControllerWithMock() (*SystemController, *MockSystemRedisClient, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	mockRedis := new(MockSystemRedisClient)
	controller := &SystemController{
		redisClient: mockRedis,
	}

	router := gin.New()
	router.GET("/health", controller.Health)
	router.GET("/version", controller.Version)
	router.GET("/system/config", controller.GetSystemConfig)
	router.POST("/system/config", controller.UpdateSystemConfig)

	return controller, mockRedis, router
}

func TestSystemController_Health_NoRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/health", controller.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "running", data["status"])
	assert.Equal(t, "prerender-shield", data["service"])
	assert.Equal(t, "unknown", data["redis_status"])
	assert.Equal(t, "unknown", data["ssl_status"])
	assert.Equal(t, float64(0), data["expiring_certs"])
	assert.NotNil(t, data["health_details"])
}

func TestSystemController_Version_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/version", controller.Version)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/version", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, data["version"])
	assert.NotEmpty(t, data["name"])
}

func TestSystemController_GetSystemConfig_NoRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/system/config", controller.GetSystemConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_UpdateSystemConfig_NoRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSystemController(nil)

	router := gin.New()
	router.POST("/system/config", controller.UpdateSystemConfig)

	configData := map[string]interface{}{
		"access_log_retention_days":  "30",
		"crawler_log_retention_days": "30",
	}
	body, _ := json.Marshal(configData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_UpdateSystemConfig_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewSystemController(nil)

	router := gin.New()
	router.POST("/system/config", controller.UpdateSystemConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_Health_WithRedisConnected(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock raw redis client
	mockRawClient := new(MockRawRedisClient)
	mockRawClient.On("Ping", mock.Anything).Return(&MockRedisStatus{err: nil})

	// Mock SetMembers to return empty list (no SSL certs)
	mockRedis.On("GetRawClient").Return(mockRawClient, nil)
	mockRedis.On("SetMembers", "ssl:certs").Return([]string{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "running", data["status"])
	assert.Equal(t, "connected", data["redis_status"])
	assert.Equal(t, "no_certificates", data["ssl_status"])
}

func TestSystemController_Health_WithRedisDisconnected(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock raw redis client with error
	mockRawClient := new(MockRawRedisClient)
	mockRawClient.On("Ping", mock.Anything).Return(&MockRedisStatus{err: assert.AnError})

	mockRedis.On("GetRawClient").Return(mockRawClient, nil)
	mockRedis.On("SetMembers", "ssl:certs").Return([]string{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "degraded", data["status"])
	assert.Equal(t, "disconnected", data["redis_status"])
}

func TestSystemController_Health_WithExpiringCerts(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock raw redis client
	mockRawClient := new(MockRawRedisClient)
	mockRawClient.On("Ping", mock.Anything).Return(&MockRedisStatus{err: nil})

	// Mock SSL certs with expiring certificate
	mockRedis.On("GetRawClient").Return(mockRawClient, nil)
	mockRedis.On("SetMembers", "ssl:certs").Return([]string{"example.com"}, nil)

	// Mock cert info with expiry date in 15 days
	expiryTime := time.Now().Add(15 * 24 * time.Hour).Unix()
	mockRedis.On("GetJSON", "ssl:cert:example.com", mock.Anything).Run(func(args mock.Arguments) {
		val := args.Get(1).(*map[string]interface{})
		*val = map[string]interface{}{
			"expires_at": float64(expiryTime),
		}
	}).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "warning", data["status"])
	assert.Equal(t, float64(1), data["expiring_certs"])
}

func TestSystemController_GetSystemConfig_WithRedis(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock config response
	mockConfig := map[string]string{
		"access_log_retention_days":  "30",
		"crawler_log_retention_days": "14",
	}
	mockRedis.On("GetSystemConfig").Return(mockConfig, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "30", data["access_log_retention_days"])
	assert.Equal(t, "14", data["crawler_log_retention_days"])
}

func TestSystemController_GetSystemConfig_EmptyConfig(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock empty config response
	mockRedis.On("GetSystemConfig").Return(map[string]string{}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "7", data["access_log_retention_days"])
	assert.Equal(t, "128", data["access_log_max_size"])
}

func TestSystemController_GetSystemConfig_Error(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock error response
	mockRedis.On("GetSystemConfig").Return(map[string]string{}, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_UpdateSystemConfig_Success(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock successful save
	configData := map[string]interface{}{
		"access_log_retention_days":  "30",
		"crawler_log_retention_days": "14",
	}
	mockRedis.On("SaveSystemConfig", configData).Return(nil)

	body, _ := json.Marshal(configData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "System config updated successfully", response["message"].(string))
}

func TestSystemController_UpdateSystemConfig_Error(t *testing.T) {
	_, mockRedis, router := setupSystemControllerWithMock()

	// Mock save error
	configData := map[string]interface{}{
		"access_log_retention_days": "30",
	}
	mockRedis.On("SaveSystemConfig", configData).Return(assert.AnError)

	body, _ := json.Marshal(configData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNewSystemController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test with nil redis client
	controller := NewSystemController(nil)
	assert.NotNil(t, controller)
	assert.Nil(t, controller.redisClient)
}
