package services

import (
	"fmt"

	"prerender-shield/internal/repository"
)

// OverviewService 概览业务服务。
// 封装概览页所需的数据访问，保证 Controller 不直接依赖 Repository（五层架构约束）。
type OverviewService struct {
	wafRepo *repository.WafRepository
}

// NewOverviewService 创建概览服务
func NewOverviewService(wafRepo *repository.WafRepository) *OverviewService {
	return &OverviewService{wafRepo: wafRepo}
}

// GetWafGlobalStats 获取指定时间范围内的全局 WAF 统计
func (s *OverviewService) GetWafGlobalStats(startTime, endTime string) (*repository.WafStats, error) {
	if s == nil || s.wafRepo == nil {
		return nil, fmt.Errorf("overview service: waf repository not configured")
	}
	return s.wafRepo.GetGlobalStats(startTime, endTime)
}
