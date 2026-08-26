package prerender

import (
	"os"
	"testing"
)

// TestMain 放开 chromium 全局进程硬上限（见 pool 包同名处理）：
// 与 pool 包测试并发运行时共享系统进程预算，避免互相拒绝创建实例导致超时。
func TestMain(m *testing.M) {
	os.Setenv("PRERENDER_PROCESS_CAP", "100000")
	os.Exit(m.Run())
}
