# Preheating Configuration Optimization Plan

## Overview
This plan optimizes the preheating configuration by simplifying UI parameters and implementing an auto-preheating daemon that triggers based on cache expiration.

## Implementation Steps

### 1. Frontend Changes (`Sites.tsx`)
- **Simplify Preheating Configuration Form**: Only retain "Enable Auto Preheating" toggle
- **Remove Unnecessary UI Elements**: 
  - Preheating schedule rules
  - Concurrency setting
  - Default priority setting
  - Advanced configuration parameters (browser idle timeout, dynamic scaling, scaling factor, scaling interval)

### 2. Backend Changes

#### A. Update Configuration Structs (`engine.go`)
- **Modify `PreheatConfig`**: Remove `Schedule`, `Concurrency`, `DefaultPriority` fields
- **Modify `PrerenderConfig`**: Remove `IdleTimeout`, `DynamicScaling`, `ScalingFactor`, `ScalingInterval` fields

#### B. Implement Auto-Preheating Daemon (`engine.go`)
- **Add `autoPreheatTicker` to EngineManager**: Run every minute
- **Core Logic**:
  - Loop through all sites
  - For each site with auto-preheating enabled:
    - Get all URLs from Redis
    - Check cache expiration for each URL
    - If cache is within 10 minutes of expiration or already expired:
      - Trigger preheating for those URLs
      - Use existing `TriggerPreheatForURL` method
      - No URL re-extraction needed

#### C. Update API Handlers (`preheat.go`)
- **Remove Unsupported Parameters**: Update API handlers to ignore removed configuration fields
- **Ensure Backward Compatibility**: Handle old configuration formats gracefully

#### D. Redis Cache Expiration Check
- **Add Cache TTL Check**: Verify cache expiration status for each URL
- **Use Existing Cache Logic**: Leverage current `CacheTTL` setting to determine expiration

### 3. Core Preheating Logic (`preheat.go`)
- **Simplify `TriggerPreheat` Method**: Remove unnecessary parameter handling
- **Optimize URL Preheating**: Ensure efficient preheating of only expired/expiring URLs
- **Maintain Progress Tracking**: Keep existing progress tracking for auto-preheating tasks

## Expected Behavior
1. **Simplified UI**: Users only see "Enable Auto Preheating" toggle
2. **Auto-Preheating**: URLs are automatically re-preheated when cache is about to expire
3. **Efficient Resource Usage**: Only preheats URLs that actually need it
4. **No Manual Configuration**: Users don't need to set complex preheating schedules

## Testing Strategy
- **Frontend UI Test**: Verify simplified preheating configuration form
- **Backend Logic Test**: Test auto-preheating daemon triggers correctly based on cache expiration
- **API Compatibility Test**: Ensure old API requests still work
- **Performance Test**: Verify minimal resource usage for the daemon process

## Files to Modify
- `/web/src/pages/Sites/Sites.tsx`
- `/cmd/api/handlers/preheat.go`
- `/internal/prerender/preheat.go`
- `/internal/prerender/engine.go`
- `/internal/redis/client.go`