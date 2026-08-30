package prerender

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CreatePreheatTask 创建预热任务
func (e *engine) CreatePreheatTask(siteID string, urls []string) (string, error) {
	// 生成任务ID
	taskID := uuid.New().String()

	// 创建任务信息
	task := map[string]interface{}{
		"task_id":        taskID,
		"site_id":        siteID,
		"urls":           urls,
		"total_urls":     len(urls),
		"completed_urls": 0,
		"failed_urls":    0,
		"status":         "pending",
		"created_at":     time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}

	// 存储任务信息到Redis
	if err := e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour); err != nil {
		return "", fmt.Errorf("failed to save task: %w", err)
	}

	// 将任务添加到站点任务列表
	if err := e.redisClient.SetAdd(fmt.Sprintf("site:%s:tasks", siteID), taskID); err != nil {
		return "", fmt.Errorf("failed to add task to site: %w", err)
	}

	// 开始执行任务
	go e.executePreheatTask(taskID)

	return taskID, nil
}

// resolvePreheatURL 把队列中的 URL 补全为绝对地址：
// 队列混合了 sitemap 全量 URL 与 crawler 的纯 route 形态（后者旧实现
// 直接当绝对 URL 渲染必然失败或产出永不命中的死键）。
func (e *engine) resolvePreheatURL(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	base := e.getPreheatBaseURL()
	if base == "" {
		return u // 无基址时保持原样，交由渲染失败路径标记 failed
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return base + u
}

// executePreheatTask 执行预热任务
// P0-9: 使用 semaphore + worker pool 并发渲染
// P0-10: 响应 ctx.Done() 和任务 status==cancelled
func (e *engine) executePreheatTask(taskID string) {
	// 获取任务信息
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return
	}

	// P0-10: 启动时检查任务是否已被取消
	if s, _ := task["status"].(string); s == "cancelled" {
		return
	}

	// 更新任务状态为运行中
	task["status"] = "running"
	task["updated_at"] = time.Now().Unix()
	e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)

	// 提取任务参数
	siteID, _ := task["site_id"].(string)
	urls, _ := task["urls"].([]interface{})

	// P0-10: 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 注册取消回调：当 Redis 中任务状态变为 cancelled 时，自动 cancel
	go e.watchPreheatCancellation(ctx, taskID, cancel)

	// P0-9: 并发执行渲染任务
	var (
		completed int64
		failed    int64
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	// P0-9: 限制并发数为 ConcurrencyManager 的当前限制
	maxConcurrent := e.concurrencyManager.GetCurrentLimit()
	if maxConcurrent < 1 {
		maxConcurrent = 5
	}
	sem := make(chan struct{}, maxConcurrent)

	for _, urlInterface := range urls {
		// P0-10: 检查是否已取消
		select {
		case <-ctx.Done():
			goto done
		default:
		}

		url, ok := urlInterface.(string)
		if !ok {
			mu.Lock()
			failed++
			mu.Unlock()
			continue
		}

		// 再次检查取消 (sem 等待时)
		select {
		case <-ctx.Done():
			goto done
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()

			// P0-10: 单个 goroutine 内也响应取消
			if ctx.Err() != nil {
				return
			}

			// 统一渲染管线：与实时请求共享状态码捕获/质检/SEO注入/信封TTL
			// （旧实现走 e.Render()+手工写缓存 24h 的独立路径，产出死键且 TTL 不接线）
			target := e.resolvePreheatURL(u)
			res, err := e.RenderAndCache(RenderRequest{
				SiteID:    siteID,
				URL:       target,
				Opts:      RenderOptions{Timeout: 30 * time.Second, CacheTTL: e.preheatEffectiveTTL(target)},
				UserAgent: "PrerenderShieldPreheat/1.0",
			})
			if err != nil {
				e.redisClient.SetURLPreheatStatus(siteID, u, "failed")
				mu.Lock()
				failed++
				mu.Unlock()
			} else if res.Thin || res.Status >= 500 {
				// 空壳页/服务端错误不入缓存，标记为失败避免污染预热统计口径
				e.redisClient.SetURLPreheatStatus(siteID, u, "failed")
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				e.redisClient.SetURLPreheatStatus(siteID, u, "success")
				mu.Lock()
				completed++
				mu.Unlock()
			}

			// 原子化更新任务进度 (每 5 个 URL 或最后一个时写一次，避免 Redis 抖动)
			mu.Lock()
			processed := completed + failed
			shouldUpdate := processed%5 == 0 || processed == int64(len(urls))
			mu.Unlock()

			if shouldUpdate {
				mu.Lock()
				task["completed_urls"] = completed
				task["failed_urls"] = failed
				task["updated_at"] = time.Now().Unix()
				if len(urls) > 0 {
					task["progress"] = int(float64(processed) / float64(len(urls)) * 100)
				}
				snapshot := completed
				snapshotFailed := failed
				mu.Unlock()
				_ = snapshot
				_ = snapshotFailed
				e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)
			}
		}(url)
	}

done:
	wg.Wait()

	// P0-10: 根据是否被取消决定最终状态
	finalStatus := "completed"
	if ctx.Err() != nil {
		finalStatus = "cancelled"
	} else if failed > 0 {
		finalStatus = "completed_with_errors"
	}

	task["status"] = finalStatus
	task["updated_at"] = time.Now().Unix()
	task["completed_urls"] = completed
	task["failed_urls"] = failed
	task["progress"] = 100
	e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)
}

// watchPreheatCancellation (P0-10) 监控任务状态，被取消时触发 context cancel
func (e *engine) watchPreheatCancellation(ctx context.Context, taskID string, cancel context.CancelFunc) {
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			task := make(map[string]interface{})
			if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
				// Redis 瞬时抖动不应终止预热任务：记录后继续下一轮检查，
				// 仅在明确读到 status==cancelled 时才取消
				continue
			}
			if status, _ := task["status"].(string); status == "cancelled" {
				return
			}
		}
	}
}

// GetPreheatTaskStatus 获取预热任务状态
func (e *engine) GetPreheatTaskStatus(taskID string) (map[string]interface{}, error) {
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	return task, nil
}

// ListPreheatTasks 列出站点的预热任务
func (e *engine) ListPreheatTasks(siteID string) ([]map[string]interface{}, error) {
	// 获取站点的任务ID列表
	taskIDs, err := e.redisClient.SetMembers(fmt.Sprintf("site:%s:tasks", siteID))
	if err != nil {
		return nil, fmt.Errorf("failed to get task IDs: %w", err)
	}

	// 获取每个任务的详细信息
	tasks := []map[string]interface{}{}
	for _, taskID := range taskIDs {
		task, err := e.GetPreheatTaskStatus(taskID)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// CancelPreheatTask 取消预热任务
// P0-10: 实际生效，watchPreheatCancellation 会检测到状态变更并取消执行
func (e *engine) CancelPreheatTask(taskID string) error {
	// 获取任务信息
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 检查任务状态
	status, _ := task["status"].(string)
	if status == "completed" || status == "completed_with_errors" || status == "cancelled" {
		return fmt.Errorf("task cannot be cancelled (current status: %s)", status)
	}

	// 更新任务状态为取消
	// watchPreheatCancellation 会每秒轮询状态，发现变更后自动取消 worker goroutine
	task["status"] = "cancelled"
	task["updated_at"] = time.Now().Unix()
	task["cancel_requested_at"] = time.Now().Unix()

	return e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)
}

// CleanupPreheatTasks 清理过期的预热任务
func (e *engine) CleanupPreheatTasks() error {
	// 获取所有预热任务
	keys, err := e.redisClient.Keys("task:preheat:*")
	if err != nil {
		return fmt.Errorf("failed to get task keys: %w", err)
	}

	// 清理24小时前创建的已完成任务
	now := time.Now().Unix()
	for _, key := range keys {
		task := make(map[string]interface{})
		if err := e.redisClient.GetJSON(key, &task); err != nil {
			continue
		}

		createdAt, ok := task["created_at"].(float64)
		if !ok {
			continue
		}

		status, _ := task["status"].(string)
		if (status == "completed" || status == "completed_with_errors" || status == "cancelled") && now-int64(createdAt) > 24*3600 {
			e.redisClient.Del(key)
		}
	}

	return nil
}
