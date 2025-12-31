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
		return extractZIP(archivePath, extractPath)
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".rar") {
		return extractRAR(archivePath, extractPath)
	}
	return fmt.Errorf("unsupported archive format")
}

// extractZIP 解压ZIP文件
func extractZIP(archivePath, extractPath string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建目标文件路径
		targetPath := filepath.Join(extractPath, file.Name)

		// 检查文件类型
		if file.FileInfo().IsDir() {
			// 创建目录
			os.MkdirAll(targetPath, os.ModePerm)
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return err
		}

		// 打开ZIP中的文件
		inFile, err := file.Open()
		if err != nil {
			return err
		}

		// 创建目标文件
		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			inFile.Close()
			return err
		}

		// 复制文件内容
		if _, err := io.Copy(outFile, inFile); err != nil {
			inFile.Close()
			outFile.Close()
			return err
		}

		// 关闭文件
		inFile.Close()
		outFile.Close()
	}

	return nil
}

// extractRAR 解压RAR文件（预留实现，需要外部库支持）
func extractRAR(archivePath, extractPath string) error {
	return fmt.Errorf("RAR extraction not implemented yet")
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
