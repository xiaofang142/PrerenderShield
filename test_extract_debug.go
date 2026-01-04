package main

import (
	"fmt"
	"os"
	"path/filepath"

	"prerender-shield/internal/api/routes"
)

func main() {
	// 测试ExtractZIP函数
	filePath := "./static/89da7d00-7500-4e3c-8b3d-f524946a2241/归档.zip"
	destDir := "./static/89da7d00-7500-4e3c-8b3d-f524946a2241/"
	
	fmt.Printf("Testing ExtractZIP with:\n")
	fmt.Printf("  filePath: %s\n", filePath)
	fmt.Printf("  destDir: %s\n", destDir)
	
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("Error: File not found at path: %s\n", filePath)
		return
	}
	
	// 调用ExtractZIP函数
	err := routes.ExtractZIP(filePath, destDir)
	if err != nil {
		fmt.Printf("Error extracting ZIP file: %v\n", err)
		return
	}
	
	fmt.Printf("ZIP file extracted successfully!\n")
	
	// 列出解压后的文件
	fmt.Printf("\nExtracted files:\n")
	listFiles(destDir)
}

func listFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory %s: %v\n", dir, err)
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  [DIR]  %s\n", entry.Name())
			// 递归列出子目录中的文件
			listFiles(filepath.Join(dir, entry.Name()))
		} else {
			fmt.Printf("  [FILE] %s\n", entry.Name())
		}
	}
}
