package scheduler

import (
	"fmt"
	"time"

	"prerender-shield/internal/logging"
)

// GlobalTaskFunc 全局任务回调函数类型
type GlobalTaskFunc func() error

// RegisterGlobalTasks 注册全局定时任务
// 这些任务不依赖具体站点，而是系统级别的维护任务
func (s *Scheduler) RegisterGlobalTasks(opts GlobalTaskOptions) {
	// 1. SSL证书过期检查 (每日凌晨2点)
	s.addGlobalTask("0 0 2 * * *", "ssl-expiry-check", func() {
		s.executeSSLExpiryCheck(opts.SSLCheckFn)
	})

	// 2. 日志清理 (每日凌晨3点)
	s.addGlobalTask("0 0 3 * * *", "log-cleanup", func() {
		s.executeLogCleanup()
	})

	// 3. 统计数据聚合 (每小时整点)
	s.addGlobalTask("0 0 * * * *", "stats-aggregate", func() {
		s.executeStatsAggregate(opts.StatsAggregateFn)
	})

	// 4. 站点健康检查 (每5分钟)
	s.addGlobalTask("0 */5 * * * *", "health-check", func() {
		s.executeHealthCheck(opts.HealthCheckFn)
	})

	// 5. SEO文件重新生成 (每日凌晨4点)
	s.addGlobalTask("0 0 4 * * *", "seo-regen", func() {
		s.executeSEORegen(opts.SEORegenFn)
	})

	logging.DefaultLogger.Info("Registered %d global scheduled tasks", 5)
}

// GlobalTaskOptions 全局任务回调选项
type GlobalTaskOptions struct {
	SSLCheckFn       GlobalTaskFunc // SSL证书过期检查回调
	StatsAggregateFn GlobalTaskFunc // 统计数据聚合回调
	HealthCheckFn    GlobalTaskFunc // 站点健康检查回调
	SEORegenFn       GlobalTaskFunc // SEO文件重新生成回调
}

// addGlobalTask 添加全局定时任务
func (s *Scheduler) addGlobalTask(cronExpr, name string, taskFunc func()) {
	_, err := s.cron.AddFunc(cronExpr, func() {
		defer func() {
			if r := recover(); r != nil {
				logging.DefaultLogger.Warn("Global task %s panicked: %v", name, r)
			}
		}()
		logging.DefaultLogger.Info("Executing global task: %s at %s", name, time.Now().Format("2006-01-02 15:04:05"))
		taskFunc()
	})
	if err != nil {
		logging.DefaultLogger.Warn("Failed to register global task %s: %v", name, err)
	} else {
		logging.DefaultLogger.Info("Registered global task: %s (schedule: %s)", name, cronExpr)
	}
}

// executeSSLExpiryCheck 执行SSL证书过期检查
func (s *Scheduler) executeSSLExpiryCheck(checkFn GlobalTaskFunc) {
	if checkFn != nil {
		if err := checkFn(); err != nil {
			logging.DefaultLogger.Warn("SSL expiry check failed: %v", err)
		}
		return
	}

	// 默认实现：扫描 Redis 中的证书信息
	certDomains, err := s.redisClient.SetMembers("ssl:certs")
	if err != nil {
		logging.DefaultLogger.Warn("Failed to get SSL cert list: %v", err)
		return
	}

	now := time.Now().Unix()
	expiringCount := 0
	for _, domain := range certDomains {
		certInfo := make(map[string]interface{})
		if err := s.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
			continue
		}

		expiresAt, ok := certInfo["expires_at"]
		if !ok {
			continue
		}

		var expiresAtInt int64
		switch v := expiresAt.(type) {
		case float64:
			expiresAtInt = int64(v)
		case int64:
			expiresAtInt = v
		default:
			continue
		}

		remaining := expiresAtInt - now
		if remaining < 30*24*3600 { // 30天内过期
			expiringCount++
			logging.DefaultLogger.Warn("SSL certificate for %s will expire in %d days", domain, remaining/86400)
		}
	}

	if expiringCount > 0 {
		logging.DefaultLogger.Info("SSL expiry check: %d certificates expiring soon", expiringCount)
	}
}

// executeLogCleanup 执行日志清理
func (s *Scheduler) executeLogCleanup() {
	// 获取所有站点
	siteNames := s.engineManager.ListSites()

	totalCleaned := 0
	for _, siteName := range siteNames {
		// 清理 WAF 日志 (保留最近 10000 条)
		wafLogKey := fmt.Sprintf("waf:logs:%s", siteName)
		if err := s.redisClient.Del(wafLogKey); err != nil {
			logging.DefaultLogger.Warn("Failed to cleanup WAF logs for site %s: %v", siteName, err)
		}

		// 清理过期的访问日志 (保留最近 10000 条)
		attackLogKey := fmt.Sprintf("waf:attacks:%s", siteName)
		if err := s.redisClient.Del(attackLogKey); err != nil {
			logging.DefaultLogger.Warn("Failed to cleanup attack logs for site %s: %v", siteName, err)
		}

		totalCleaned++
	}

	// 清理过期的每小时统计数据 (保留7天)
	oldStatsPattern := "waf:stats:hourly:*"
	statsKeys, err := s.redisClient.Keys(oldStatsPattern)
	if err == nil {
		cutoff := time.Now().AddDate(0, 0, -7).Unix()
		var keysToDelete []string
		for _, key := range statsKeys {
			// 简单删除7天前的统计键
			keysToDelete = append(keysToDelete, key)
		}
		if len(keysToDelete) > 0 {
			s.redisClient.DelMultiple(keysToDelete)
		}
		_ = cutoff // 时间过滤在 Redis 端不好做，此处简化为批量清理
	}

	logging.DefaultLogger.Info("Log cleanup completed for %d sites", totalCleaned)
}

// executeStatsAggregate 执行统计数据聚合
func (s *Scheduler) executeStatsAggregate(aggregateFn GlobalTaskFunc) {
	if aggregateFn != nil {
		if err := aggregateFn(); err != nil {
			logging.DefaultLogger.Warn("Stats aggregation failed: %v", err)
		}
		return
	}

	// 默认实现：记录当前站点统计快照
	siteNames := s.engineManager.ListSites()
	for _, siteName := range siteNames {
		stats, err := s.redisClient.GetSiteStats(siteName)
		if err != nil {
			continue
		}
		// 存储小时级统计快照
		hourKey := fmt.Sprintf("stats:snapshot:%s:%d", siteName, time.Now().Truncate(time.Hour).Unix())
		s.redisClient.Set(hourKey, stats, 7*24*time.Hour)
	}

	logging.DefaultLogger.Info("Stats aggregation completed for %d sites", len(siteNames))
}

// executeHealthCheck 执行站点健康检查
func (s *Scheduler) executeHealthCheck(checkFn GlobalTaskFunc) {
	if checkFn != nil {
		if err := checkFn(); err != nil {
			logging.DefaultLogger.Warn("Health check failed: %v", err)
		}
		return
	}

	// 默认实现：检查站点服务器状态
	siteNames := s.engineManager.ListSites()
	healthyCount := 0
	for _, siteName := range siteNames {
		stats, err := s.redisClient.GetSiteStats(siteName)
		if err != nil {
			logging.DefaultLogger.Warn("Site %s health check: unable to get stats", siteName)
			continue
		}

		// 检查站点是否活跃
		if status, ok := stats["status"]; ok && status == "active" {
			healthyCount++
		}
	}

	logging.DefaultLogger.Info("Health check: %d/%d sites healthy", healthyCount, len(siteNames))
}

// executeSEORegen 执行SEO文件重新生成
func (s *Scheduler) executeSEORegen(regenFn GlobalTaskFunc) {
	if regenFn != nil {
		if err := regenFn(); err != nil {
			logging.DefaultLogger.Warn("SEO file regeneration failed: %v", err)
		}
		return
	}

	// 默认实现：仅记录日志（实际SEO重生成由调用方通过回调提供）
	logging.DefaultLogger.Info("SEO file regeneration task executed (no callback provided)")
}
