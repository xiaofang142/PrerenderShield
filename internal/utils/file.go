package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractArchive 解压归档文件（支持ZIP格式）
func ExtractArchive(archivePath, extractPath string) error {
	// 检查文件类型
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return ExtractZIP(archivePath, extractPath)
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".rar") {
		return extractRAR(archivePath, extractPath)
	}
	return fmt.Errorf("unsupported archive format")
}

// extractRAR 解压RAR文件
// Go 标准库不支持 RAR 格式，需要安装 unrar 命令行工具
func extractRAR(archivePath, extractPath string) error {
	return fmt.Errorf("RAR format is not supported in Go standard library. Please convert to ZIP/TAR.GZ format, or install 'unrar' and use external extraction")
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// DeleteDir 删除目录及其内容
func DeleteDir(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dirPath)
}

// ListDir 列出目录内容
func ListDir(dirPath string) ([]os.DirEntry, error) {
	return os.ReadDir(dirPath)
}

// ExtractZIP 解压ZIP文件（带 zip-slip 路径穿越防护）
func ExtractZIP(filePath, destDir string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		destFilePath := filepath.Join(destDir, file.Name)

		// 防止 zip-slip：目标路径必须仍在解压目录内
		if !strings.HasPrefix(destFilePath, filepath.Clean(destDir)+string(os.PathSeparator)) && destFilePath != filepath.Clean(destDir) {
			return fmt.Errorf("illegal file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		zipFile, err := file.Open()
		if err != nil {
			destFile.Close()
			return err
		}

		if _, err := io.Copy(destFile, zipFile); err != nil {
			zipFile.Close()
			destFile.Close()
			return err
		}

		zipFile.Close()
		destFile.Close()
	}

	return nil
}
