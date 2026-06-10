package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
)

// AuthController 认证控制器
type AuthController struct {
	userManager *auth.UserManager
	jwtManager  *auth.JWTManager
	auditLogger *audit.Logger
}

// NewAuthController 创建认证控制器实例
func NewAuthController(userManager *auth.UserManager, jwtManager *auth.JWTManager, auditLogger *audit.Logger) *AuthController {
	return &AuthController{
		userManager: userManager,
		jwtManager:  jwtManager,
		auditLogger: auditLogger,
	}
}

// CheckFirstRun 检查是否是首次运行
func (c *AuthController) CheckFirstRun(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"isFirstRun": c.userManager.IsFirstRun(),
		},
	})
}

// Login 用户登录
func (c *AuthController) Login(ctx *gin.Context) {
	// 解析请求
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Invalid request"})
		return
	}

	var user *auth.User
	var err error
	clientIP := ctx.ClientIP()

	if c.userManager.IsFirstRun() {
		user, err = c.userManager.CreateUser(req.Username, req.Password)
		if err != nil {
			if c.auditLogger != nil {
				c.auditLogger.Log(audit.Entry{UserID: req.Username, Action: audit.ActionLogin, Resource: "system", Detail: "first run user creation failed", Severity: audit.SeverityWarning, ClientIP: clientIP, Status: "failed"})
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "Failed to create user: " + err.Error()})
			return
		}
	} else {
		user, err = c.userManager.AuthenticateUser(req.Username, req.Password)
		if err != nil {
			if c.auditLogger != nil {
				c.auditLogger.Log(audit.Entry{UserID: req.Username, Action: audit.ActionLogin, Resource: "system", Detail: "authentication failed", Severity: audit.SeverityWarning, ClientIP: clientIP, Status: "denied"})
			}
			ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid username or password"})
			return
		}
	}

	token, err := c.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "Failed to generate token"})
		return
	}

	if c.auditLogger != nil {
		c.auditLogger.Log(audit.Entry{UserID: user.ID, Action: audit.ActionLogin, Resource: "system", Detail: "login successful", ClientIP: clientIP, Status: "success"})
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Login successful", "data": gin.H{"token": token, "username": user.Username}})
}

// Logout 用户退出登录
func (c *AuthController) Logout(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	authHeader := ctx.GetHeader("Authorization")
	if authHeader != "" && len(authHeader) > 7 {
		token := authHeader[7:]
		c.jwtManager.RevokeToken(token)
	}

	if c.auditLogger != nil {
		uid, _ := userID.(string)
		c.auditLogger.Log(audit.Entry{UserID: uid, Action: audit.ActionLogout, Resource: "system", Detail: "logout", ClientIP: ctx.ClientIP(), Status: "success"})
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Logout successful"})
}
