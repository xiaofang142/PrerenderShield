package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

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
	dataPath    string
	redisClient *redis.Client
}

// NewUserManager 创建用户管理器
func NewUserManager(dataPath string, redisClient *redis.Client) *UserManager {
	manager := &UserManager{
		users:       make(map[string]*User),
		dataPath:    filepath.Join(dataPath, "users.json"),
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

	// 保存用户到文件和Redis
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
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// IsFirstRun 检查是否是首次运行（没有用户）
func (m *UserManager) IsFirstRun() bool {
	return len(m.users) == 0
}

// loadUsers 加载用户数据
func (m *UserManager) loadUsers() {
	// 优先从Redis加载用户数据（如果Redis客户端可用）
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
			return
		}
	}

	// Redis加载失败或没有用户，从文件加载
	// 检查文件是否存在
	if _, err := os.Stat(m.dataPath); os.IsNotExist(err) {
		// 文件不存在，创建空文件
		file, err := os.Create(m.dataPath)
		if err != nil {
			return
		}
		file.Close()
		return
	}

	// 读取文件
	file, err := os.Open(m.dataPath)
	if err != nil {
		return
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return
	}
	fileSize := fileInfo.Size()
	if fileSize == 0 {
		return
	}

	// 读取文件内容
	content := make([]byte, fileSize)
	_, err = file.Read(content)
	if err != nil {
		return
	}

	// 解析JSON
	var users []*User
	if err := json.Unmarshal(content, &users); err != nil {
		return
	}

	// 将用户添加到map
	for _, user := range users {
		m.users[user.ID] = user
	}
}

// saveUsers 保存用户数据
func (m *UserManager) saveUsers() error {
	// 将map转换为切片
	var users []*User
	for _, user := range m.users {
		users = append(users, user)
	}

	// 序列化JSON
	content, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(m.dataPath, content, 0644); err != nil {
		return err
	}

	// 保存到Redis（如果Redis客户端可用）
	if m.redisClient != nil {
		for _, user := range users {
			if err := m.redisClient.SaveUser(user.ID, user.Username, user.Password); err != nil {
				return err
			}
		}
	}

	return nil
}
