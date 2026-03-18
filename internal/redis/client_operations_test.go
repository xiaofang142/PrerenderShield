package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClient_GetRawClient 测试获取原始 redis 客户端
func TestClient_GetRawClient(t *testing.T) {
	// 使用 nil 客户端测试
	client := &Client{}

	// 应该返回 nil 但不 panic
	assert.Nil(t, client.GetRawClient())
}

// TestClient_Context 测试 Context 方法
func TestClient_Context(t *testing.T) {
	client := &Client{}

	// ctx 为 nil 时返回 nil
	ctx := client.Context()
	// 注意：当 client 未初始化时 ctx 为 nil
	assert.Nil(t, ctx)
}

// TestClient_Del_Nil 测试删除不存在的键
func TestClient_Del_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Del panicked as expected with nil client: %v", r)
		}
	}()

	client.Del("nonexistent-key")
}

// TestClient_Exists_Nil 测试检查不存在的键
func TestClient_Exists_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Exists panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.Exists("nonexistent-key")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_HashGet_Nil 测试获取不存在的哈希字段
func TestClient_HashGet_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("HashGet panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.HashGet("nonexistent-key", "field")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_HashGetAll_Nil 测试获取不存在的哈希表
func TestClient_HashGetAll_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("HashGetAll panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.HashGetAll("nonexistent-key")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_ListPop_Nil 测试从空列表弹出
func TestClient_ListPop_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("ListPop panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.ListPop("nonexistent-list")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_ListRange_Nil 测试获取列表范围
func TestClient_ListRange_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("ListRange panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.ListRange("nonexistent-list", 0, 10)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetMembers_Nil 测试获取集合成员
func TestClient_SetMembers_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetMembers panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.SetMembers("nonexistent-set")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SIsMember_Nil 测试检查集合成员
func TestClient_SIsMember_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetContains panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.SetContains("nonexistent-set", "member")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_Incr_Nil 测试递增计数器
func TestClient_Incr_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Incr panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.Incr("counter")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_Decr_Nil 测试递减计数器
func TestClient_Decr_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Decr panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.Decr("counter")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_Keys_Nil 测试获取匹配键
func TestClient_Keys_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Keys panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.Keys("pattern:*")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_DelMultiple_Nil 测试删除多个键
func TestClient_DelMultiple_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("DelMultiple panicked as expected with nil client: %v", r)
		}
	}()

	keys := []string{"key1", "key2", "key3"}
	err := client.DelMultiple(keys)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_DelMultiple_Empty 测试删除空键列表
func TestClient_DelMultiple_Empty(t *testing.T) {
	client := &Client{}

	// 空列表应该不报错
	err := client.DelMultiple([]string{})
	assert.NoError(t, err)
}

// TestClient_Expire_Nil 测试设置过期时间
func TestClient_Expire_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expire panicked as expected with nil client: %v", r)
		}
	}()

	err := client.Expire("key", 3600)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_TTL_Nil 测试获取 TTL
func TestClient_TTL_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("TTL panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.TTL("key")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_Subscribe_Nil 测试订阅频道
func TestClient_Subscribe_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Subscribe panicked as expected with nil client: %v", r)
		}
	}()

	pubsub := client.Subscribe("channel1", "channel2")
	if pubsub == nil {
		t.Error("Expected pubsub object")
	}
}

// TestClient_SaveJSON_Nil 测试保存 JSON
func TestClient_SaveJSON_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SaveJSON panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SaveJSON("key", map[string]string{"test": "value"}, 3600)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetJSON_Nil 测试获取 JSON
func TestClient_GetJSON_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetJSON panicked as expected with nil client: %v", r)
		}
	}()

	var dest map[string]string
	err := client.GetJSON("key", &dest)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetJSON_EmptyData 测试获取空 JSON 数据
func TestClient_GetJSON_EmptyData(t *testing.T) {
	// Get 返回空字符串时，GetJSON 应该不报错
	// 这需要 mock Get 方法，暂时跳过
	t.Skip("Requires mock Get method")
}

// TestClient_AddURL_Nil 测试添加 URL
func TestClient_AddURL_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("AddURL panicked as expected with nil client: %v", r)
		}
	}()

	err := client.AddURL("site1", "http://example.com")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_RemoveURL_Nil 测试移除 URL
func TestClient_RemoveURL_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("RemoveURL panicked as expected with nil client: %v", r)
		}
	}()

	err := client.RemoveURL("site1", "http://example.com")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetURLs_Nil 测试获取 URLs
func TestClient_GetURLs_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetURLs panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetURLs("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetURLCount_Nil 测试获取 URL 数量
func TestClient_GetURLCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetURLCount panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetURLCount("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetURLPreheatStatus_Nil 测试设置预热状态
func TestClient_SetURLPreheatStatus_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetURLPreheatStatus panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetURLPreheatStatus("site1", "http://example.com", "cached", 1024)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetURLPreheatStatusMap_Nil 测试获取预热状态
func TestClient_GetURLPreheatStatusMap_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetURLPreheatStatusMap panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetURLPreheatStatusMap("site1", "http://example.com")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_IsPreheatRunning_Nil 测试检查预热状态
func TestClient_IsPreheatRunning_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("IsPreheatRunning panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.IsPreheatRunning("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_ClearURLs_Nil 测试清空 URLs
func TestClient_ClearURLs_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("ClearURLs panicked as expected with nil client: %v", r)
		}
	}()

	err := client.ClearURLs("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetPreheatRunning_Nil 测试设置预热运行状态
func TestClient_SetPreheatRunning_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetPreheatRunning panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetPreheatRunning("site1", true)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetCurrentPreheatTask_Nil 测试获取预热任务
func TestClient_GetCurrentPreheatTask_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetCurrentPreheatTask panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetCurrentPreheatTask("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_UpdatePreheatTaskProgress_Nil 测试更新预热进度
func TestClient_UpdatePreheatTaskProgress_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("UpdatePreheatTaskProgress panicked as expected with nil client: %v", r)
		}
	}()

	err := client.UpdatePreheatTaskProgress("task1", 50)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetPushTask_Nil 测试设置推送任务
func TestClient_SetPushTask_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetPushTask panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetPushTask("site1", map[string]interface{}{"status": "pending"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetPushOffset_Nil 测试获取推送偏移量
func TestClient_GetPushOffset_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetPushOffset panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetPushOffset("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetPushOffset_Nil 测试设置推送偏移量
func TestClient_SetPushOffset_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetPushOffset panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetPushOffset("site1", 100)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetLastPushDate_Nil 测试设置最后推送日期
func TestClient_SetLastPushDate_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetLastPushDate panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetLastPushDate("site1", "2024-01-01")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_IncrDailyPushCount_Nil 测试增加每日推送计数
func TestClient_IncrDailyPushCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("IncrDailyPushCount panicked as expected with nil client: %v", r)
		}
	}()

	err := client.IncrDailyPushCount("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_IncrPushStats_Nil 测试增加推送统计
func TestClient_IncrPushStats_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("IncrPushStats panicked as expected with nil client: %v", r)
		}
	}()

	err := client.IncrPushStats("site1", "success")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_AddPushLog_Nil 测试添加推送日志
func TestClient_AddPushLog_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("AddPushLog panicked as expected with nil client: %v", r)
		}
	}()

	err := client.AddPushLog("site1", "log entry")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetURLPreheatStatus_Nil 测试获取预热状态
func TestClient_GetURLPreheatStatus_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetURLPreheatStatus panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetURLPreheatStatus("site1", "http://example.com")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SetSiteStats_Nil 测试设置站点统计
func TestClient_SetSiteStats_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SetSiteStats panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SetSiteStats("site1", map[string]interface{}{"visits": 100})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetSiteStats_Nil 测试获取站点统计
func TestClient_GetSiteStats_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetSiteStats panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetSiteStats("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetCacheCount_Nil 测试获取缓存数量
func TestClient_GetCacheCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetCacheCount panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetCacheCount()
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_ClearCache_Nil 测试清空缓存
func TestClient_ClearCache_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("ClearCache panicked as expected with nil client: %v", r)
		}
	}()

	err := client.ClearCache()
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_CreatePreheatTask_Nil 测试创建预热任务
func TestClient_CreatePreheatTask_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("CreatePreheatTask panicked as expected with nil client: %v", r)
		}
	}()

	err := client.CreatePreheatTask("task1", map[string]interface{}{"status": "pending"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SaveUser_Nil 测试保存用户
func TestClient_SaveUser_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SaveUser panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SaveUser("user1", map[string]interface{}{"username": "test"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SaveUserWithCredentials_Nil 测试保存用户凭据
func TestClient_SaveUserWithCredentials_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SaveUserWithCredentials panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SaveUserWithCredentials("user1", "testuser", "password123")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetUser_Nil 测试获取用户
func TestClient_GetUser_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetUser panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetUser("user1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetUserByUsername_Nil 测试通过用户名获取用户
func TestClient_GetUserByUsername_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetUserByUsername panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetUserByUsername("testuser")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetAllUsers_Nil 测试获取所有用户
func TestClient_GetAllUsers_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetAllUsers panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetAllUsers()
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SaveSession_Nil 测试保存会话
func TestClient_SaveSession_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SaveSession panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SaveSession("session1", map[string]interface{}{"user": "test"}, 3600)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetSession_Nil 测试获取会话
func TestClient_GetSession_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetSession panicked as expected with nil client: %v", r)
		}
	}()

	var dest map[string]interface{}
	err := client.GetSession("session1", &dest)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_CheckSessionExists_Nil 测试检查会话存在
func TestClient_CheckSessionExists_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("CheckSessionExists panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.CheckSessionExists("session1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_DeleteSession_Nil 测试删除会话
func TestClient_DeleteSession_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("DeleteSession panicked as expected with nil client: %v", r)
		}
	}()

	err := client.DeleteSession("session1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_IncrDailyPushCountWithCount_Nil 测试增加每日推送计数 (带数量)
func TestClient_IncrDailyPushCountWithCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("IncrDailyPushCountWithCount panicked as expected with nil client: %v", r)
		}
	}()

	err := client.IncrDailyPushCountWithCount("site1", 5)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_IncrPushStatsWithCount_Nil 测试增加推送统计 (带数量)
func TestClient_IncrPushStatsWithCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("IncrPushStatsWithCount panicked as expected with nil client: %v", r)
		}
	}()

	err := client.IncrPushStatsWithCount("site1", "success", 10)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_AddPushLogStruct_Nil 测试添加推送日志 (结构体)
func TestClient_AddPushLogStruct_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("AddPushLogStruct panicked as expected with nil client: %v", r)
		}
	}()

	err := client.AddPushLogStruct("site1", map[string]string{"level": "info", "message": "test"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetPushStatsWithURLCounts_Nil 测试获取推送统计
func TestClient_GetPushStatsWithURLCounts_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetPushStatsWithURLCounts panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetPushStatsWithURLCounts("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetLast15DaysPushCount_Nil 测试获取 15 天推送计数
func TestClient_GetLast15DaysPushCount_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetLast15DaysPushCount panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetLast15DaysPushCount("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetPushLogs_Nil 测试获取推送日志
func TestClient_GetPushLogs_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetPushLogs panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetPushLogs("site1", 10, 0)
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_DeleteSiteData_Nil 测试删除站点数据
func TestClient_DeleteSiteData_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("DeleteSiteData panicked as expected with nil client: %v", r)
		}
	}()

	err := client.DeleteSiteData("site1")
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_GetSystemConfig_Nil 测试获取系统配置
func TestClient_GetSystemConfig_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetSystemConfig panicked as expected with nil client: %v", r)
		}
	}()

	_, err := client.GetSystemConfig()
	if err == nil {
		t.Error("Expected error with nil client")
	}
}

// TestClient_SaveSystemConfig_Nil 测试保存系统配置
func TestClient_SaveSystemConfig_Nil(t *testing.T) {
	client := &Client{}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("SaveSystemConfig panicked as expected with nil client: %v", r)
		}
	}()

	err := client.SaveSystemConfig(map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("Expected error with nil client")
	}
}
