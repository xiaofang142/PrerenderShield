package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/auth"
)

// AuthController 认证控制器
type AuthController struct {
	userManager *auth.UserManager
	jwtManager  *auth.JWTManager
}

// NewAuthController 创建认证控制器实例
func NewAuthController(userManager *auth.UserManager, jwtManager *auth.JWTManager) *AuthController {
	return &AuthController{
		userManager: userManager,
		jwtManager:  jwtManager,
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

	// 检查是否是首次登录
	if c.userManager.IsFirstRun() {
		// 首次登录，创建用户
		user, err = c.userManager.CreateUser(req.Username, req.Password)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "Failed to create user: " + err.Error(),
			})
			return
		}
	} else {
		// 非首次登录，验证用户
		user, err = c.userManager.AuthenticateUser(req.Username, req.Password)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "Invalid username or password",
			})
			return
		}
	}

	// 生成JWT令牌
	token, err := c.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to generate token",
		})
		return
	}

	// 返回登录成功响应
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Login successful",
		"data": gin.H{
			"token":    token,
			"username": user.Username,
		},
	})
}

// Logout 用户退出登录
func (c *AuthController) Logout(ctx *gin.Context) {
	// JWT是无状态的，退出登录只需要前端清除token即可
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Logout successful",
	})
}
