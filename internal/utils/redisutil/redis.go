package redisutil

import (
	"strconv"
	"strings"

	"prerender-shield/internal/redis"
)

// ParseRedisURL 从 Redis URL 字符串解析连接参数
// 支持格式：redis://password@host:port/db
func ParseRedisURL(redisURL string) (*redis.Client, error) {
	redisURL = strings.TrimPrefix(redisURL, "redis://")

	// 默认参数
	host := "localhost:6379"
	password := ""
	db := 0

	parts := strings.Split(redisURL, "/")
	if len(parts) >= 1 {
		addrPart := parts[0]
		if strings.Contains(addrPart, "@") {
			pwHost := strings.SplitN(addrPart, "@", 2)
			password = pwHost[0]
			host = pwHost[1]
		} else {
			host = addrPart
		}
	}
	if len(parts) >= 2 && parts[1] != "" {
		var err error
		db, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
	}

	return redis.NewClient(host, password, db)
}
