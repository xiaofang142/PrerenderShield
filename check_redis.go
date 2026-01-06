package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	redisv8 "github.com/go-redis/redis/v8"
)

func main() {
	// 解析命令行参数
	siteID := flag.String("site-id", "9dbfaa2b-9015-4012-a00a-8e7f47ab01dd", "Site ID to check in Redis")
	redisURL := flag.String("redis", "localhost:6379", "Redis URL")
	flag.Parse()

	// 创建直接的go-redis客户端
	opts := &redisv8.Options{
		Addr:     *redisURL,
		Password: "",
		DB:       0,
	}

	directClient := redisv8.NewClient(opts)
	defer directClient.Close()

	ctx := context.Background()

	// 检查站点统计信息
	fmt.Printf("\n检查站点统计信息 (prerender:%s:stats):\n", *siteID)
	siteStats, err := directClient.HGetAll(ctx, fmt.Sprintf("prerender:%s:stats", *siteID)).Result()
	if err != nil {
		log.Printf("获取站点统计信息失败: %v", err)
	} else if len(siteStats) == 0 {
		fmt.Println("  站点统计信息不存在")
	} else {
		for k, v := range siteStats {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	// 检查预渲染配置
	fmt.Printf("\n检查预渲染配置 (prerender:%s_prerender:stats):\n", *siteID)
	prerenderStats, err := directClient.HGetAll(ctx, fmt.Sprintf("prerender:%s_prerender:stats", *siteID)).Result()
	if err != nil {
		log.Printf("获取预渲染配置失败: %v", err)
	} else if len(prerenderStats) == 0 {
		fmt.Println("  预渲染配置不存在")
		// 检查预渲染相关的其他键
		fmt.Println("  检查预渲染相关的其他键:")
		preKeys, err := directClient.Keys(ctx, fmt.Sprintf("prerender:%s*prerender*", *siteID)).Result()
		if err != nil {
			log.Printf("获取预渲染相关键失败: %v", err)
		} else {
			for _, key := range preKeys {
				fmt.Printf("    %s\n", key)
			}
		}
	} else {
		for k, v := range prerenderStats {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	// 检查推送配置
	fmt.Printf("\n检查推送配置 (prerender:%s_push:stats):\n", *siteID)
	pushStats, err := directClient.HGetAll(ctx, fmt.Sprintf("prerender:%s_push:stats", *siteID)).Result()
	if err != nil {
		log.Printf("获取推送配置失败: %v", err)
	} else if len(pushStats) == 0 {
		fmt.Println("  推送配置不存在")
	} else {
		for k, v := range pushStats {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	// 检查URL集合
	fmt.Printf("\n检查URL集合 (prerender:%s:urls):\n", *siteID)
	urls, err := directClient.SMembers(ctx, fmt.Sprintf("prerender:%s:urls", *siteID)).Result()
	if err != nil {
		log.Printf("获取URL集合失败: %v", err)
	} else {
		fmt.Printf("  URL数量: %d\n", len(urls))
		if len(urls) > 0 {
			fmt.Println("  前5个URL:")
			for i, url := range urls[:min(5, len(urls))] {
				fmt.Printf("    %d: %s\n", i+1, url)
			}
		}
	}

	// 获取所有与该站点相关的键
	fmt.Printf("\n所有与站点 %s 相关的键:\n", *siteID)
	siteKeys, err := directClient.Keys(ctx, fmt.Sprintf("prerender:%s*", *siteID)).Result()
	if err != nil {
		log.Printf("获取站点相关键失败: %v", err)
	} else {
		for _, key := range siteKeys {
			fmt.Printf("  %s\n", key)
		}
	}
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}