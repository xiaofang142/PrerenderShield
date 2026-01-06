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

	// 测试保存站点配置
	testSiteID := "test_site_123"

	// 保存站点基本信息
	basicStats := map[string]interface{}{
		"name":   "Test Site",
		"domain": "localhost",
		"port":   8080,
		"mode":   "production",
	}
	if err := redisClient.SetSiteStats(testSiteID, basicStats); err != nil {
		log.Printf("Failed to save basic stats: %v", err)
	} else {
		fmt.Println("Basic stats saved successfully!")
	}

	// 保存预渲染配置（扁平化结构）
	prerenderConfig := map[string]interface{}{
		"enabled":             true,
		"pool_size":           10,
		"min_pool_size":       5,
		"max_pool_size":       20,
		"timeout":             30,
		"cache_ttl":           3600,
		"idle_timeout":        600,
		"preheat_enabled":     true,
		"preheat_sitemap_url": "http://localhost/sitemap.xml",
		"preheat_schedule":    "0 0 * * *",
		"preheat_concurrency": 5,
		"preheat_max_depth":   3,
	}
	if err := redisClient.SetSiteStats(testSiteID+"_prerender", prerenderConfig); err != nil {
		log.Printf("Failed to save prerender config: %v", err)
	} else {
		fmt.Println("Prerender config saved successfully!")
	}

	// 保存推送配置
	pushConfig := map[string]interface{}{
		"enabled":           true,
		"baidu_api":         "https://data.zz.baidu.com/urls",
		"baidu_token":       "test_token",
		"bing_api":          "https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl",
		"bing_token":        "test_bing_token",
		"baidu_daily_limit": 1000,
		"bing_daily_limit":  2000,
		"push_domain":       "localhost",
	}
	if err := redisClient.SetSiteStats(testSiteID+"_push", pushConfig); err != nil {
		log.Printf("Failed to save push config: %v", err)
	} else {
		fmt.Println("Push config saved successfully!")
	}

	fmt.Println("Test completed!")
}