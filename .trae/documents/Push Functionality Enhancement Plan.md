# Push Functionality Enhancement Plan

## Overview
This plan outlines the changes needed to enhance the push functionality according to the requirements, including changing from cron scheduling to simple hour selection, implementing automatic push scheduling, updating push stats, and adding a push trend chart.

## Changes to Implement

### 1. Update PushConfig Structure
- **File**: `internal/config/config.go`
- **Change**: Replace `Schedule` string field with `Hour` int field (0-24)
- **Reason**: Simplify user experience by using a simple hour selection instead of cron syntax

### 2. Enable Push Task Scheduling
- **File**: `internal/scheduler/scheduler.go`
- **Changes**:
  - Uncomment and update push task scheduling code
  - Update `createTask` to use the new `Hour` field instead of cron syntax
  - Implement `executePush` method to call `pushManager.TriggerPush`
  - Add push manager initialization to the scheduler
- **Reason**: Enable automatic push scheduling based on the selected hour

### 3. Update PushManager Implementation
- **File**: `internal/prerender/push/manager.go`
- **Changes**:
  - Update `logPushResult` to include only required fields
  - Ensure push logic correctly handles daily limits with offset
  - Add logic to track daily push counts for trend chart
- **Reason**: Implement proper push task orchestration and data tracking

### 4. Update Redis Client for New Metrics
- **File**: `internal/redis/client.go`
- **Changes**:
  - Add methods to track total, pushed, and unpushed URLs
  - Add methods to get daily push counts for trend chart
  - Add expiration for push logs after 30 days
- **Reason**: Support new push statistics and trend chart data

### 5. Update API Handlers
- **File**: `cmd/api/handlers/push.go`
- **Changes**:
  - Remove `TriggerPush` endpoint (manual push)
  - Update `GetPushStats` to return total, pushed, unpushed counts
  - Add new endpoint `GetPushTrend` for last 15 days data
- **Reason**: Remove manual push functionality and support new statistics

### 6. Update Main Application
- **File**: `cmd/api/main.go`
- **Changes**:
  - Initialize push manager and pass to scheduler
  - Update scheduler initialization to include push manager
- **Reason**: Integrate push manager with scheduler

## Implementation Steps

1. **Update PushConfig struct** in config.go
2. **Update Redis client** to support new metrics and expiration
3. **Update PushManager** to track daily push counts
4. **Update Scheduler** to enable push task scheduling
5. **Update API handlers** to remove manual push and add trend data
6. **Update main.go** to integrate push manager with scheduler
7. **Test the implementation** to ensure all features work correctly

## Expected Outcomes

- Users can select a daily push hour (0-24) instead of using cron syntax
- Automatic push scheduling runs based on the selected hour
- Push stats include total, pushed, and unpushed URL counts
- Push trend chart shows last 15 days of push activity
- Push logs are automatically cleaned up after 30 days
- Manual push functionality is removed
- Push task orchestration handles URLs exceeding daily limits by splitting across days