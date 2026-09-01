package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
)

// AuthController 认证控制器
type AuthController struct {
	userManager   *auth.UserManager
	jwtManager    *auth.JWTManager
	auditLogger   *audit.Logger
	twoFactorAuth *auth.TwoFactorAuth
}

// NewAuthController 创建认证控制器实例
func NewAuthController(userManager *auth.UserManager, jwtManager *auth.JWTManager, auditLogger *audit.Logger, twoFactorAuth *auth.TwoFactorAuth) *AuthController {
	return &AuthController{
		userManager:   userManager,
		jwtManager:    jwtManager,
		auditLogger:   auditLogger,
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
		// P0-23: First run 仍然自动创建首个管理员
		// (后续可由该管理员通过 Admin API 创建更多用户)
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

	// R16-BUG-1 修复：2FA 启用时登录必须完成第二因子验证。
	// 此前密码通过即发正式 JWT，2FA 形同虚设。现签发短时 tmp_token
	//（仅可调 /auth/2fa/verify），验证通过后才发正式 JWT。
	if c.twoFactorAuth != nil {
		if enabled, err := c.twoFactorAuth.Is2FAEnabled(user.ID); err == nil && enabled {
			nonce, err := c.twoFactorAuth.LoginChallenge(user.ID, user.Username, 5*time.Minute)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to start 2FA challenge"})
				return
			}
			tmpToken, err := c.jwtManager.Generate2FAToken(user.ID, user.Username, nonce)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to generate 2FA token"})
				return
			}
			ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "2FA required", "data": gin.H{
				"require_2fa": true,
				"tmp_token":   tmpToken,
				"username":    user.Username,
			}})
			return
		}
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

// VerifyLogin2FA 登录第二因子验证（R16-BUG-1）：公开端点。
// 校验 tmp_token（签名+5min TTL+2fa- 前缀，不查会话），消耗登录挑战 nonce（防重放），
// 校验 TOTP 后签发正式 JWT。
func (c *AuthController) VerifyLogin2FA(ctx *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Verification code is required"})
		return
	}
	authHeader := ctx.GetHeader("Authorization")
	if len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Unauthorized"})
		return
	}
	uid, uname, nonce, err := c.jwtManager.Validate2FAToken(authHeader[7:])
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Invalid or expired 2FA challenge"})
		return
	}
	// 防重放：挑战 nonce 必须仍在且匹配，验证后立即删除
	if err := c.twoFactorAuth.CheckLoginNonce(uid, nonce); err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Login challenge expired or already used"})
		return
	}
	if err := c.twoFactorAuth.Verify2FA(uid, req.Code); err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Invalid verification code"})
		return
	}
	token, err := c.jwtManager.GenerateToken(uid, uname)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to generate token"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Login successful", "data": gin.H{"token": token, "username": uname}})
}
