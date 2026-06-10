package firewall

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"prerender-shield/internal/firewall/detectors"
	"prerender-shield/internal/firewall/detectors/ai"
	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// DetectorManager 检测器管理器
type DetectorManager struct {
	mutex          sync.RWMutex
	owaspDetectors map[string]OWASPDetector
	coreDetectors  []CoreDetector
	failStrategy   FailStrategy
	logger         Logger
}

// NewDetectorManager 创建新的检测器管理器
func NewDetectorManager(failStrategy FailStrategy, logger Logger) *DetectorManager {
	return &DetectorManager{
		owaspDetectors: make(map[string]OWASPDetector),
		coreDetectors:  make([]CoreDetector, 0),
		failStrategy:   failStrategy,
		logger:         logger,
	}
}

// RegisterOWASPDetector 注册 OWASP 检测器
func (dm *DetectorManager) RegisterOWASPDetector(name string, detector OWASPDetector) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	dm.owaspDetectors[name] = detector
}

// RegisterCoreDetector 注册核心检测器
func (dm *DetectorManager) RegisterCoreDetector(detector CoreDetector) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	dm.coreDetectors = append(dm.coreDetectors, detector)
}

// Detect 执行所有检测器
func (dm *DetectorManager) Detect(req *http.Request) (*CheckResult, error) {
	threatsChan := make(chan []types.Threat, len(dm.owaspDetectors)+len(dm.coreDetectors))
	errChan := make(chan error, len(dm.owaspDetectors)+len(dm.coreDetectors))

	var wg sync.WaitGroup

	dm.mutex.RLock()
	owaspDetectors := make(map[string]OWASPDetector)
	for k, v := range dm.owaspDetectors {
		owaspDetectors[k] = v
	}
	coreDetectors := make([]CoreDetector, len(dm.coreDetectors))
	copy(coreDetectors, dm.coreDetectors)
	dm.mutex.RUnlock()

	for name, detector := range owaspDetectors {
		wg.Add(1)
		go func(det OWASPDetector, detectorName string) {
			defer wg.Done()
			threats, err := det.Detect(req)
			if err != nil {
				errChan <- fmt.Errorf("detector %s error: %w", detectorName, err)
				return
			}
			threatsChan <- threats
		}(detector, name)
	}

	for _, detector := range coreDetectors {
		wg.Add(1)
		go func(det CoreDetector) {
			defer wg.Done()
			threats, err := det.Detect(req)
			if err != nil {
				errChan <- fmt.Errorf("core detector %s error: %w", det.Name(), err)
				return
			}
			threatsChan <- threats
		}(detector)
	}

	go func() {
		wg.Wait()
		close(threatsChan)
		close(errChan)
	}()

	result := &CheckResult{
		Threats:   make([]types.Threat, 0),
		CreatedAt: time.Now(),
		Allow:     true,
	}

	for threats := range threatsChan {
		result.Threats = append(result.Threats, threats...)
	}

	var criticalErrors []error
	for err := range errChan {
		if dm.logger != nil {
			dm.logger.Error("Detector error: %s", err.Error())
		}
		criticalErrors = append(criticalErrors, err)
	}

	if len(criticalErrors) > 0 && dm.failStrategy == FailClosed {
		result.Allow = false
		result.Threats = append(result.Threats, types.Threat{
			Type:     "detector_error",
			SubType:  "Security Detector Failure",
			Severity: "critical",
			Message:  fmt.Sprintf("Security detector failed (%d errors), request blocked by fail-closed policy", len(criticalErrors)),
			RuleID:   "system-failclosed",
			RuleName: "Fail-Closed Policy",
		})
	}

	if len(result.Threats) > 0 {
		result.Allow = false
	}

	return result, nil
}

// SetupDefaultDetectors 设置默认检测器
func (dm *DetectorManager) SetupDefaultDetectors(ruleManager *RuleManager, redisClient *redis.Client, siteName string, config Config) {
	dm.RegisterOWASPDetector("injection", detectors.NewInjectionDetector(ruleManager))
	dm.RegisterOWASPDetector("xss", detectors.NewXSSDetector(ruleManager))
	dm.RegisterOWASPDetector("csrf", detectors.NewCSRFDetector(ruleManager))
	dm.RegisterOWASPDetector("deserialization", detectors.NewDeserializationDetector(ruleManager))
	dm.RegisterOWASPDetector("sensitive-data", detectors.NewSensitiveDataDetector(ruleManager))

	if config.GeoIPConfig != nil {
		dm.RegisterCoreDetector(detectors.NewGeoIPDetector(config.GeoIPConfig))
	}

	if config.RateLimitConfig != nil {
		dm.RegisterCoreDetector(detectors.NewRateLimitDetector(config.RateLimitConfig))
	}

	if config.FileIntegrityConfig != nil {
		dm.RegisterCoreDetector(detectors.NewFileIntegrityDetector(config.StaticDir, config.FileIntegrityConfig))
	}

	if redisClient != nil {
		dm.RegisterCoreDetector(detectors.NewBlacklistDetector(redisClient, siteName, config.Blacklist, config.Whitelist))
	} else {
		dm.RegisterCoreDetector(detectors.NewBlacklistDetector(nil, siteName, config.Blacklist, config.Whitelist))
	}

	if config.AIConfig != nil && config.AIConfig.Enabled {
		dm.setupAIDetector(config.AIConfig)
	}
}

func (dm *DetectorManager) setupAIDetector(aiConfig *AIEngineConfig) {
	config := &ai.Config{
		ModelPath:           aiConfig.ModelPath,
		WorkerPool:          aiConfig.WorkerPool,
		ConfidenceThreshold: aiConfig.ConfidenceThreshold,
		PredictTimeout:      time.Duration(aiConfig.TimeoutMs) * time.Millisecond,
		CacheSize:           aiConfig.CacheSize,
		Enabled:             true,
	}

	if config.WorkerPool <= 0 {
		config.WorkerPool = 4
	}
	if config.ConfidenceThreshold <= 0 {
		config.ConfidenceThreshold = 0.85
	}
	if config.PredictTimeout <= 0 {
		config.PredictTimeout = 50 * time.Millisecond
	}
	if config.CacheSize <= 0 {
		config.CacheSize = 10000
	}

	aiDetector, err := ai.NewAIDetector(config)
	if err != nil {
		logging.DefaultLogger.Info("AI detector initialization failed: %v\n", err)
	} else {
		dm.RegisterOWASPDetector("ai", aiDetector)
	}
}
