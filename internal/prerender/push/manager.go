package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
)

// RedisClient 是 redis.Client 的接口，用于测试 mock
type RedisClient interface {
	GetURLs(siteID string) ([]string, error)
	SetPushTask(siteID string, task map[string]interface{}) error
	GetPushOffset(siteID string) (int64, error)
	SetPushOffset(siteID string, offset int64) error
	SetLastPushDate(siteID string, date string) error
	IncrDailyPushCountWithCount(siteID string, count int) error
	IncrPushStats(siteID string, stat string) error
	AddPushLogStruct(siteID string, log interface{}) error
	GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error)
	GetLast15DaysPushCount(siteID string) (map[string]int64, error)
	GetPushLogs(siteID string, limit, offset int) ([]interface{}, error)
	GetPushLogCount(siteID string) (int64, error)
}

// 确保 redis.Client 实现 RedisClient 接口
var _ RedisClient = (*redis.Client)(nil)

// PushManager 推送管理器
type PushManager struct {
	config      *config.Config
	redisClient RedisClient
	mutex       sync.Mutex
	// configProvider 每次返回最新配置（copy-on-write 换指针场景下，启动注入的
	// *Config 快照看不到会话内新建/修改的站点）。nil 时回退 config 字段。
	configProvider func() *config.Config
}

// SetConfigProvider 注入每请求新鲜配置来源（controller_setup 装配）
func (pm *PushManager) SetConfigProvider(fn func() *config.Config) {
	pm.configProvider = fn
}

// currentConfig 返回当前有效配置（provider 优先，回退启动快照）
func (pm *PushManager) currentConfig() *config.Config {
	if pm.configProvider != nil {
		if c := pm.configProvider(); c != nil {
			return c
		}
	}
	return pm.config
}

// NewPushManager 创建推送管理器实例
func NewPushManager(config *config.Config, redisClient RedisClient) *PushManager {
	return &PushManager{
		config:      config,
		redisClient: redisClient,
	}
}

// PushTask 推送任务
type PushTask struct {
	ID           string    `json:"id"`
	SiteID       string    `json:"siteId"`
	SiteName     string    `json:"siteName"`
	URLs         []string  `json:"urls"`
	Status       string    `json:"status"` // pending, running, completed, failed
	CreatedAt    time.Time `json:"createdAt"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	CompletedAt  time.Time `json:"completedAt,omitempty"`
	SuccessCount int       `json:"successCount"`
	FailedCount  int       `json:"failedCount"`
}

// PushLog 推送日志
type PushLog struct {
	ID           string    `json:"id"`
	SiteID       string    `json:"siteId"`
	SiteName     string    `json:"siteName"`
	URL          string    `json:"url"`
	Route        string    `json:"route"`
	SearchEngine string    `json:"searchEngine"`
	Status       string    `json:"status"` // success, failed
	Message      string    `json:"message"`
	PushTime     time.Time `json:"pushTime"`
}

// TriggerPush 触发推送
func (pm *PushManager) TriggerPush(siteID string) (string, error) {
	// 检查配置是否为 nil
	if pm.currentConfig() == nil {
		return "", fmt.Errorf("config is nil")
	}

	// 获取站点配置
	var siteConfig *config.SiteConfig
	for _, site := range pm.currentConfig().Sites {
		if site.ID == siteID {
			siteConfig = &site
			break
		}
	}

	if siteConfig == nil {
		return "", fmt.Errorf("site not found: %s", siteID)
	}

	// 检查推送是否启用
	if !siteConfig.Prerender.Push.Enabled {
		return "", fmt.Errorf("push is not enabled for site: %s", siteID)
	}

	// 创建推送任务
	taskID := fmt.Sprintf("push-%s-%d", siteID, time.Now().Unix())
	task := PushTask{
		ID:           taskID,
		SiteID:       siteID,
		SiteName:     siteConfig.Name,
		Status:       "pending",
		CreatedAt:    time.Now(),
		SuccessCount: 0,
		FailedCount:  0,
	}

	// 保存任务到Redis
	taskMap, err := json.Marshal(task)
	if err != nil {
		return "", err
	}
	var taskMapInterface map[string]interface{}
	if err := json.Unmarshal(taskMap, &taskMapInterface); err != nil {
		return "", err
	}
	if err := pm.redisClient.SetPushTask(siteID, taskMapInterface); err != nil {
		return "", err
	}

	// 异步执行推送
	go pm.executePush(task, siteConfig)

	return taskID, nil
}

// executePush 执行推送任务
func (pm *PushManager) executePush(task PushTask, siteConfig *config.SiteConfig) {
	// 更新任务状态为running
	task.Status = "running"
	task.StartedAt = time.Now()

	// 保存任务到Redis
	taskMap, _ := json.Marshal(task)
	var taskMapInterface map[string]interface{}
	json.Unmarshal(taskMap, &taskMapInterface)
	pm.redisClient.SetPushTask(task.SiteID, taskMapInterface)

	// 获取站点的URL列表
	allURLs, err := pm.redisClient.GetURLs(siteConfig.ID)
	if err != nil {
		// 记录错误日志
		task.Status = "failed"
		task.CompletedAt = time.Now()

		// 保存任务到Redis
		taskMap, _ := json.Marshal(task)
		var taskMapInterface map[string]interface{}
		json.Unmarshal(taskMap, &taskMapInterface)
		pm.redisClient.SetPushTask(task.SiteID, taskMapInterface)
		return
	}

	// 检查 URL 列表是否为空
	if len(allURLs) == 0 {
		// URL 列表为空，标记任务为完成
		task.Status = "completed"
		task.CompletedAt = time.Now()
		task.SuccessCount = 0
		task.FailedCount = 0

		// 保存任务到 Redis
		taskMap, _ := json.Marshal(task)
		var taskMapInterface map[string]interface{}
		json.Unmarshal(taskMap, &taskMapInterface)
		pm.redisClient.SetPushTask(task.SiteID, taskMapInterface)
		return
	}

	pushConfig := siteConfig.Prerender.Push

	// 获取今日日期
	today := time.Now().Format("2006-01-02")

	// 获取当前推送进度
	pushOffset, err := pm.redisClient.GetPushOffset(task.SiteID)
	if err != nil {
		pushOffset = 0
	}

	// 推送URL到搜索引擎
	totalPushed := 0
	successCount := 0
	failedCount := 0

	// 分别处理百度和必应的推送，使用不同的偏移量逻辑
	// 推送到百度
	if pushConfig.BaiduAPI != "" && pushConfig.BaiduToken != "" {
		// 计算百度本次推送的URL数量
		var baiduUrlsToPush []string

		// 计算百度本次推送的 URL 数量
		baiduStart := int(pushOffset % int64(len(allURLs)))
		baiduUrlsNeeded := pushConfig.BaiduDailyLimit

		// 计算实际可推送的 URL 数量（不超过 URL 总数）
		baiduActualCount := min(baiduUrlsNeeded, len(allURLs))

		// 计算结束位置（处理循环）
		baiduEnd := baiduStart + baiduActualCount
		if baiduEnd > len(allURLs) {
			// 需要循环：从 start 到末尾 + 从开头到剩余数量
			baiduUrlsToPush = append(allURLs[baiduStart:], allURLs[:baiduEnd-len(allURLs)]...)
		} else {
			baiduUrlsToPush = allURLs[baiduStart:baiduEnd]
		}

		// 执行百度推送
		for _, route := range baiduUrlsToPush {
			// 构建完整URL
			fullURL := buildFullURL(pushConfig.PushDomain, siteConfig.Port, route)

			if err := pm.pushToBaidu(fullURL, route, pushConfig, siteConfig); err != nil {
				failedCount++
			} else {
				successCount++
			}
			totalPushed++

			// 避免推送过快
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 推送到必应
	if pushConfig.BingAPI != "" && pushConfig.BingToken != "" {
		// 计算必应本次推送的URL数量
		var bingUrlsToPush []string

		// 计算必应本次推送的 URL 数量

		// 计算必应本次推送的 URL 数量
		bingStart := int(pushOffset % int64(len(allURLs)))
		bingUrlsNeeded := pushConfig.BingDailyLimit

		// 计算实际可推送的 URL 数量（不超过 URL 总数）
		bingActualCount := min(bingUrlsNeeded, len(allURLs))

		// 计算结束位置（处理循环）
		bingEnd := bingStart + bingActualCount
		if bingEnd > len(allURLs) {
			// 需要循环：从 start 到末尾 + 从开头到剩余数量
			bingUrlsToPush = append(allURLs[bingStart:], allURLs[:bingEnd-len(allURLs)]...)
		} else {
			bingUrlsToPush = allURLs[bingStart:bingEnd]
		}

		// 执行必应推送
		for _, route := range bingUrlsToPush {
			// 构建完整URL
			fullURL := buildFullURL(pushConfig.PushDomain, siteConfig.Port, route)

			if err := pm.pushToBing(fullURL, route, pushConfig, siteConfig); err != nil {
				failedCount++
			} else {
				successCount++
			}
			totalPushed++

			// 避免推送过快
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 推送到 IndexNow (Bing/Yandex/Naver/Seznam)
	if pushConfig.IndexNowEnabled && pushConfig.IndexNowKey != "" {
		// 构建完整 URL 列表
		var indexnowURLs []string
		for _, route := range allURLs {
			fullURL := buildFullURL(pushConfig.PushDomain, siteConfig.Port, route)
			indexnowURLs = append(indexnowURLs, fullURL)
		}

		// IndexNow 单次最多推送 10000 个 URL
		batchSize := 10000
		for i := 0; i < len(indexnowURLs); i += batchSize {
			end := i + batchSize
			if end > len(indexnowURLs) {
				end = len(indexnowURLs)
			}
			batch := indexnowURLs[i:end]

			client := NewIndexNowClient(pushConfig.IndexNowKey)
			host := pushConfig.PushDomain
			if host == "" && len(siteConfig.Domains) > 0 {
				host = siteConfig.Domains[0]
			}

			result, err := client.Push(batch, host)
			if err != nil || !result.Success {
				failedCount += len(batch)
				// 记录失败日志
				errMsg := result.Message
				if err != nil {
					errMsg = fmt.Sprintf("IndexNow push failed: %v", err)
				}
				pm.logPushResult(task.SiteID, siteConfig.Name, fmt.Sprintf("%d URLs", len(batch)), "indexnow", "indexnow", "failed", errMsg)
			} else {
				successCount += len(batch)
				pm.logPushResult(task.SiteID, siteConfig.Name, fmt.Sprintf("%d URLs", len(batch)), "indexnow", "indexnow", "success", result.Message)
			}
			totalPushed += len(batch)
		}
	}

	// 更新每日推送计数
	if totalPushed > 0 {
		pm.redisClient.IncrDailyPushCountWithCount(task.SiteID, totalPushed)
	}

	// 更新推送进度和日期
	// 计算新的偏移量，使用最小的限制作为偏移量计算基准，确保所有搜索引擎都能完成推送
	minLimit := pushConfig.BaiduDailyLimit
	if minLimit == 0 || (pushConfig.BingDailyLimit > 0 && pushConfig.BingDailyLimit < minLimit) {
		minLimit = pushConfig.BingDailyLimit
	}

	// 如果没有设置限制，使用默认值100
	if minLimit == 0 {
		minLimit = 100
	}

	newOffset := pushOffset + int64(minLimit)
	if newOffset >= int64(len(allURLs)) {
		newOffset = 0 // 推送完毕，重置偏移量
	}

	pm.redisClient.SetPushOffset(task.SiteID, newOffset)
	pm.redisClient.SetLastPushDate(task.SiteID, today)

	// 更新任务状态
	task.Status = "completed"
	task.CompletedAt = time.Now()
	task.SuccessCount = successCount
	task.FailedCount = failedCount

	// 保存任务到Redis
	taskMap, _ = json.Marshal(task)
	json.Unmarshal(taskMap, &taskMapInterface)
	pm.redisClient.SetPushTask(task.SiteID, taskMapInterface)

	// 更新站点统计
	pm.redisClient.IncrPushStats(task.SiteID, "success")
	pm.redisClient.IncrPushStats(task.SiteID, "failed")
}

// buildFullURL 构建完整URL
func buildFullURL(pushDomain string, port int, route string) string {
	// 如果路由不是以/开头，添加/
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}

	// 如果没有指定推送域名，使用默认值
	if pushDomain == "" {
		pushDomain = "localhost"
	}

	// 构建URL
	var urlBuilder strings.Builder
	urlBuilder.WriteString("http://")

	// 确保推送域名没有尾部斜杠
	pushDomain = strings.TrimSuffix(pushDomain, "/")
	urlBuilder.WriteString(pushDomain)

	// 只有非80端口才需要显示
	if port != 80 {
		urlBuilder.WriteString(fmt.Sprintf(":%d", port))
	}

	urlBuilder.WriteString(route)

	return urlBuilder.String()
}

// pushToBaidu 推送到百度
func (pm *PushManager) pushToBaidu(url, route string, pushConfig config.PushConfig, siteConfig *config.SiteConfig) error {
	// 构建请求
	req, err := http.NewRequest("POST", pushConfig.BaiduAPI, bytes.NewBuffer([]byte(url)))
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "failed", err.Error())
		return err
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", pushConfig.BaiduToken))

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "failed", err.Error())
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "failed", err.Error())
		return err
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "success", string(body))
		return nil
	}

	// 检查结果
	if success, ok := result["success"].(float64); ok && success > 0 {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "success", string(body))
		return nil
	}

	pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "baidu", "failed", string(body))
	return fmt.Errorf("baidu push failed: %s", string(body))
}

// pushToBing 推送到必应
func (pm *PushManager) pushToBing(url, route string, pushConfig config.PushConfig, siteConfig *config.SiteConfig) error {
	// 构建请求体
	reqBody := map[string]string{
		"apikey": pushConfig.BingToken,
		"url":    url,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "failed", err.Error())
		return err
	}

	// 构建请求
	req, err := http.NewRequest("POST", pushConfig.BingAPI, bytes.NewBuffer(jsonData))
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "failed", err.Error())
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "failed", err.Error())
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "failed", err.Error())
		return err
	}

	// 检查响应状态
	if resp.StatusCode == http.StatusOK {
		pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "success", string(body))
		return nil
	}

	pm.logPushResult(siteConfig.ID, siteConfig.Name, url, route, "bing", "failed", string(body))
	return fmt.Errorf("bing push failed: %s", string(body))
}

// logPushResult 记录推送结果
func (pm *PushManager) logPushResult(siteID, siteName, url, route, searchEngine, status, message string) {
	if pm.redisClient == nil {
		return
	}
	log := PushLog{
		ID:           fmt.Sprintf("log-%s-%d", siteID, time.Now().UnixNano()),
		SiteID:       siteID,
		SiteName:     siteName,
		URL:          url,
		Route:        route,
		SearchEngine: searchEngine,
		Status:       status,
		Message:      message,
		PushTime:     time.Now(),
	}

	// 保存到Redis
	pm.redisClient.AddPushLogStruct(siteID, log)
}

// GetPushStats 获取推送统计
func (pm *PushManager) GetPushStats(siteID string) (map[string]interface{}, error) {
	if pm.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return pm.redisClient.GetPushStatsWithURLCounts(siteID)
}

// GetPushTrend 获取最近15天的推送趋势
func (pm *PushManager) GetPushTrend(siteID string) (map[string]int64, error) {
	if pm.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return pm.redisClient.GetLast15DaysPushCount(siteID)
}

// GetPushLogs 获取推送日志
func (pm *PushManager) GetPushLogs(siteID string, limit, offset int) ([]PushLog, error) {
	if pm.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 从Redis获取日志
	logInterfaces, err := pm.redisClient.GetPushLogs(siteID, limit, offset)
	if err != nil {
		return nil, err
	}

	// 转换为PushLog类型
	pushLogs := make([]PushLog, 0, len(logInterfaces))
	for _, logInterface := range logInterfaces {
		// 将interface{}转换为map[string]interface{}
		if logMap, ok := logInterface.(map[string]interface{}); ok {
			// 转换为PushLog对象
			pushLog := PushLog{
				ID:           logMap["id"].(string),
				SiteID:       logMap["siteId"].(string),
				SiteName:     logMap["siteName"].(string),
				URL:          logMap["url"].(string),
				Route:        logMap["route"].(string),
				SearchEngine: logMap["searchEngine"].(string),
				Status:       logMap["status"].(string),
				Message:      logMap["message"].(string),
			}
			// 转换时间
			if pushTimeStr, ok := logMap["pushTime"].(string); ok {
				if pushTime, err := time.Parse(time.RFC3339, pushTimeStr); err == nil {
					pushLog.PushTime = pushTime
				}
			}
			pushLogs = append(pushLogs, pushLog)
		}
	}

	return pushLogs, nil
}

// GetPushLogCount 获取推送日志总数
func (pm *PushManager) GetPushLogCount(siteID string) (int64, error) {
	if pm.redisClient == nil {
		return 0, fmt.Errorf("redis client is nil")
	}
	return pm.redisClient.GetPushLogCount(siteID)
}

// GetPushConfig 获取推送配置
func (pm *PushManager) GetPushConfig(siteID string) (*config.PushConfig, error) {
	for _, site := range pm.currentConfig().Sites {
		if site.ID == siteID {
			return &site.Prerender.Push, nil
		}
	}
	return nil, fmt.Errorf("site not found: %s", siteID)
}

// UpdatePushConfig 更新推送配置
func (pm *PushManager) UpdatePushConfig(siteID string, pushConfig *config.PushConfig) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	cfg := pm.currentConfig()
	for i, site := range cfg.Sites {
		if site.ID == siteID {
			cfg.Sites[i].Prerender.Push = *pushConfig
			return nil
		}
	}

	return fmt.Errorf("site not found: %s", siteID)
}
