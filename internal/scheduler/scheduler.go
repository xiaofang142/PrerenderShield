package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron          *cron.Cron
	engineManager *prerender.EngineManager
	pushManager   *push.PushManager
	redisClient   *redis.Client
	tasks         map[string][]cron.EntryID // 站点名 -> 任务ID 列表（预热+推送各一条）
	tasksMutex    sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewScheduler 创建新的定时任务调度器
func NewScheduler(engineManager *prerender.EngineManager, redisClient *redis.Client, cfg *config.Config) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建cron实例，支持秒级精度
	c := cron.New(cron.WithSeconds())

	return &Scheduler{
		cron:          c,
		engineManager: engineManager,
		pushManager:   push.NewPushManager(cfg, redisClient),
		redisClient:   redisClient,
		tasks:         make(map[string][]cron.EntryID),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动定时任务调度器
func (s *Scheduler) Start() {
	// 启动cron调度器
	s.cron.Start()

	// 启动监控协程
	s.wg.Add(1)
	go s.monitorSites()

	logging.DefaultLogger.Info("Scheduler started")
}

// Stop 停止定时任务调度器
func (s *Scheduler) Stop() {
	// 取消上下文
	s.cancel()

	// 停止cron调度器
	s.cron.Stop()

	// 等待监控协程结束
	s.wg.Wait()

	logging.DefaultLogger.Info("Scheduler stopped")
}

// monitorSites 监控站点配置变化，动态调整定时任务
func (s *Scheduler) monitorSites() {
	defer s.wg.Done()

	// 初始加载所有站点的定时任务
	s.reloadSites()

	// 定期检查站点配置变化（每30秒）
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.reloadSites()
		}
	}
}

// reloadSites 重新加载所有站点的定时任务
func (s *Scheduler) reloadSites() {
	// 获取所有站点名称
	siteNames := s.engineManager.ListSites()

	// 记录当前所有站点名
	currentSites := make(map[string]bool)

	// 从配置管理器获取站点配置
	cm := config.GetInstance()
	cfg := cm.GetConfig()

	// 构建站点ID -> 配置映射
	siteConfigMap := make(map[string]config.PrerenderConfig)
	for _, site := range cfg.Sites {
		siteConfigMap[site.ID] = site.Prerender
	}

	// 全程持写锁完成"检查+创建/更新/删除"，消除 RLock→Lock 的 TOCTOU 竞态窗口
	// （并发调用会导致重复 AddFunc 与旧 EntryID 永续泄漏）
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	for _, siteName := range siteNames {
		currentSites[siteName] = true

		prerenderCfg, exists := siteConfigMap[siteName]
		if !exists {
			prerenderCfg = config.PrerenderConfig{}
		}

		if _, taskExists := s.tasks[siteName]; taskExists {
			s.updateTaskLocked(siteName, prerenderCfg)
		} else {
			s.createTaskLocked(siteName, prerenderCfg)
		}
	}

	// 删除不再存在的站点的任务
	for siteName := range s.tasks {
		if !currentSites[siteName] {
			s.removeTaskLocked(siteName)
		}
	}
}

// createTaskLocked 为站点创建定时任务（调用方必须已持 tasksMutex 写锁）
func (s *Scheduler) createTaskLocked(siteName string, config config.PrerenderConfig) {
	ids := make([]cron.EntryID, 0, 2)

	// 为预热任务创建定时任务
	if config.Preheat.Enabled && config.Preheat.Schedule != "" {
		preheatTaskFunc := func() {
			s.executePreheat(siteName)
		}
		if id, err := s.cron.AddFunc(config.Preheat.Schedule, preheatTaskFunc); err != nil {
			logging.DefaultLogger.Info("Failed to add preheat cron task for site %s: %v\n", siteName, err)
		} else {
			ids = append(ids, id)
			logging.DefaultLogger.Info("Created preheat cron task for site %s with schedule: %s\n", siteName, config.Preheat.Schedule)
		}
	}

	// 为推送任务创建定时任务
	if config.Push.Enabled {
		pushTaskFunc := func() {
			s.executePush(siteName)
		}
		cronExpr := config.Push.Schedule
		if cronExpr == "" {
			cronExpr = "0 0 8 * * *"
		}
		if id, err := s.cron.AddFunc(cronExpr, pushTaskFunc); err != nil {
			logging.DefaultLogger.Info("Failed to add push cron task for site %s: %v\n", siteName, err)
		} else {
			ids = append(ids, id)
			logging.DefaultLogger.Info("Created push cron task for site %s with schedule: %s\n", siteName, cronExpr)
		}
	}

	if len(ids) > 0 {
		s.tasks[siteName] = ids
	}
}

// updateTaskLocked 更新站点的定时任务（调用方必须已持写锁）：先移除全部旧 Entry 再重建，
// 避免配置变化后旧调度残留
func (s *Scheduler) updateTaskLocked(siteName string, config config.PrerenderConfig) {
	s.removeTaskLocked(siteName)
	s.createTaskLocked(siteName, config)
}

// removeTaskLocked 移除站点的全部定时任务（调用方必须已持写锁）。
// 此前每站点仅记录单个 EntryID，预热+推送双任务时另一个必然泄漏在 cron 中
func (s *Scheduler) removeTaskLocked(siteName string) {
	entryIDs, exists := s.tasks[siteName]
	if !exists {
		return
	}
	for _, entryID := range entryIDs {
		s.cron.Remove(entryID)
	}
	delete(s.tasks, siteName)
	logging.DefaultLogger.Info("Removed cron tasks for site %s (%d entries)\n", siteName, len(entryIDs))
}

func (s *Scheduler) executePreheat(siteName string) {
	logging.DefaultLogger.Info("Executing preheat for site %s at %s\n", siteName, time.Now().Format("2006-01-02 15:04:05"))

	// 获取站点的引擎实例
	engine, exists := s.engineManager.GetEngine(siteName)
	if !exists {
		logging.DefaultLogger.Info("Engine not found for site %s\n", siteName)
		return
	}

	// 从Redis获取站点的URL列表
	urls, err := s.redisClient.GetURLs(siteName)
	if err != nil || len(urls) == 0 {
		logging.DefaultLogger.Info("Failed to get URLs for site %s: %v\n", siteName, err)
		return
	}

	// 注入预热通道 TTL 配置（分级规则首中 > 站点 CacheTTL > 引擎默认）
	if cm := config.GetInstance(); cm != nil {
		for _, site := range cm.GetConfig().Sites {
			if site.ID == siteName {
				engine.SetPreheatTTLConfig(site.Prerender.CacheTTL, site.Prerender.TTLRules)
				break
			}
		}
	}

	// 调用引擎的创建预热任务方法
	taskID, err := engine.CreatePreheatTask(siteName, urls)
	if err != nil {
		logging.DefaultLogger.Info("Failed to trigger preheat for site %s: %v\n", siteName, err)
		return
	}

	// 存储当前预热任务ID
	s.redisClient.Set(fmt.Sprintf("site:%s:preheat:current_task", siteName), taskID, 24*time.Hour)

	logging.DefaultLogger.Info("Preheat triggered for site %s with task ID: %s\n", siteName, taskID)
}

// executePush 执行站点的推送任务
func (s *Scheduler) executePush(siteName string) {
	logging.DefaultLogger.Info("Executing push for site %s at %s\n", siteName, time.Now().Format("2006-01-02 15:04:05"))

	// 调用推送管理器的TriggerPush方法
	_, err := s.pushManager.TriggerPush(siteName)
	if err != nil {
		logging.DefaultLogger.Info("Failed to trigger push for site %s: %v\n", siteName, err)
		return
	}

	logging.DefaultLogger.Info("Push completed for site %s\n", siteName)
}

// AddManualTask 添加手动触发的预热任务
func (s *Scheduler) AddManualTask(siteName string) {
	// 异步执行预热任务
	go s.executePreheat(siteName)
}

// GetTaskStatus 获取站点的任务状态
func (s *Scheduler) GetTaskStatus(siteName string) (bool, string) {
	// 检查任务是否存在
	s.tasksMutex.RLock()
	ids, exists := s.tasks[siteName]
	s.tasksMutex.RUnlock()

	if !exists {
		return false, "not scheduled"
	}

	// 取该站点任一任务的下次执行时间（预热/推送取最早者）
	idSet := make(map[cron.EntryID]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	entries := s.cron.Entries()
	earliest := ""
	for _, entry := range entries {
		if !idSet[entry.ID] {
			continue
		}
		next := entry.Next.Format("2006-01-02 15:04:05")
		if earliest == "" || next < earliest {
			earliest = next
		}
	}
	if earliest != "" {
		return true, earliest
	}
	return false, "not found"
}

// ListTasks 列出所有定时任务
func (s *Scheduler) ListTasks() map[string]string {
	result := make(map[string]string)

	// 获取所有任务
	entries := s.cron.Entries()

	// 反向映射：entryID -> siteName
	s.tasksMutex.RLock()
	entryToSite := make(map[cron.EntryID]string)
	for siteName, entryIDs := range s.tasks {
		for _, id := range entryIDs {
			entryToSite[id] = siteName
		}
	}
	s.tasksMutex.RUnlock()

	// 构建结果
	for _, entry := range entries {
		if siteName, exists := entryToSite[entry.ID]; exists {
			result[siteName] = entry.Next.Format("2006-01-02 15:04:05")
		}
	}

	return result
}
