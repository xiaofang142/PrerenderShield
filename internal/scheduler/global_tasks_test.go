package scheduler

import (
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
)

func newTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	em := prerender.NewEngineManager(client, nil, 1)
	return NewScheduler(em, client, &config.Config{})
}

func TestGlobalTasks_SchedulerLifecycle(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	em := prerender.NewEngineManager(client, nil, 1)
	s := NewScheduler(em, client, &config.Config{})
	s.Start()
	time.Sleep(100 * time.Millisecond) // 让 monitorSites 初始 reload 完成
	s.Stop()
	// 双重 Stop 幂等
	s.Stop()
}

func TestScheduler_AddRemoveManualTask(t *testing.T) {
	s := newTestScheduler(t) // 不 Start：nil engineManager 下 monitorSites 会 panic，注册语义用 cron 未启动态验证

	// 语义澄清：AddManualTask = 立即异步触发一次预热（executePreheat），非注册 cron 条目；
	// 引擎缺失时执行层自会失败记日志，此处只验证不 panic
	s.AddManualTask("nonexistent-site")
	if exists, status := s.GetTaskStatus("nonexistent-site"); exists || status != "not scheduled" {
		t.Fatalf("manual trigger must not create cron entry, got exists=%v status=%q", exists, status)
	}
	if tasks := s.ListTasks(); len(tasks) != 0 {
		t.Fatalf("ListTasks=%v", tasks)
	}
}

func TestExecuteLogCleanup_NoPanic(t *testing.T) {
	s := newTestScheduler(t)
	// 无匹配键时静默通过
	s.executeLogCleanup()
}

func TestExecuteGlobalTasks_InjectedFuncs(t *testing.T) {
	s := newTestScheduler(t)

	called := map[string]bool{}
	s.executeSSLExpiryCheck(func() error { called["ssl"] = true; return nil })
	s.executeStatsAggregate(func() error { called["stats"] = true; return nil })
	s.executeHealthCheck(func() error { called["health"] = true; return nil })
	s.executeSEORegen(func() error { called["seo"] = true; return nil })

	for _, k := range []string{"ssl", "stats", "health", "seo"} {
		if !called[k] {
			t.Fatalf("injected %s check not invoked", k)
		}
	}

	// 回调报错不 panic（记日志路径）
	s.executeSSLExpiryCheck(func() error { return errTest })
	s.executeStatsAggregate(func() error { return errTest })
	s.executeHealthCheck(func() error { return errTest })
	s.executeSEORegen(func() error { return errTest })
}

var errTest = &testError{}

type testError struct{}

func (*testError) Error() string { return "test error" }

func TestExecuteSSLExpiryCheck_DefaultScan(t *testing.T) {
	s := newTestScheduler(t)
	// 空 Redis（无 ssl:certs 成员）时静默通过
	s.executeSSLExpiryCheck(nil)
	// 注入一个即将过期的证书元数据
	s.redisClient.SetAdd("ssl:certs", "expiring.example")
	s.redisClient.SaveJSON("ssl:cert:expiring.example", map[string]interface{}{
		"expires_at": float64(time.Now().Add(-24 * time.Hour).Unix()),
	}, time.Hour)
	s.executeSSLExpiryCheck(nil) // 不 panic 即通过（过期路径记日志/通知由 monitor 承担）
	s.redisClient.SetRemove("ssl:certs", "expiring.example")
	s.redisClient.Del("ssl:cert:expiring.example")
}

// redisNewClient 连接本地 Redis（测试环境约定），失败时供 skip
func redisNewClient() (*redis.Client, error) {
	return redis.NewClient("localhost:6379", "", 0)
}
