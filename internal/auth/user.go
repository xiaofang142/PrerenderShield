package auth

import (
	"errors"

	"prerender-shield/internal/redis"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// 错误定义
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserExists         = errors.New("user already exists")
)

// User 用户信息
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"` // 存储加密后的密码
}

// UserManager 用户管理器
type UserManager struct {
	// 移除内存缓存，直接从Redis读取数据
	redisClient *redis.Client
}

// NewUserManager 创建用户管理器
func NewUserManager(_ string, redisClient *redis.Client) *UserManager {
	return &UserManager{
		redisClient: redisClient,
	}
}

// CreateUser 创建用户
func (m *UserManager) CreateUser(username, password string) (*User, error) {
	// 检查是否已经有用户存在（只允许创建一个用户）
	if m.redisClient != nil {
		userIDs, err := m.redisClient.GetAllUsers()
		if err == nil && len(userIDs) > 0 {
			return nil, errors.New("system already initialized, only one user is allowed")
		}
	}

	// 生成用户ID
	userID := uuid.New().String()

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &User{
		ID:       userID,
		Username: username,
		Password: string(hashedPassword),
	}

	// 直接保存用户到Redis
	if m.redisClient != nil {
		if err := m.redisClient.SaveUser(user.ID, user.Username, user.Password); err != nil {
			return nil, err
		}
	}

	return user, nil
}

// GetUserByUsername 通过用户名获取用户
func (m *UserManager) GetUserByUsername(username string) (*User, error) {
	// 直接从Redis中获取用户数据
	if m.redisClient == nil {
		return nil, ErrUserNotFound
	}

	// 通过用户名获取用户ID
	userID, err := m.redisClient.GetUserByUsername(username)
	if err != nil || userID == "" {
		return nil, ErrUserNotFound
	}

	// 通过用户ID获取用户数据
	userData, err := m.redisClient.GetUser(userID)
	if err != nil || len(userData) == 0 {
		return nil, ErrUserNotFound
	}

	// 创建用户对象
	user := &User{
		ID:       userData["id"],
		Username: userData["username"],
		Password: userData["password"],
	}

	return user, nil
}

// AuthenticateUser 验证用户身份
func (m *UserManager) AuthenticateUser(username, password string) (*User, error) {
	// 获取用户
	user, err := m.GetUserByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		// 密码验证失败，返回错误
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// IsFirstRun 检查是否是首次运行（没有用户）
func (m *UserManager) IsFirstRun() bool {
	// 直接检查Redis中是否存在用户数据
	if m.redisClient != nil {
		// 获取所有用户ID
		userIDs, err := m.redisClient.GetAllUsers()
		if err == nil && len(userIDs) > 0 {
			// Redis中有用户数据，不是首次运行
			return false
		}
	}

	// Redis中没有用户数据，或者Redis不可用，返回首次运行
	return true
}
