package logging

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// TestRecordCrawlerLog 测试记录爬虫日志
func TestRecordCrawlerLog(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan CrawlerLog, 10),
	}

	log := CrawlerLog{
		Site:     "test.com",
		IP:       "192.168.1.1",
		Route:    "/api/test",
		UA:       "TestBot/1.0",
		Status:   200,
		Method:   "GET",
		HitCache: true,
		Washed:   false,
	}

	manager.RecordCrawlerLog(log)

	select {
	case receivedLog := <-manager.logChan:
		assert.Equal(t, "test.com", receivedLog.Site)
		assert.Equal(t, "192.168.1.1", receivedLog.IP)
	case <-time.After(1 * time.Second):
		t.Error("log was not sent to channel")
	}
}

// TestRecordCrawlerLog_ZeroTime 测试时间戳为零时自动设置
func TestRecordCrawlerLog_ZeroTime(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan CrawlerLog, 10),
	}

	log := CrawlerLog{
		Site: "test.com",
		IP:   "192.168.1.1",
		Time: time.Time{},
	}

	manager.RecordCrawlerLog(log)

	select {
	case receivedLog := <-manager.logChan:
		assert.False(t, receivedLog.Time.IsZero())
	case <-time.After(1 * time.Second):
		t.Error("log was not sent to channel")
	}
}

// TestRecordCrawlerLog_ChannelFull 测试通道满时直接保存
func TestRecordCrawlerLog_ChannelFull(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	logChan := make(chan CrawlerLog, 1)
	logChan <- CrawlerLog{Site: "filler"}

	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     logChan,
	}

	log := CrawlerLog{Site: "test.com", IP: "192.168.1.1"}
	manager.RecordCrawlerLog(log)

	select {
	case <-manager.logChan:
		// 正确
	default:
		t.Error("channel should still have the filler log")
	}
}

// TestSaveLog 测试保存日志到 Redis
func TestSaveLog(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan CrawlerLog, 10),
	}

	log := CrawlerLog{
		Site:     "test.com",
		IP:       "192.168.1.1",
		Time:     time.Now(),
		Route:    "/api/test",
		UA:       "TestBot/1.0",
		Status:   200,
		Method:   "GET",
		HitCache: true,
		Washed:   false,
	}

	manager.saveLog(log)
	time.Sleep(100 * time.Millisecond)

	dateStr := log.Time.Format("2006-01-02")
	siteKey := "crawler_logs:test.com:" + dateStr

	result, err := client.ZRange(ctx, siteKey, 0, -1).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	var savedLog CrawlerLog
	err = json.Unmarshal([]byte(result[0]), &savedLog)
	assert.NoError(t, err)
	assert.Equal(t, "test.com", savedLog.Site)
}

// TestGetUnwashedLogs 测试获取待清洗日志
func TestGetUnwashedLogs(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	unwashedKey := "crawler_logs:unwashed"
	client.Del(ctx, unwashedKey)

	log1 := CrawlerLog{Site: "test1.com", IP: "1.1.1.1", Washed: false}
	log2 := CrawlerLog{Site: "test2.com", IP: "2.2.2.2", Washed: false}
	data1, _ := json.Marshal(log1)
	data2, _ := json.Marshal(log2)

	client.RPush(ctx, unwashedKey, data1)
	client.RPush(ctx, unwashedKey, data2)

	logs, err := manager.GetUnwashedLogs(2)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)

	length, _ := client.LLen(ctx, unwashedKey).Result()
	assert.Equal(t, int64(0), length)
}

// TestGetUnwashedLogs_Empty 测试待清洗队列为空
func TestGetUnwashedLogs_Empty(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	client.Del(ctx, "crawler_logs:unwashed")

	logs, err := manager.GetUnwashedLogs(10)
	assert.NoError(t, err)
	assert.Empty(t, logs)
}

// TestProcessLogs 测试异步日志处理
func TestProcessLogs(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	logChan := make(chan CrawlerLog, 10)
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     logChan,
	}

	go manager.processLogs()

	log := CrawlerLog{
		Site: "test.com",
		IP:   "192.168.1.1",
		Time: time.Now(),
	}
	logChan <- log
	time.Sleep(200 * time.Millisecond)

	dateStr := log.Time.Format("2006-01-02")
	siteKey := "crawler_logs:test.com:" + dateStr

	result, err := client.ZRange(ctx, siteKey, 0, -1).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

// TestProcessLogs_ChannelClosed 测试通道关闭时的处理
func TestProcessLogs_ChannelClosed(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	logChan := make(chan CrawlerLog, 10)
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     logChan,
	}

	log := CrawlerLog{Site: "test.com", IP: "192.168.1.1", Time: time.Now()}
	logChan <- log
	close(logChan)

	done := make(chan bool)
	go func() {
		manager.processLogs()
		done <- true
	}()

	select {
	case <-done:
		// 正常退出
	case <-time.After(1 * time.Second):
		t.Error("processLogs did not exit after channel closed")
	}
}

// TestUpdateLog 测试更新日志
func TestUpdateLog(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	log := CrawlerLog{
		Site:     "test.com",
		IP:       "192.168.1.1",
		Time:     time.Now(),
		Washed:   false,
		Country:  "US",
	}
	manager.saveLog(log)
	time.Sleep(100 * time.Millisecond)

	oldLog := log
	newLog := log
	newLog.Washed = true
	newLog.Country = "CN"

	err := manager.UpdateLog(oldLog, newLog)
	assert.NoError(t, err)
}

// TestGetCrawlerLogs 测试获取爬虫日志
func TestGetCrawlerLogs(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	site := "test-crawler-logs.com"
	dateStr := time.Now().Format("2006-01-02")
	siteKey := "crawler_logs:" + site + ":" + dateStr

	client.Del(ctx, siteKey)

	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	log := CrawlerLog{
		Site:  site,
		IP:    "192.168.1.1",
		Time:  time.Now(),
		Route: "/api/test",
	}
	data, _ := json.Marshal(log)
	client.ZAdd(ctx, siteKey, &redis.Z{Score: float64(time.Now().UnixNano()), Member: data})
	time.Sleep(100 * time.Millisecond)

	startTime := time.Now().AddDate(0, 0, -1)
	endTime := time.Now().AddDate(0, 0, 1)
	logs, total, err := manager.GetCrawlerLogs(site, startTime, endTime, 1, 10)
	assert.NoError(t, err)
	assert.NotEmpty(t, logs)
	assert.GreaterOrEqual(t, total, int64(1))
}

// TestGetCrawlerStats 测试获取爬虫统计
func TestGetCrawlerStats(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	site := "test-stats.com"
	dateStr := time.Now().Format("2006-01-02")
	siteKey := "crawler_logs:" + site + ":" + dateStr

	client.Del(ctx, siteKey)

	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	for i := 0; i < 10; i++ {
		log := CrawlerLog{
			Site:     site,
			IP:       "192.168.1." + string(rune(i%10+'0')),
			Time:     time.Now(),
			Status:   200,
			Method:   "GET",
			HitCache: i%2 == 0,
		}
		data, _ := json.Marshal(log)
		client.ZAdd(ctx, siteKey, &redis.Z{Score: float64(time.Now().UnixNano()), Member: data})
	}
	time.Sleep(100 * time.Millisecond)

	startTime := time.Now().AddDate(0, 0, -1)
	endTime := time.Now().AddDate(0, 0, 1)

	stats, err := manager.GetCrawlerStats(site, startTime, endTime, "day")
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "totalRequests")
}

// TestGetClientIP 测试获取客户端真实 IP
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1"},
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.1, 172.16.0.1"},
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Real-IP": "192.168.1.2"},
			expected:   "192.168.1.2",
		},
		{
			name:       "No headers IPv4",
			remoteAddr: "192.168.1.3:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.3",
		},
		{
			name:       "IPv6 RemoteAddr",
			remoteAddr: "[::1]:8080",
			headers:    map[string]string{},
			expected:   "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := GetClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

// TestGetTrafficTrend 测试获取流量趋势
func TestGetTrafficTrend(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	site := "test-trend.com"
	dateStr := time.Now().Format("2006-01-02")
	siteKey := "crawler_logs:" + site + ":" + dateStr

	client.Del(ctx, siteKey)

	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	now := time.Now()
	for i := 0; i < 10; i++ {
		log := CrawlerLog{
			Site:   site,
			IP:     "192.168.1." + string(rune(i%10+'0')),
			Time:   now.Add(time.Duration(i) * time.Hour),
			Route:  "/page" + string(rune(i+'1')),
			Status: 200,
		}
		data, _ := json.Marshal(log)
		client.ZAdd(ctx, siteKey, &redis.Z{Score: float64(now.Add(time.Duration(i) * time.Hour).UnixNano()), Member: data})
	}
	time.Sleep(100 * time.Millisecond)

	startTime := now.AddDate(0, 0, -1)
	endTime := now.AddDate(0, 0, 1)

	trend := manager.GetTrafficTrend(startTime, endTime)
	assert.NotNil(t, trend)
	assert.Equal(t, 6, len(trend))
}

// TestCleanupOldLogs 测试清理旧日志
func TestCleanupOldLogs(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
	}

	oldDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	oldSiteKey := "crawler_logs:test-cleanup.com:" + oldDate
	oldAllKey := "crawler_logs:all:" + oldDate

	log := CrawlerLog{
		Site: "test-cleanup.com",
		IP:   "192.168.1.1",
		Time: time.Now().AddDate(0, 0, -30),
	}
	data, _ := json.Marshal(log)
	client.ZAdd(ctx, oldSiteKey, &redis.Z{Score: float64(time.Now().UnixNano()), Member: data})
	client.ZAdd(ctx, oldAllKey, &redis.Z{Score: float64(time.Now().UnixNano()), Member: data})

	exists, _ := client.Exists(ctx, oldAllKey).Result()
	assert.Equal(t, int64(1), exists)

	client.HSet(ctx, "config:system", "crawler_log_retention_days", "15")

	manager.cleanupOldLogs()

	exists, _ = client.Exists(ctx, oldAllKey).Result()
	assert.Equal(t, int64(0), exists)
}

// TestCrawlerLogManager_Integration 集成测试
func TestCrawlerLogManager_Integration(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	client.Del(ctx, "crawler_logs:integration-test.com:"+time.Now().Format("2006-01-02"))
	client.Del(ctx, "crawler_logs:all:"+time.Now().Format("2006-01-02"))
	client.Del(ctx, "crawler_logs:unwashed")

	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan CrawlerLog, 100),
	}

	// 启动异步处理 goroutine
	go manager.processLogs()

	log := CrawlerLog{
		Site:     "integration-test.com",
		IP:       "192.168.1.100",
		Time:     time.Now(),
		Route:    "/test",
		UA:       "TestBot/1.0",
		Status:   200,
		Method:   "GET",
		HitCache: true,
		Washed:   false,
	}

	manager.RecordCrawlerLog(log)
	time.Sleep(200 * time.Millisecond)

	startTime := time.Now().AddDate(0, 0, -1)
	endTime := time.Now().AddDate(0, 0, 1)

	_, total, err := manager.GetCrawlerLogs("integration-test.com", startTime, endTime, 1, 10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	stats, err := manager.GetCrawlerStats("integration-test.com", startTime, endTime, "day")
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	trend := manager.GetTrafficTrend(startTime, endTime)
	assert.NotNil(t, trend)
}
