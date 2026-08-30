package redis

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	goRedis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// ---------- 测试辅助 ----------

// newGapClient 创建熔断阈值极高的客户端，供错误注入测试使用，
// 避免多次注入失败后被熔断拦截而触达不了原始错误分支
func newGapClient(t *testing.T) *Client {
	t.Helper()
	cl, err := NewClientWithFullConfig("localhost:6379", "", 15, DefaultPoolConfig(), CircuitBreakerConfig{
		FailureThreshold: 1000,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})
	if err != nil {
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

// gapFailCmdHook 通过 go-redis Hook 在真实请求路径上注入命令失败：
// BeforeProcess 返回错误即短路（命令不会真正发出），其余命令正常放行。
// 这是 go-redis 官方中间件扩展点，调用方代码走的是真实客户端管线。
type gapFailCmdHook struct {
	name      string // 需要失败的命令名（小写），空表示匹配全部
	keyPrefix string // 仅当首个 key 参数以该前缀开头时生效，空表示不限制
	remaining int64  // 剩余注入次数（原子），耗尽后不再拦截
}

func (h *gapFailCmdHook) match(cmd goRedis.Cmder) bool {
	if h.name != "" && cmd.Name() != h.name {
		return false
	}
	if h.keyPrefix != "" {
		args := cmd.Args()
		if len(args) < 2 {
			return false
		}
		key, _ := args[1].(string)
		if !strings.HasPrefix(key, h.keyPrefix) {
			return false
		}
	}
	return true
}

func (h *gapFailCmdHook) BeforeProcess(ctx context.Context, cmd goRedis.Cmder) (context.Context, error) {
	if h.match(cmd) && atomic.AddInt64(&h.remaining, -1) >= 0 {
		return ctx, fmt.Errorf("injected %s failure", cmd.Name())
	}
	return ctx, nil
}

func (h *gapFailCmdHook) AfterProcess(ctx context.Context, cmd goRedis.Cmder) error {
	return nil
}

func (h *gapFailCmdHook) BeforeProcessPipeline(ctx context.Context, cmds []goRedis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *gapFailCmdHook) AfterProcessPipeline(ctx context.Context, cmds []goRedis.Cmder) error {
	return nil
}

// ---------- 熔断器 Allow 的半开分支 ----------

func TestGapCircuitBreaker_AllowHalfOpen(t *testing.T) {
	// 同包白盒：直接构造半开状态（对应 Allow 的 case StateHalfOpen 分支）
	cb := &CircuitBreaker{state: StateHalfOpen}
	assert.True(t, cb.Allow())
}

// ---------- 关闭客户端的原始错误分支 ----------

// client_coverage_test.go 的 TestCovStringMethods_ClosedClient 中，这些方法排在
// 5 个失败之后（默认熔断阈值为 5），熔断已打开，未触达各方法内部的原始错误分支。
// 此处在熔断阈值极高的客户端上关闭后调用，覆盖 DelMultiple/Decr/Expire/TTL/
// Publish/GetPushLogs 的 recordFailure 错误分支。
func TestGapClosedClient_RawErrorBranches(t *testing.T) {
	cl := newGapClient(t)
	_ = cl.Close()

	assert.Error(t, cl.DelMultiple([]string{"k1", "k2"}))
	_, err := cl.Decr("k")
	assert.Error(t, err)
	assert.Error(t, cl.Expire("k", time.Minute))
	_, err = cl.TTL("k")
	assert.Error(t, err)
	assert.Error(t, cl.Publish("ch", "msg"))
	_, err = cl.GetPushLogs("s", 10, 0)
	assert.Error(t, err)
}

// ---------- DeleteSiteData 内层 Del 失败分支 ----------

// 覆盖 DeleteSiteData 中 Keys(SCAN) 成功但 Del 失败的分支：
// 真实客户端上 SCAN 正常放行，Del 经 Hook 注入失败
func TestGapDeleteSiteData_DelFails(t *testing.T) {
	cl := newGapClient(t)

	// 先造数据，让 SCAN 能扫到站点键
	assert.NoError(t, cl.Set("site:gap-deldel:main", "x", time.Minute))

	cl.GetRawClient().AddHook(&gapFailCmdHook{name: "del", remaining: 1})
	err := cl.DeleteSiteData("gap-deldel")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "injected del failure")

	// 注入次数已耗尽，清理可正常执行
	assert.NoError(t, cl.Del("site:gap-deldel:main"))
}
