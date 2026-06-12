package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
)

// AuthController 认证控制器
type AuthController struct {
	userManager  *auth.UserManager
	jwtManager   *auth.JWTManager
	auditLogger  *audit.Logger
	twoFactorAuth *auth.TwoFactorAuth
}

// NewAuthController 创建认证控制器实例
func NewAuthController(userManager *auth.UserManager, jwtManager *auth.JWTManager, auditLogger *audit.Logger, twoFactorAuth *auth.TwoFactorAuth) *AuthController {
	return &AuthController{
		userManager:  userManager,
		jwtManager:   jwtManager,
		auditLogger:  auditLogger,
		twoFactorAuth: twoFactorAuth,
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

	forceChange := false
	if !c.userManager.IsFirstRun() {
		forceChange = c.userManager.IsDefaultPassword(user.ID)
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Login successful", "data": gin.H{"token": token, "username": user.Username, "force_change_password": forceChange}})
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

// Get2FAStatus 获取 2FA 状态
func (c *AuthController) Get2FAStatus(ctx *gin.Context) {
	if c.twoFactorAuth == nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": gin.H{"enabled": false, "available": false}})
		return
	}
	userID, _ := ctx.Get("user_id")
	uid, _ := userID.(string)
	enabled, _ := c.twoFactorAuth.Is2FAEnabled(uid)
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": gin.H{"enabled": enabled, "available": true}})
}

// Enable2FA 开启 2FA
func (c *AuthController) Enable2FA(ctx *gin.Context) {
	if c.twoFactorAuth == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "2FA not configured"})
		return
	}
	userID, _ := ctx.Get("user_id")
	uid, _ := userID.(string)
	secret, qrURL, err := c.twoFactorAuth.Enable2FA(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": gin.H{"secret": secret, "qr_code_url": qrURL}})
}

// Confirm2FA 确认 2FA 并激活
func (c *AuthController) Confirm2FA(ctx *gin.Context) {
	if c.twoFactorAuth == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "2FA not configured"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Missing code"})
		return
	}
	userID, _ := ctx.Get("user_id")
	uid, _ := userID.(string)
	if err := c.twoFactorAuth.Confirm2FA(uid, req.Code); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "2FA enabled successfully"})
}

// Disable2FA 关闭 2FA
func (c *AuthController) Disable2FA(ctx *gin.Context) {
	if c.twoFactorAuth == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "2FA not configured"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Missing code"})
		return
	}
	userID, _ := ctx.Get("user_id")
	uid, _ := userID.(string)
	if err := c.twoFactorAuth.Disable2FA(uid, req.Code); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "2FA disabled successfully"})
}

// ChangePassword 修改密码
func (c *AuthController) ChangePassword(ctx *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request: password must be at least 6 characters"})
		return
	}

	userID, _ := ctx.Get("user_id")
	uid, _ := userID.(string)

	if err := c.userManager.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		statusCode := http.StatusBadRequest
		message := err.Error()
		if err == auth.ErrInvalidCredentials {
			message = "Current password is incorrect"
		} else if err == auth.ErrUserNotFound {
			statusCode = http.StatusNotFound
		}
		ctx.JSON(statusCode, gin.H{"code": statusCode, "message": message})
		return
	}

	if c.auditLogger != nil {
		c.auditLogger.Log(audit.Entry{UserID: uid, Action: audit.ActionLogin, Resource: "system", Detail: "password changed", ClientIP: ctx.ClientIP(), Status: "success"})
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Password changed successfully"})
}

// ListUsers 列出所有用户
func (c *AuthController) ListUsers(ctx *gin.Context) {
	users, err := c.userManager.ListUsers()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	// 隐藏密码字段
	type UserVO struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	var result []UserVO
	for _, u := range users {
		result = append(result, UserVO{ID: u.ID, Username: u.Username})
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": result})
}

// DeleteUser 删除用户
func (c *AuthController) DeleteUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	if userID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "User ID is required"})
		return
	}
	// 不允许删除自己
	currentUserID, _ := ctx.Get("user_id")
	if currentUserID == userID {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Cannot delete yourself"})
		return
	}
	if err := c.userManager.DeleteUser(userID); err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "User deleted successfully"})
}

// ResetUserPassword 管理员重置用户密码
func (c *AuthController) ResetUserPassword(ctx *gin.Context) {
	userID := ctx.Param("id")
	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request: password must be at least 6 characters"})
		return
	}
	if err := c.userManager.ResetPassword(userID, req.NewPassword); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Password reset successfully"})
}
