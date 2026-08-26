package pool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// JanitorLogger 清扫器日志接口（与项目 logging.Logger 匹配，避免直接依赖 zap）
type JanitorLogger interface {
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
}

// chromedp 进程特征：chromedp 为每个浏览器实例创建
// `<tmp>/chromedp-runner<pid><rand>` 的 user-data-dir，该标记可用于区分
// 本产品拉起的无头浏览器与用户桌面 Chrome。
const chromedpMarker = "chromedp-runner"

// HardProcessCap 每实例会派生多个 OS 进程（browser/gpu/renderer/utility），
// 全局进程数硬上限按最大实例数放大，防止异常累积拖垮宿主机。
// 可通过环境变量 PRERENDER_PROCESS_CAP 显式覆盖（测试并发运行时
// 多个二进制共享系统进程预算，需要放开以避免互相拒绝创建实例）。
func (p *Pool) HardProcessCap() int {
	if v := os.Getenv("PRERENDER_PROCESS_CAP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return p.config.MaxInstances*8 + 16
}

// listChromedpProcesses 返回当前系统中带 chromedp-runner 标记的进程 PID 列表。
// 使用 ps 命令实现（Linux/macOS 通用），解析失败时返回错误由调用方降级。
func listChromedpProcesses() ([]int, error) {
	out, err := exec.Command("ps", "-eo", "pid=", "-o", "args=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}
	pids := make([]int, 0, 32)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, chromedpMarker) {
			continue
		}
		pidStr := strings.Fields(line)[0]
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 || pid == os.Getpid() {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// CountChromiumProcesses 统计当前 chromedp 无头浏览器相关进程数
func CountChromiumProcesses() int {
	pids, err := listChromedpProcesses()
	if err != nil {
		return 0
	}
	return len(pids)
}

// SweepOrphans 清理孤儿无头浏览器进程与残留的 chromedp 临时目录。
//
// 场景：父进程被 SIGKILL/崩溃时，chromedp 的 cancel 链路失效，
// 子浏览器进程变孤儿、user-data-dir 临时目录永久残留。
// 在应用启动阶段（创建实例池之前）调用一次即可回收上次运行的遗留；
// 本运行中活跃实例的目录 mtime 较新且进程有存活父链，
// 通过"仅清理 mtime 超过 1 小时的目录 + 启动期调用"双重条件避免误杀。
func SweepOrphans(logger JanitorLogger) (killedProcs, removedDirs int) {
	// 1) 杀掉孤儿浏览器进程（启动早期池尚未创建，此时存在的均为遗留）
	if pids, err := listChromedpProcesses(); err == nil && len(pids) > 0 {
		for _, pid := range pids {
			if err := syscallKill(pid); err != nil {
				logger.Warn("janitor: failed to kill orphaned chromium pid=%d: %v", pid, err)
				continue
			}
			killedProcs++
		}
		if killedProcs > 0 {
			logger.Warn("sweeping orphaned headless browsers from previous runs: %d killed", killedProcs)
		}
	}

	// 2) 清理超过 1 小时的 chromedp 临时目录
	tmpDir := os.TempDir()
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return killedProcs, removedDirs
	}
	cutoff := time.Now().Add(-time.Hour)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), chromedpMarker) || !e.IsDir() {
			continue
		}
		full := filepath.Join(tmpDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue // 可能属于本运行或刚结束的运行，保留
		}
		if err := os.RemoveAll(full); err == nil {
			removedDirs++
		}
	}
	if removedDirs > 0 {
		logger.Info("janitor: removed %d stale chromedp temp directories", removedDirs)
	}
	return killedProcs, removedDirs
}

// syscallKill 平台相关的进程终止
func syscallKill(pid int) error {
	// 先 SIGTERM 给浏览器优雅退出机会，短等待后 SIGKILL 兜底
	if err := exec.Command("kill", "-15", strconv.Itoa(pid)).Run(); err != nil {
		// 进程可能已退出，忽略
		return nil
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func processAlive(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid=").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}
