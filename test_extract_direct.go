package main

import (
	"fmt"
	"prerender-shield/internal/api/routes"
)

func main() {
	// 测试直接调用ExtractZIP函数
	filePath := "/Users/xiaofang/Documents/www/prerender/prerender-shield/static/89da7d00-7500-4e3c-8b3d-f524946a2241/归档.zip"
	destDir := "/Users/xiaofang/Documents/www/prerender/prerender-shield/static/89da7d00-7500-4e3c-8b3d-f524946a2241"

	fmt.Printf("Testing direct ExtractZIP call:\n")
	fmt.Printf("  filePath: %s\n", filePath)
	fmt.Printf("  destDir: %s\n", destDir)

	err := routes.ExtractZIP(filePath, destDir)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  SUCCESS: File extracted successfully\n")
	}
}