package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})

	ctx := context.Background()

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// 获取所有爬虫日志keys
	keys, err := client.Keys(ctx, "crawler_logs:*").Result()
	if err != nil {
		log.Fatalf("Failed to get keys: %v", err)
	}

	fmt.Println("Crawler log keys:")
	for _, key := range keys {
		fmt.Printf("  %s\n", key)

		// 获取每个key的内容
		if len(keys) < 10 { // 只显示前10个key的内容，避免输出过多
			values, err := client.ZRange(ctx, key, 0, 5).Result() // 获取前5条日志
			if err != nil {
				log.Printf("Failed to get values for key %s: %v", key, err)
				continue
			}

			for i, value := range values {
				fmt.Printf("    Log %d: %s\n", i+1, value)
			}
		}
	}

	// 关闭连接
	client.Close()
}