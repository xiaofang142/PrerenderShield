package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	fmt.Println("=== Full Flow Test ===")
	fmt.Printf("Testing site: 9dbfaa2b-9015-4012-a00a-8e7f47ab01dd\n")
	
	// 1. 清理之前的解压结果
	cleanupPreviousResults()
	
	// 2. 登录获取令牌
	token, err := login("admin", "123456")
	if err != nil {
		fmt.Printf("❌ Login failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Login successful, token: %s...\n", token[:20])
	
	// 3. 测试解压请求
	testExtractRequest(token)
	
	// 4. 验证解压结果
	verifyExtractionResults()
	
	// 5. 清理测试文件
	cleanupTestFiles()
	
	fmt.Println("\n🎉 All tests completed!")
}

// 清理之前的解压结果
func cleanupPreviousResults() {
	fmt.Println("\n1. Cleaning up previous extraction results...")
	
	// 站点ID
	siteID := "9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"
	
	// 静态目录
	siteStaticDir := filepath.Join("./static", siteID)
	
	// 移除之前解压的文件
	filesToRemove := []string{
		filepath.Join(siteStaticDir, "assets"),
		filepath.Join(siteStaticDir, "index.html"),
		filepath.Join(siteStaticDir, "vite.svg"),
	}
	
	for _, file := range filesToRemove {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			err := os.RemoveAll(file)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to remove %s: %v\n", file, err)
			} else {
				fmt.Printf("   ✅ Removed %s\n", file)
			}
		}
	}
	
	fmt.Println("   ✅ Cleanup completed!")
}

// 登录获取令牌
func login(username, password string) (string, error) {
	fmt.Println("\n2. Logging in...")
	
	// 登录请求
	loginURL := "http://localhost:9598/api/v1/auth/login"
	
	// 登录凭据
	credentials := map[string]string{
		"username": username,
		"password": password,
	}
	
	// 转换为JSON
	jsonData, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}
	
	// 发送POST请求
	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	
	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	// 解析响应
	var loginResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse login response: %v, body: %s", err, string(body))
	}
	
	if loginResp.Code != 200 {
		return "", fmt.Errorf("login failed: %s", loginResp.Message)
	}
	
	return loginResp.Data.Token, nil
}

// 测试解压请求
func testExtractRequest(token string) {
	fmt.Println("\n3. Testing extract request...")
	
	// API URL
	extractURL := "http://localhost:9598/api/v1/sites/9dbfaa2b-9015-4012-a00a-8e7f47ab01dd/static/extract"
	
	// 请求体
	form := url.Values{}
	form.Add("filename", "归档.zip")
	form.Add("path", "/")
	
	// 发送请求
	req, err := http.NewRequest("POST", extractURL, strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Printf("   ❌ Failed to create request: %v\n", err)
		return
	}
	
	// 设置请求头
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	
	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ Failed to send request: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("   ❌ Failed to read response: %v\n", err)
		return
	}
	
	// 打印响应
	fmt.Printf("   ✅ Response status: %d\n", resp.StatusCode)
	fmt.Printf("   ✅ Response body: %s\n", string(body))
	
	// 解析响应
	var extractResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	
	if err := json.Unmarshal(body, &extractResp); err != nil {
		fmt.Printf("   ⚠️  Failed to parse extract response: %v\n", err)
		return
	}
	
	if extractResp.Code == 200 {
		fmt.Printf("   ✅ Extract request successful: %s\n", extractResp.Message)
	} else {
		fmt.Printf("   ❌ Extract request failed: %s\n", extractResp.Message)
	}
}

// 验证解压结果
func verifyExtractionResults() {
	fmt.Println("\n4. Verifying extraction results...")
	
	// 站点ID
	siteID := "9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"
	
	// 静态目录
	siteStaticDir := filepath.Join("./static", siteID)
	
	// 检查解压后的文件
	filesToCheck := []string{
		filepath.Join(siteStaticDir, "assets"),
		filepath.Join(siteStaticDir, "index.html"),
		filepath.Join(siteStaticDir, "vite.svg"),
	}
	
	fmt.Printf("   Checking files in: %s\n", siteStaticDir)
	
	// 列出目录内容，看看实际有什么
	listDirectory(siteStaticDir, "   ")
	
	allFound := true
	for _, file := range filesToCheck {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			fileInfo, _ := os.Stat(file)
			fileType := "file"
			if fileInfo.IsDir() {
				fileType = "dir"
			}
			fmt.Printf("   ✅ Found %s: %s\n", fileType, file)
		} else {
			fmt.Printf("   ❌ Missing: %s\n", file)
			allFound = false
		}
	}
	
	if allFound {
		fmt.Println("   🎉 All files extracted successfully!")
	} else {
		fmt.Println("   ⚠️  Some files are missing!")
	}
	
	// 检查日志
	checkLogs()
}

// 列出目录内容
func listDirectory(dirPath, indent string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("%s❌ Failed to read directory: %v\n", indent, err)
		return
	}
	
	fmt.Printf("%sDirectory contents (%d items):\n", indent, len(files))
	for _, file := range files {
		fileInfo, _ := file.Info()
		fileType := "📄"
		if file.IsDir() {
			fileType = "📁"
		}
		fmt.Printf("%s%s %s (%s)\n", indent, fileType, file.Name(), formatSize(fileInfo.Size()))
	}
}

// 格式化文件大小
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 检查日志
func checkLogs() {
	fmt.Println("\n5. Checking recent logs...")
	
	// 读取日志文件
	logFile := "./data/prerender-shield.log"
	content, err := ioutil.ReadFile(logFile)
	if err != nil {
		fmt.Printf("   ❌ Failed to read log file: %v\n", err)
		return
	}
	
	// 搜索与解压相关的日志
	lines := bytes.Split(content, []byte("\n"))
	fmt.Printf("   Found %d log lines\n", len(lines))
	
	fmt.Println("   Recent API requests:")
	recentCount := 0
	for i := len(lines) - 1; i >= 0 && recentCount < 10; i-- {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		fmt.Printf("   %s\n", string(line))
		recentCount++
	}
}

// 清理测试文件
func cleanupTestFiles() {
	fmt.Println("\n6. Cleaning up test files...")
	os.Remove("test_full_flow.go")
	fmt.Println("   ✅ Test file removed")
}
