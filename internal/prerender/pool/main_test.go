package pool

import (
	"os"
	"testing"
)

// TestMain 放开全局进程硬上限：go test 并发运行多个包的二进制时，
// 系统级 chromedp 进程预算被各二进制共享，默认上限会导致
// createInstance 被拒绝、Acquire 永久阻塞（全量测试超时的根因）。
func TestMain(m *testing.M) {
	os.Setenv("PRERENDER_PROCESS_CAP", "100000")
	os.Exit(m.Run())
}
