package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
)

// PushManager 推送管理器
type PushManager struct {
	config      *config.Config
	redisClient *redis.Client
	mutex       sync.Mutex
}

// NewPushManager 创建推送管理器实例
func NewPushManager(config *config.Config, redisClient *redis.Client) *PushManager {
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
	// 获取站点配置
	var siteConfig *config.SiteConfig
	for _, site := range pm.config.Sites {
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
	if err := pm.redisClient.SetPushTask(siteID, task); err != nil {
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
	pm.redisClient.SetPushTask(task.SiteID, task)

	// 获取站点的URL列表
	allURLs, err := pm.redisClient.GetURLs(siteConfig.ID)
	if err != nil {
		// 记录错误日志
		task.Status = "failed"
		task.CompletedAt = time.Now()
		pm.redisClient.SetPushTask(task.SiteID, task)
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

		// 计算百度本次推送的URL数量
		baiduStart := pushOffset % len(allURLs)
		baiduEnd := baiduStart + pushConfig.BaiduDailyLimit

		// 如果超过URL总数，循环到开头
		if baiduEnd > len(allURLs) {
			// 推送剩余部分
			baiduUrlsToPush = append(allURLs[baiduStart:], allURLs[:baiduEnd-len(allURLs)]...)
		} else {
			// 正常推送
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

		// 计算必应本次推送的URL数量
		bingStart := pushOffset % len(allURLs)
		bingEnd := bingStart + pushConfig.BingDailyLimit

		// 如果超过URL总数，循环到开头
		if bingEnd > len(allURLs) {
			// 推送剩余部分
			bingUrlsToPush = append(allURLs[bingStart:], allURLs[:bingEnd-len(allURLs)]...)
		} else {
			// 正常推送
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

	// 更新每日推送计数
	if totalPushed > 0 {
		pm.redisClient.IncrDailyPushCount(task.SiteID, totalPushed)
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

	newOffset := pushOffset + minLimit
	if newOffset >= len(allURLs) {
		newOffset = 0 // 推送完毕，重置偏移量
	}

	pm.redisClient.SetPushOffset(task.SiteID, newOffset)
	pm.redisClient.SetLastPushDate(task.SiteID, today)

	// 更新任务状态
	task.Status = "completed"
	task.CompletedAt = time.Now()
	task.SuccessCount = successCount
	task.FailedCount = failedCount
	pm.redisClient.SetPushTask(task.SiteID, task)

	// 更新站点统计
	pm.redisClient.IncrPushStats(task.SiteID, successCount, failedCount)
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
	body, err := ioutil.ReadAll(resp.Body)
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
	body, err := ioutil.ReadAll(resp.Body)
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
	pm.redisClient.AddPushLog(siteID, log)
}

// GetPushStats 获取推送统计
func (pm *PushManager) GetPushStats(siteID string) (map[string]interface{}, error) {
	return pm.redisClient.GetPushStatsWithURLCounts(siteID)
}

// GetPushTrend 获取最近15天的推送趋势
func (pm *PushManager) GetPushTrend(siteID string) (map[string]int64, error) {
	return pm.redisClient.GetLast15DaysPushCount(siteID)
}

// GetPushLogs 获取推送日志
func (pm *PushManager) GetPushLogs(siteID string, limit, offset int) ([]PushLog, error) {
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

// GetPushConfig 获取推送配置
func (pm *PushManager) GetPushConfig(siteID string) (*config.PushConfig, error) {
	for _, site := range pm.config.Sites {
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

	for i, site := range pm.config.Sites {
		if site.ID == siteID {
			pm.config.Sites[i].Prerender.Push = *pushConfig
			return nil
		}
	}

	return fmt.Errorf("site not found: %s", siteID)
}
