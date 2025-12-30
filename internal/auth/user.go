package auth

import (
	"errors"
	"os"
	"path/filepath"

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
	users    map[string]*User
	dataPath string
}

// NewUserManager 创建用户管理器
func NewUserManager(dataPath string) *UserManager {
	manager := &UserManager{
		users:    make(map[string]*User),
		dataPath: filepath.Join(dataPath, "users.json"),
	}

	// 加载用户数据
	manager.loadUsers()

	return manager
}

// CreateUser 创建用户
func (m *UserManager) CreateUser(username, password string) (*User, error) {
	// 检查用户是否已存在
	for _, user := range m.users {
		if user.Username == username {
			return nil, ErrUserExists
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

	// 保存用户
	m.users[userID] = user
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

	// 解析JSON
	// 这里简化处理，实际项目中应该使用json.Unmarshal
	// 暂时返回空map
}

// saveUsers 保存用户数据
func (m *UserManager) saveUsers() error {
	// 这里简化处理，实际项目中应该使用json.Marshal
	// 暂时返回nil
	return nil
}
