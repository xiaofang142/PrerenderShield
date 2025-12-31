package detectors

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall/types"
)

// FileIntegrityDetector 文件完整性检测器
type FileIntegrityDetector struct {
	mutex         sync.RWMutex
	fileHashes    map[string]string   // 文件名到哈希值的映射
	checkInterval time.Duration       // 检查间隔
	staticDir     string              // 静态文件目录
	enabled       bool                // 是否启用
	hashAlgorithm string              // 哈希算法
	threatsChan   chan []types.Threat // 威胁检测结果通道
}

// NewFileIntegrityDetector 创建新的文件完整性检测器
func NewFileIntegrityDetector(staticDir string, fileIntegrityConfig *config.FileIntegrityConfig) *FileIntegrityDetector {
	// 如果没有配置，使用默认值
	if fileIntegrityConfig == nil {
		fileIntegrityConfig = &config.FileIntegrityConfig{
			Enabled:       false,
			CheckInterval: 300,
			HashAlgorithm: "sha256",
		}
	}

	checkInterval := time.Duration(fileIntegrityConfig.CheckInterval) * time.Second
	if checkInterval <= 0 {
		checkInterval = 300 * time.Second
	}

	d := &FileIntegrityDetector{
		fileHashes:    make(map[string]string),
		checkInterval: checkInterval,
		staticDir:     staticDir,
		enabled:       fileIntegrityConfig.Enabled,
		hashAlgorithm: fileIntegrityConfig.HashAlgorithm,
		threatsChan:   make(chan []types.Threat, 10),
	}

	// 只有启用时才初始化和启动检查
	if d.enabled {
		// 初始化文件哈希值
		d.initFileHashes()

		// 启动定期检查的协程
		go d.checkLoop()
	}

	return d
}

// Detect 检测文件是否被篡改
// 注意：这个检测器主要通过定期检查，而不是每次请求都检查
func (d *FileIntegrityDetector) Detect(req *http.Request) ([]types.Threat, error) {
	// 从通道中获取所有威胁
	threats := []types.Threat{}

	// 非阻塞读取通道中的所有威胁
	for {
		select {
		case t := <-d.threatsChan:
			threats = append(threats, t...)
		default:
			// 通道为空，退出循环
			return threats, nil
		}
	}
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
	staticDirs, err := filepath.Glob(filepath.Join(d.staticDir, "*"))
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
				hash, err := d.calculateFileHash(path)
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

	threats := []types.Threat{}

	// 检查每个文件
	for path, oldHash := range oldHashes {
		// 计算当前文件哈希值
		currentHash, err := d.calculateFileHash(path)
		if err != nil {
			// 文件可能被删除
			d.mutex.Lock()
			delete(d.fileHashes, path)
			d.mutex.Unlock()
			threats = append(threats, types.Threat{
				Type:     "file_integrity",
				SubType:  "file_deleted",
				Severity: "high",
				Message:  "File has been deleted: " + path,
				Details: map[string]interface{}{
					"file_path": path,
					"detector":  d.Name(),
				},
			})
			continue
		}

		// 比较哈希值
		if currentHash != oldHash {
			// 文件被篡改
			threats = append(threats, types.Threat{
				Type:     "file_integrity",
				SubType:  "file_tampered",
				Severity: "critical",
				Message:  "File has been tampered with: " + path,
				Details: map[string]interface{}{
					"file_path":    path,
					"old_hash":     oldHash,
					"current_hash": currentHash,
					"detector":     d.Name(),
					"algorithm":    d.hashAlgorithm,
				},
			})

			// 更新哈希值
			d.mutex.Lock()
			d.fileHashes[path] = currentHash
			d.mutex.Unlock()
		}
	}

	// 检查是否有新文件
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 重新初始化哈希值，检查新文件
	newHashes := make(map[string]string)
	staticDirs, _ := filepath.Glob(filepath.Join(d.staticDir, "*"))
	for _, dir := range staticDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				hash, _ := d.calculateFileHash(path)
				newHashes[path] = hash

				// 检查是否是新文件
				if _, exists := oldHashes[path]; !exists && hash != "" {
					threats = append(threats, types.Threat{
						Type:     "file_integrity",
						SubType:  "new_file_added",
						Severity: "medium",
						Message:  "New file has been added: " + path,
						Details: map[string]interface{}{
							"file_path": path,
							"detector":  d.Name(),
						},
					})
				}
			}
			return nil
		})
	}

	// 更新哈希映射
	d.fileHashes = newHashes

	// 如果有威胁，发送到威胁通道
	if len(threats) > 0 {
		select {
		case d.threatsChan <- threats:
		default:
			// 通道已满，丢弃
		}
	}
}

// calculateFileHash 计算文件哈希值，支持多种算法
func (d *FileIntegrityDetector) calculateFileHash(path string) (string, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	// 根据配置的哈希算法计算哈希值
	var hash []byte
	switch d.hashAlgorithm {
	case "md5":
		h := md5.Sum(data)
		hash = h[:]
	case "sha1":
		h := sha1.Sum(data)
		hash = h[:]
	case "sha256":
		h := sha256.Sum256(data)
		hash = h[:]
	case "sha512":
		h := sha512.Sum512(data)
		hash = h[:]
	default:
		// 默认使用sha256
		h := sha256.Sum256(data)
		hash = h[:]
	}

	return hex.EncodeToString(hash[:]), nil
}
