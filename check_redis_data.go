package main

import (
	"fmt"
	"log"

	"prerender-shield/internal/redis"
)

func main() {
	// 创建Redis客户端
	redisClient, err := redis.NewClient("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	fmt.Println("Connected to Redis successfully!")

	// 测试站点ID
	testSiteID := "test_site_123"

	// 获取站点基本信息
	basicStats, err := redisClient.GetSiteStats(testSiteID)
	if err != nil {
		log.Printf("Failed to get basic stats: %v", err)
	} else {
		fmt.Println("=== Site Basic Stats ===")
		for k, v := range basicStats {
			fmt.Printf("%s: %s\n", k, v)
		}
	}

	// 获取预渲染配置
	prerenderConfig, err := redisClient.GetSiteStats(testSiteID+"_prerender")
	if err != nil {
		log.Printf("Failed to get prerender config: %v", err)
	} else {
		fmt.Println("\n=== Prerender Config ===")
		for k, v := range prerenderConfig {
			fmt.Printf("%s: %s\n", k, v)
		}
	}

	// 获取推送配置
	pushConfig, err := redisClient.GetSiteStats(testSiteID+"_push")
	if err != nil {
		log.Printf("Failed to get push config: %v", err)
	} else {
		fmt.Println("\n=== Push Config ===")
		for k, v := range pushConfig {
			fmt.Printf("%s: %s\n", k, v)
		}
	}

	fmt.Println("\nData check completed!")
}