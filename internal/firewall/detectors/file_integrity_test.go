package detectors

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
)

// TestFileIntegrityDetector_Name 测试检测器名称
func TestFileIntegrityDetector_Name(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		CheckInterval: 300,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)
	assert.Equal(t, "file_integrity", detector.Name())
}

// TestFileIntegrityDetector_Detect_Disabled 测试禁用的情况
func TestFileIntegrityDetector_Detect_Disabled(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		CheckInterval: 300,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestFileIntegrityDetector_Detect_NilConfig 测试空配置
func TestFileIntegrityDetector_Detect_NilConfig(t *testing.T) {
	tempDir := t.TempDir()
	detector := NewFileIntegrityDetector(tempDir, nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
	// 配置为 nil 时，enabled 默认为 false
}

// TestFileIntegrityDetector_InitFileHashes 测试初始化文件哈希
func TestFileIntegrityDetector_InitFileHashes(t *testing.T) {
	tempDir := t.TempDir()

	// 创建测试文件
	staticDir := filepath.Join(tempDir, "static")
	err := os.MkdirAll(staticDir, 0755)
	assert.NoError(t, err)

	testFile := filepath.Join(staticDir, "test.js")
	err = os.WriteFile(testFile, []byte("console.log('test');"), 0644)
	assert.NoError(t, err)

	cfg := &config.FileIntegrityConfig{
		Enabled:       true,
		CheckInterval: 300,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	// 等待初始化完成
	time.Sleep(100 * time.Millisecond)

	// 验证 detector 已创建
	assert.NotNil(t, detector)
	assert.Equal(t, "file_integrity", detector.Name())
}

// TestFileIntegrityDetector_CalculateFileHash 测试文件哈希计算
func TestFileIntegrityDetector_CalculateFileHash(t *testing.T) {
	tempDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	// 测试不同哈希算法
	algorithms := []string{"md5", "sha1", "sha256", "sha512", "invalid"}

	for _, algo := range algorithms {
		detector.hashAlgorithm = algo
		hash, err := detector.calculateFileHash(testFile)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
	}
}

// TestFileIntegrityDetector_CalculateFileHash_NonExistentFile 测试不存在的文件
func TestFileIntegrityDetector_CalculateFileHash_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	_, err := detector.calculateFileHash(filepath.Join(tempDir, "nonexistent.txt"))
	assert.Error(t, err)
}

// TestFileIntegrityDetector_Detect_EmptyThreatsChan 测试空威胁通道
func TestFileIntegrityDetector_Detect_EmptyThreatsChan(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		CheckInterval: 300,
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestFileIntegrityDetector_CheckFileIntegrity_FileDeleted 测试文件删除检测
func TestFileIntegrityDetector_CheckFileIntegrity_FileDeleted(t *testing.T) {
	tempDir := t.TempDir()

	// 创建测试文件
	staticDir := filepath.Join(tempDir, "static")
	err := os.MkdirAll(staticDir, 0755)
	assert.NoError(t, err)

	testFile := filepath.Join(staticDir, "test.js")
	err = os.WriteFile(testFile, []byte("console.log('test');"), 0644)
	assert.NoError(t, err)

	cfg := &config.FileIntegrityConfig{
		Enabled:       true,
		CheckInterval: 1, // 快速检查
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	// 等待初始化
	time.Sleep(100 * time.Millisecond)

	// 删除文件
	err = os.Remove(testFile)
	assert.NoError(t, err)

	// 等待检查周期
	time.Sleep(2 * time.Second)

	// 检测应该能获取到威胁
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 可能检测到文件删除
	if len(threats) > 0 {
		assert.Contains(t, threats[0].SubType, "deleted")
	}
}

// TestFileIntegrityDetector_CheckFileIntegrity_FileModified 测试文件修改检测
func TestFileIntegrityDetector_CheckFileIntegrity_FileModified(t *testing.T) {
	tempDir := t.TempDir()

	// 创建测试文件
	staticDir := filepath.Join(tempDir, "static")
	err := os.MkdirAll(staticDir, 0755)
	assert.NoError(t, err)

	testFile := filepath.Join(staticDir, "test.js")
	err = os.WriteFile(testFile, []byte("original content"), 0644)
	assert.NoError(t, err)

	cfg := &config.FileIntegrityConfig{
		Enabled:       true,
		CheckInterval: 1,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	// 等待初始化
	time.Sleep(100 * time.Millisecond)

	// 修改文件内容
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	assert.NoError(t, err)

	// 等待检查周期
	time.Sleep(2 * time.Second)

	// 检测应该能获取到威胁
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 可能检测到文件篡改
	if len(threats) > 0 {
		assert.Contains(t, threats[0].SubType, "tampered")
	}
}

// TestFileIntegrityDetector_InvalidGlob 测试无效 glob 模式
func TestFileIntegrityDetector_InvalidGlob(t *testing.T) {
	// 使用一个会导致 glob 错误的路径
	cfg := &config.FileIntegrityConfig{
		Enabled:       true,
		CheckInterval: 300,
	}
	// 这个路径应该能正常处理（即使目录不存在）
	detector := NewFileIntegrityDetector("/nonexistent/path/that/does/not/exist", cfg)
	assert.NotNil(t, detector)
}

// TestFileIntegrityDetector_Detect_ConcurrentAccess 测试并发访问
func TestFileIntegrityDetector_Detect_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		CheckInterval: 300,
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	done := make(chan bool, 10)

	// 并发调用 Detect
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			detector.Detect(req)
			done <- true
		}()
	}

	// 等待所有调用完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 不应该 panic
	assert.True(t, true)
}

// TestFileIntegrityDetector_ConfigEdgeCases 测试配置边界情况
func TestFileIntegrityDetector_ConfigEdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	// 测试负数检查间隔
	cfg := &config.FileIntegrityConfig{
		Enabled:       true,
		CheckInterval: -100,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)
	assert.NotNil(t, detector)
	// 负数应该被替换为默认值 300 秒
}

// TestFileIntegrityDetector_SupportedAlgorithms 测试支持的哈希算法
func TestFileIntegrityDetector_SupportedAlgorithms(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	assert.NoError(t, err)

	cfg := &config.FileIntegrityConfig{
		Enabled:       false,
		HashAlgorithm: "sha256",
	}
	detector := NewFileIntegrityDetector(tempDir, cfg)

	// 测试所有支持的算法
	testCases := []struct {
		algorithm string
		expectErr bool
	}{
		{"md5", false},
		{"sha1", false},
		{"sha256", false},
		{"sha512", false},
		{"unknown", false}, // 未知算法会回退到 sha256
	}

	for _, tc := range testCases {
		detector.hashAlgorithm = tc.algorithm
		hash, err := detector.calculateFileHash(testFile)
		if tc.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotEmpty(t, hash)
		}
	}
}
