package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"prerender-shield/internal/config"
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
	tasks         map[string]cron.EntryID // 站点名 -> 任务ID
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
		tasks:         make(map[string]cron.EntryID),
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
	
	fmt.Println("Scheduler started")
}

// Stop 停止定时任务调度器
func (s *Scheduler) Stop() {
	// 取消上下文
	s.cancel()
	
	// 停止cron调度器
	s.cron.Stop()
	
	// 等待监控协程结束
	s.wg.Wait()
	
	fmt.Println("Scheduler stopped")
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
	
	// 为每个站点创建或更新定时任务
	for _, siteName := range siteNames {
		currentSites[siteName] = true
		
		// 简化实现：直接创建或更新任务，不检查配置
		// 检查是否已存在该站点的任务
		s.tasksMutex.RLock()
		_, taskExists := s.tasks[siteName]
		s.tasksMutex.RUnlock()
		
		// 简化实现：使用默认配置创建任务
		defaultConfig := config.PrerenderConfig{}
		if taskExists {
			// 任务已存在，更新任务
			s.updateTask(siteName, defaultConfig)
		} else {
			// 任务不存在，创建新任务
			s.createTask(siteName, defaultConfig)
		}
	}
	
	// 删除不再存在的站点的任务
	s.tasksMutex.RLock()
	for siteName := range s.tasks {
		if !currentSites[siteName] {
			// 站点已不存在，删除任务
			go s.removeTask(siteName)
		}
	}
	s.tasksMutex.RUnlock()
}

// createTask 为站点创建定时任务
func (s *Scheduler) createTask(siteName string, config config.PrerenderConfig) {
	// 为预热任务创建定时任务
	if config.Preheat.Enabled && config.Preheat.Schedule != "" {
		// 创建预热任务函数
		preheatTaskFunc := func() {
			s.executePreheat(siteName)
		}
		
		// 添加到cron调度器
		_, err := s.cron.AddFunc(config.Preheat.Schedule, preheatTaskFunc)
		if err != nil {
			fmt.Printf("Failed to add preheat cron task for site %s: %v\n", siteName, err)
		} else {
			fmt.Printf("Created preheat cron task for site %s with schedule: %s\n", siteName, config.Preheat.Schedule)
		}
	}

	// 为推送任务创建定时任务
	if config.Push.Enabled {
		// 创建推送任务函数
		pushTaskFunc := func() {
			s.executePush(siteName)
		}
		
		// 固定每天早上8点推送
		cronExpr := "0 0 8 * * *"
		
		// 添加到cron调度器
		_, err := s.cron.AddFunc(cronExpr, pushTaskFunc)
		if err != nil {
			fmt.Printf("Failed to add push cron task for site %s: %v\n", siteName, err)
		} else {
			fmt.Printf("Created push cron task for site %s with schedule: %s\n", siteName, cronExpr)
		}
	}
}

// updateTask 更新站点的定时任务
func (s *Scheduler) updateTask(siteName string, config config.PrerenderConfig) {
	// 简化实现：直接删除旧任务，创建新任务
	s.removeTask(siteName)
	s.createTask(siteName, config)
}

// removeTask 移除站点的定时任务
func (s *Scheduler) removeTask(siteName string) {
	// 获取任务ID
	s.tasksMutex.RLock()
	entryID, exists := s.tasks[siteName]
	s.tasksMutex.RUnlock()
	
	if !exists {
		return
	}
	
	// 从cron调度器中移除任务
	s.cron.Remove(entryID)
	
	// 从任务映射中移除
	s.tasksMutex.Lock()
	delete(s.tasks, siteName)
	s.tasksMutex.Unlock()
	
	fmt.Printf("Removed cron task for site %s\n", siteName)
}

// executePreheat 执行站点的预热任务
func (s *Scheduler) executePreheat(siteName string) {
	fmt.Printf("Executing preheat for site %s at %s\n", siteName, time.Now().Format("2006-01-02 15:04:05"))
	
	// 获取站点的引擎实例
	engine, exists := s.engineManager.GetEngine(siteName)
	if !exists {
		fmt.Printf("Engine not found for site %s\n", siteName)
		return
	}
	
	// 简化实现：直接调用引擎的TriggerPreheat方法
	_, err := engine.TriggerPreheat()
	if err != nil {
		fmt.Printf("Failed to trigger preheat for site %s: %v\n", siteName, err)
		return
	}
	
	fmt.Printf("Preheat completed for site %s\n", siteName)
}

// executePush 执行站点的推送任务
func (s *Scheduler) executePush(siteName string) {
	fmt.Printf("Executing push for site %s at %s\n", siteName, time.Now().Format("2006-01-02 15:04:05"))
	
	// 调用推送管理器的TriggerPush方法
	_, err := s.pushManager.TriggerPush(siteName)
	if err != nil {
		fmt.Printf("Failed to trigger push for site %s: %v\n", siteName, err)
		return
	}
	
	fmt.Printf("Push completed for site %s\n", siteName)
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
	entryID, exists := s.tasks[siteName]
	s.tasksMutex.RUnlock()
	
	if !exists {
		return false, "not scheduled"
	}
	
	// 获取任务的下次执行时间
	entries := s.cron.Entries()
	for _, entry := range entries {
		if entry.ID == entryID {
			nextRun := entry.Next.Format("2006-01-02 15:04:05")
			return true, nextRun
		}
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
	for siteName, entryID := range s.tasks {
		entryToSite[entryID] = siteName
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
