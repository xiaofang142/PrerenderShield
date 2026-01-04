package main

import (
	"fmt"
	"prerender-shield/internal/api/routes"
)

func main() {
	// 测试完整的解压流程
	siteID := "89da7d00-7500-4e3c-8b3d-f524946a2241"
	fileName := "归档.zip"
	siteStaticDir := "/Users/xiaofang/Documents/www/prerender/prerender-shield/static"

	// 构建完整的文件路径
	filePath := fmt.Sprintf("%s/%s/%s", siteStaticDir, siteID, fileName)
	destDir := fmt.Sprintf("%s/%s", siteStaticDir, siteID)

	fmt.Printf("Testing full extract flow:\n")
	fmt.Printf("  siteID: %s\n", siteID)
	fmt.Printf("  fileName: %s\n", fileName)
	fmt.Printf("  siteStaticDir: %s\n", siteStaticDir)
	fmt.Printf("  filePath: %s\n", filePath)
	fmt.Printf("  destDir: %s\n", destDir)

	// 直接调用ExtractZIP函数
	err := routes.ExtractZIP(filePath, destDir)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  SUCCESS: File extracted successfully\n")
	}
}