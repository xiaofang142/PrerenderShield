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
	users       map[string]*User
	redisClient *redis.Client
}

// NewUserManager 创建用户管理器
func NewUserManager(_ string, redisClient *redis.Client) *UserManager {
	manager := &UserManager{
		users:       make(map[string]*User),
		redisClient: redisClient,
	}

	// 加载用户数据
	manager.loadUsers()

	return manager
}

// CreateUser 创建用户
func (m *UserManager) CreateUser(username, password string) (*User, error) {
	// 检查是否已经有用户存在（只允许创建一个用户）
	if len(m.users) > 0 {
		return nil, errors.New("system already initialized, only one user is allowed")
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

	// 保存用户到内存
	m.users[userID] = user

	// 保存用户到Redis和文件
	if err := m.saveUsers(); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByUsername 通过用户名获取用户
func (m *UserManager) GetUserByUsername(username string) (*User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, ErrUserNotFound
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
	// 重新加载用户数据，确保与Redis保持同步
	m.loadUsers()
	// 返回内存中用户数量是否为0
	return len(m.users) == 0
}

// loadUsers 加载用户数据
func (m *UserManager) loadUsers() {
	// 初始化用户映射
	m.users = make(map[string]*User)

	// 从Redis加载用户数据
	if m.redisClient != nil {
		userIDs, err := m.redisClient.GetAllUsers()
		if err == nil && len(userIDs) > 0 {
			for _, userID := range userIDs {
				userData, err := m.redisClient.GetUser(userID)
				if err == nil && len(userData) > 0 {
					user := &User{
						ID:       userData["id"],
						Username: userData["username"],
						Password: userData["password"],
					}
					m.users[user.ID] = user
				}
			}
		}
	}
}

// saveUsers 保存用户数据，只保存到Redis
func (m *UserManager) saveUsers() error {
	// 保存到Redis（如果Redis客户端可用）
	if m.redisClient != nil {
		for _, user := range m.users {
			if err := m.redisClient.SaveUser(user.ID, user.Username, user.Password); err != nil {
				return err
			}
		}
	}

	return nil
}
