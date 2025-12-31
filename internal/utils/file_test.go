package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureDir 测试EnsureDir函数
func TestEnsureDir(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test_dir")

	// 测试目录不存在的情况
	if err := EnsureDir(testDir); err != nil {
		t.Errorf("EnsureDir failed: %v", err)
	}

	// 测试目录已存在的情况
	if err := EnsureDir(testDir); err != nil {
		t.Errorf("EnsureDir failed when dir exists: %v", err)
	}

	// 验证目录确实存在
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created: %s", testDir)
	}
}

// TestDeleteDir 测试DeleteDir函数
func TestDeleteDir(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test_dir")

	// 先创建目录
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// 测试删除存在的目录
	if err := DeleteDir(testDir); err != nil {
		t.Errorf("DeleteDir failed: %v", err)
	}

	// 验证目录确实被删除
	if _, err := os.Stat(testDir); err == nil {
		t.Errorf("Directory was not deleted: %s", testDir)
	}

	// 测试删除不存在的目录
	if err := DeleteDir(testDir); err != nil {
		t.Errorf("DeleteDir failed when dir does not exist: %v", err)
	}
}

// TestListDir 测试ListDir函数
func TestListDir(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 创建测试子目录
	testSubDir := filepath.Join(tempDir, "sub_dir")
	if err := os.MkdirAll(testSubDir, 0755); err != nil {
		t.Fatalf("Failed to create test subdirectory: %v", err)
	}

	// 测试列出目录内容
	entries, err := ListDir(tempDir)
	if err != nil {
		t.Errorf("ListDir failed: %v", err)
		return
	}

	// 验证目录内容
	foundFile := false
	foundDir := false

	for _, entry := range entries {
		if entry.Name() == "test.txt" && !entry.IsDir() {
			foundFile = true
		}
		if entry.Name() == "sub_dir" && entry.IsDir() {
			foundDir = true
		}
	}

	if !foundFile {
		t.Error("test.txt was not found in directory listing")
	}
	if !foundDir {
		t.Error("sub_dir was not found in directory listing")
	}
}

// TestExtractArchive 测试ExtractArchive函数
func TestExtractArchive(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	zipFile := filepath.Join(tempDir, "test.zip")
	extractDir := filepath.Join(tempDir, "extract")

	// 创建一个测试ZIP文件
	if err := createTestZip(zipFile); err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}

	// 测试解压ZIP文件
	if err := ExtractArchive(zipFile, extractDir); err != nil {
		t.Errorf("ExtractArchive failed: %v", err)
		return
	}

	// 验证解压后的文件
	testFile := filepath.Join(extractDir, "test.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("Extracted file not found: %s", testFile)
	}

	// 验证文件内容
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
		return
	}

	if string(content) != "test content" {
		t.Errorf("Extracted file content mismatch. Expected 'test content', got '%s'", string(content))
	}
}

// createTestZip 创建一个测试用的ZIP文件
func createTestZip(zipPath string) error {
	// 创建ZIP文件
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// 创建ZIP写入器
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 创建一个测试文件
	testFile, err := zipWriter.Create("test.txt")
	if err != nil {
		return err
	}

	// 写入文件内容
	_, err = testFile.Write([]byte("test content"))
	if err != nil {
		return err
	}

	return nil
}
