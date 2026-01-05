package routes

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// 添加安全头中间件
func addSecurityHeaders(ginRouter *gin.Engine) {
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击，允许跨域请求
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:5173 http://localhost:9598")

		// X-Frame-Options 头，防止Clickjacking攻击
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection 头，启用浏览器的XSS过滤
		c.Header("X-XSS-Protection", "1; mode=block")

		// X-Content-Type-Options 头，防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer-Policy 头，控制Referrer信息的发送
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS) 头，强制使用HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Permissions-Policy 头，控制浏览器API的访问
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), usb=(), accelerometer=(), gyroscope=()")

		c.Next()
	})
}

// 添加CORS中间件
func addCorsMiddleware(ginRouter *gin.Engine) {
	ginRouter.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

// 检查端口是否可用
func isPortAvailable(port int) bool {
	// 常用互联网端口列表，这些端口将被排除
	reservedPorts := map[int]bool{
		// 常用服务端口
		21:  true, // FTP
		22:  true, // SSH
		23:  true, // Telnet
		25:  true, // SMTP
		53:  true, // DNS
		80:  true, // HTTP
		110: true, // POP3
		143: true, // IMAP
		443: true, // HTTPS
		465: true, // SMTPS
		587: true, // SMTP (STARTTLS)
		993: true, // IMAPS
		995: true, // POP3S

		// 常用应用端口
		3306:  true, // MySQL
		5432:  true, // PostgreSQL
		6379:  true, // Redis
		8080:  true, // Tomcat
		9000:  true, // PHP-FPM
		9090:  true, // Prometheus
		15672: true, // RabbitMQ
		27017: true, // MongoDB
	}

	// 检查是否是保留端口
	if reservedPorts[port] {
		return false
	}

	// 尝试监听端口
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}

// ExtractZIP 解压ZIP文件，导出供测试使用
func ExtractZIP(filePath, destDir string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建目标文件路径
		destFilePath := filepath.Join(destDir, file.Name)

		// 检查文件是否是目录
		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		// 创建目标文件
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		// 不使用defer，而是立即关闭文件

		// 获取ZIP文件中的文件
		zipFile, err := file.Open()
		if err != nil {
			destFile.Close() // 确保文件关闭
			return err
		}

		// 复制文件内容
		if _, err := io.Copy(destFile, zipFile); err != nil {
			zipFile.Close()
			destFile.Close() // 确保文件关闭
			return err
		}

		// 立即关闭文件，避免资源泄漏和文件锁定问题
		zipFile.Close()
		destFile.Close()
	}

	return nil
}
