package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/auth"
)

func setupAuthController() (*AuthController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 使用 nil redis client 进行单元测试
	userManager := auth.NewUserManager("", nil)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000, // 1 hour in nanoseconds
	}, nil)

	controller := NewAuthController(userManager, jwtManager)

	router := gin.New()
	router.POST("/auth/first-run", controller.CheckFirstRun)
	router.POST("/auth/login", controller.Login)
	router.POST("/auth/logout", controller.Logout)

	return controller, router
}

func TestAuthController_CheckFirstRun(t *testing.T) {
	_, router := setupAuthController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/first-run", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"])

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["isFirstRun"])
}

func TestAuthController_Login_InvalidRequest(t *testing.T) {
	_, router := setupAuthController()

	// 测试空请求体
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_Login_FirstRun(t *testing.T) {
	_, router := setupAuthController()

	loginData := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	body, _ := json.Marshal(loginData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Contains(t, response["message"], "Login successful")

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, data["token"])
	assert.Equal(t, "admin", data["username"])
}

func TestAuthController_Logout_NoToken(t *testing.T) {
	_, router := setupAuthController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/logout", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthController_Login_InvalidCredentials(t *testing.T) {
	_, router := setupAuthController()

	loginData := map[string]string{
		"username": "invalid",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(loginData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 首次运行时会自动创建用户，所以返回 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthController_Login_MissingFields(t *testing.T) {
	_, router := setupAuthController()

	loginData := map[string]string{
		"username": "admin",
	}
	body, _ := json.Marshal(loginData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userManager := auth.NewUserManager("", nil)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, nil)

	controller := NewAuthController(userManager, jwtManager)

	assert.NotNil(t, controller)
	assert.NotNil(t, controller.userManager)
	assert.NotNil(t, controller.jwtManager)
}
