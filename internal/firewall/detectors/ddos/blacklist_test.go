package ddos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestBlacklist_New 测试创建黑名单
func TestBlacklist_New(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)
	assert.NotNil(t, bl)
	assert.NotNil(t, bl.blockedIPs)
	assert.Equal(t, 10*time.Minute, bl.blockDuration)
}

// TestBlacklist_New_InvalidDuration 测试无效持续时间
func TestBlacklist_New_InvalidDuration(t *testing.T) {
	bl := NewBlacklist(0)
	assert.NotNil(t, bl)
	assert.Equal(t, 10*time.Minute, bl.blockDuration)
}

// TestBlacklist_Add 测试添加 IP
func TestBlacklist_Add(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.1", "test reason")

	// 验证已添加
	assert.True(t, bl.IsBlacklisted("192.168.1.1"))

	entry := bl.GetEntry("192.168.1.1")
	assert.NotNil(t, entry)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.Equal(t, "test reason", entry.Reason)
	assert.Equal(t, 1, entry.Hits)
}

// TestBlacklist_Add_UpdateExisting 测试更新已存在的 IP
func TestBlacklist_Add_UpdateExisting(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.2", "reason 1")
	bl.Add("192.168.1.2", "reason 2")

	entry := bl.GetEntry("192.168.1.2")
	assert.NotNil(t, entry)
	assert.Equal(t, 2, entry.Hits)
	assert.Equal(t, "reason 2", entry.Reason)
}

// TestBlacklist_AddWithDuration 测试自定义持续时间添加
func TestBlacklist_AddWithDuration(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.AddWithDuration("192.168.1.3", "custom duration", 5*time.Minute)

	entry := bl.GetEntry("192.168.1.3")
	assert.NotNil(t, entry)
	assert.Equal(t, "custom duration", entry.Reason)
}

// TestBlacklist_Remove 测试移除 IP
func TestBlacklist_Remove(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.4", "test")

	// 移除
	removed := bl.Remove("192.168.1.4")
	assert.True(t, removed)

	// 验证已移除
	assert.False(t, bl.IsBlacklisted("192.168.1.4"))
}

// TestBlacklist_Remove_NotExists 测试移除不存在的 IP
func TestBlacklist_Remove_NotExists(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	removed := bl.Remove("192.168.1.5")
	assert.False(t, removed)
}

// TestBlacklist_IsBlacklisted 测试检查是否在黑名单
func TestBlacklist_IsBlacklisted(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.6", "test")

	assert.True(t, bl.IsBlacklisted("192.168.1.6"))
	assert.False(t, bl.IsBlacklisted("192.168.1.7"))
}

// TestBlacklist_IsBlacklistedWithReason 测试检查并获取原因
func TestBlacklist_IsBlacklistedWithReason(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.8", "specific reason")

	blocked, reason := bl.IsBlacklistedWithReason("192.168.1.8")
	assert.True(t, blocked)
	assert.Equal(t, "specific reason", reason)

	// 不存在的 IP
	blocked2, reason2 := bl.IsBlacklistedWithReason("192.168.1.9")
	assert.False(t, blocked2)
	assert.Equal(t, "", reason2)
}

// TestBlacklist_IsBlacklisted_Expired 测试过期自动移除
func TestBlacklist_IsBlacklisted_Expired(t *testing.T) {
	bl := NewBlacklist(50 * time.Millisecond)

	bl.Add("192.168.1.10", "test")

	// 验证已添加
	assert.True(t, bl.IsBlacklisted("192.168.1.10"))

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 应该返回 false（已过期）
	assert.False(t, bl.IsBlacklisted("192.168.1.10"))
}

// TestBlacklist_GetEntry 测试获取条目
func TestBlacklist_GetEntry(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.11", "test")

	entry := bl.GetEntry("192.168.1.11")
	assert.NotNil(t, entry)
	assert.Equal(t, "192.168.1.11", entry.IP)
}

// TestBlacklist_CleanupExpired 测试清理过期条目
func TestBlacklist_CleanupExpired(t *testing.T) {
	bl := NewBlacklist(50 * time.Millisecond)

	bl.Add("192.168.1.12", "test")

	// 验证已添加
	assert.Equal(t, 1, bl.GetCount())

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 清理
	bl.CleanupExpired()

	// 验证已清理
	assert.Equal(t, 0, bl.GetCount())
}

// TestBlacklist_GetCount 测试获取数量
func TestBlacklist_GetCount(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.13", "test1")
	bl.Add("192.168.1.14", "test2")

	assert.Equal(t, 2, bl.GetCount())
}

// TestBlacklist_GetAll 测试获取所有条目
func TestBlacklist_GetAll(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.15", "test1")
	bl.Add("192.168.1.16", "test2")

	entries := bl.GetAll()
	assert.GreaterOrEqual(t, len(entries), 2)
}

// TestBlacklist_GetActiveCount 测试获取活跃数量
func TestBlacklist_GetActiveCount(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.17", "test")

	count := bl.GetActiveCount()
	assert.GreaterOrEqual(t, count, 1)
}

// TestBlacklist_Clear 测试清空
func TestBlacklist_Clear(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.18", "test")
	assert.Equal(t, 1, bl.GetCount())

	bl.Clear()
	assert.Equal(t, 0, bl.GetCount())
}

// TestBlacklist_GetStats 测试获取统计
func TestBlacklist_GetStats(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.19", "test")

	stats := bl.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.Total, 1)
	assert.Equal(t, 10*time.Minute, stats.Duration)
}

// TestBlacklist_Block_Unblock 测试 Block/Unblock 方法
func TestBlacklist_Block_Unblock(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Block("192.168.1.20", 5*time.Minute)
	assert.True(t, bl.IsBlocked("192.168.1.20"))

	unblocked := bl.Unblock("192.168.1.20")
	assert.True(t, unblocked)
	assert.False(t, bl.IsBlocked("192.168.1.20"))
}

// TestBlacklist_IncrementHits_GetHits 测试增加/获取命中次数
func TestBlacklist_IncrementHits_GetHits(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.21", "test")

	assert.Equal(t, 1, bl.GetHits("192.168.1.21"))

	bl.IncrementHits("192.168.1.21")
	assert.Equal(t, 2, bl.GetHits("192.168.1.21"))

	// 不存在的 IP
	assert.Equal(t, 0, bl.GetHits("192.168.1.100"))
}

// TestBlacklist_ExtendBlock 测试延长封禁
func TestBlacklist_ExtendBlock(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.22", "test")

	expiresAt := bl.GetExpiresAt("192.168.1.22")

	bl.ExtendBlock("192.168.1.22", 5*time.Minute)

	newExpiresAt := bl.GetExpiresAt("192.168.1.22")
	assert.True(t, newExpiresAt.After(expiresAt))
}

// TestBlacklist_GetBlockReason 测试获取封禁原因
func TestBlacklist_GetBlockReason(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.23", "specific reason")

	reason := bl.GetBlockReason("192.168.1.23")
	assert.Equal(t, "specific reason", reason)

	// 不存在的 IP
	reason2 := bl.GetBlockReason("192.168.1.100")
	assert.Equal(t, "", reason2)
}

// TestBlacklist_GetBlockedAt 测试获取封禁时间
func TestBlacklist_GetBlockedAt(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	before := time.Now()
	bl.Add("192.168.1.24", "test")
	after := time.Now()

	blockedAt := bl.GetBlockedAt("192.168.1.24")
	assert.WithinDuration(t, before, blockedAt, time.Second)
	assert.WithinDuration(t, after, blockedAt, time.Second)
}

// TestBlacklist_GetExpiresAt 测试获取过期时间
func TestBlacklist_GetExpiresAt(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.25", "test")

	expiresAt := bl.GetExpiresAt("192.168.1.25")
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), expiresAt, time.Second)
}

// TestBlacklist_GetRemainingTime 测试获取剩余时间
func TestBlacklist_GetRemainingTime(t *testing.T) {
	bl := NewBlacklist(10 * time.Minute)

	bl.Add("192.168.1.26", "test")

	remaining := bl.GetRemainingTime("192.168.1.26")
	assert.Greater(t, remaining, time.Duration(0))
	assert.Less(t, remaining, 10*time.Minute+time.Second)

	// 不存在的 IP
	remaining2 := bl.GetRemainingTime("192.168.1.100")
	assert.Equal(t, time.Duration(0), remaining2)
}

// TestBlacklistEntry 测试 BlacklistEntry 结构
func TestBlacklistEntry(t *testing.T) {
	entry := &BlacklistEntry{
		IP:        "192.168.1.1",
		Reason:    "test reason",
		BlockedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
		Hits:      5,
	}

	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.Equal(t, "test reason", entry.Reason)
	assert.Equal(t, 5, entry.Hits)
}

// TestBlacklistStats 测试 BlacklistStats 结构
func TestBlacklistStats(t *testing.T) {
	stats := &BlacklistStats{
		Total:    100,
		Active:   80,
		Expired:  20,
		Duration: 10 * time.Minute,
	}

	assert.Equal(t, 100, stats.Total)
	assert.Equal(t, 80, stats.Active)
	assert.Equal(t, 20, stats.Expired)
	assert.Equal(t, 10*time.Minute, stats.Duration)
}
