package detectors

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prerendershield/internal/firewall/types"
)

// FileIntegrityDetector 文件完整性检测器
type FileIntegrityDetector struct {
	mutex         sync.RWMutex
	fileHashes    map[string]string // 文件名到哈希值的映射
	checkInterval time.Duration     // 检查间隔
}

// NewFileIntegrityDetector 创建新的文件完整性检测器
func NewFileIntegrityDetector() *FileIntegrityDetector {
	d := &FileIntegrityDetector{
		fileHashes:    make(map[string]string),
		checkInterval: 300 * time.Second, // 默认5分钟检查一次
	}

	// 初始化文件哈希值
	d.initFileHashes()

	// 启动定期检查的协程
	go d.checkLoop()

	return d
}

// Detect 检测文件是否被篡改
// 注意：这个检测器主要通过定期检查，而不是每次请求都检查
func (d *FileIntegrityDetector) Detect(req *http.Request) ([]types.Threat, error) {
	// 暂时不通过请求检测，而是通过定期检查
	return []types.Threat{}, nil
}

// Name 返回检测器名称
func (d *FileIntegrityDetector) Name() string {
	return "file_integrity"
}

// initFileHashes 初始化文件哈希值
func (d *FileIntegrityDetector) initFileHashes() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 获取所有静态文件目录
	staticDirs, err := filepath.Glob("./static/*")
	if err != nil {
		return
	}

	for _, dir := range staticDirs {
		// 遍历目录下的所有文件
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 只处理文件，不处理目录
			if !info.IsDir() {
				// 计算文件哈希值
				hash, err := calculateFileHash(path)
				if err == nil {
					d.fileHashes[path] = hash
				}
			}

			return nil
		})
	}
}

// checkLoop 定期检查文件完整性
func (d *FileIntegrityDetector) checkLoop() {
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		d.checkFileIntegrity()
	}
}

// checkFileIntegrity 检查文件完整性
func (d *FileIntegrityDetector) checkFileIntegrity() {
	d.mutex.RLock()
	oldHashes := make(map[string]string)
	for k, v := range d.fileHashes {
		oldHashes[k] = v
	}
	d.mutex.RUnlock()

	// 检查每个文件
	for path, oldHash := range oldHashes {
		// 计算当前文件哈希值
		currentHash, err := calculateFileHash(path)
		if err != nil {
			// 文件可能被删除
			d.mutex.Lock()
			delete(d.fileHashes, path)
			d.mutex.Unlock()
			continue
		}

		// 比较哈希值
		if currentHash != oldHash {
			// 文件被篡改
			// 这里应该记录日志或触发告警
			// 暂时只更新哈希值
			d.mutex.Lock()
			d.fileHashes[path] = currentHash
			d.mutex.Unlock()
		}
	}

	// 检查是否有新文件
	d.initFileHashes()
}

// calculateFileHash 计算文件哈希值
func calculateFileHash(path string) (string, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	// 计算SHA256哈希值
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
