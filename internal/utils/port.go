package utils

import (
	"fmt"
	"net"
)

// reservedPorts 常用服务端口，站点服务器禁止占用
var reservedPorts = map[int]bool{
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

	3306:  true, // MySQL
	5432:  true, // PostgreSQL
	6379:  true, // Redis
	8080:  true, // Tomcat
	9000:  true, // PHP-FPM
	9090:  true, // Prometheus
	15672: true, // RabbitMQ
	27017: true, // MongoDB
}

// IsPortAvailable 检查端口是否可用（排除常用保留端口且未被监听）
func IsPortAvailable(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	if reservedPorts[port] {
		return false
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()

	return true
}
