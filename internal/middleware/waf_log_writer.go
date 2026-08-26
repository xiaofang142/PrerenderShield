package middleware

import (
	"sync"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/models"
	"prerender-shield/internal/repository"
)

// WafLogWriter 异步批量写入 WAF 日志，兼顾性能和可靠性
type WafLogWriter struct {
	wafRepo       *repository.WafRepository
	logChan       chan models.AccessLog
	batchSize     int
	flushInterval time.Duration
	wg            sync.WaitGroup
	stopChan      chan struct{}
}

// NewWafLogWriter 创建批量日志写入器
func NewWafLogWriter(wafRepo *repository.WafRepository, batchSize int, flushInterval time.Duration) *WafLogWriter {
	if batchSize <= 0 {
		batchSize = 50
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	w := &WafLogWriter{
		wafRepo:       wafRepo,
		logChan:       make(chan models.AccessLog, 500),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}

	w.wg.Add(1)
	go w.processLoop()

	return w
}

// Write 异步写入一条日志（非阻塞，满了丢弃并记录警告）
func (w *WafLogWriter) Write(log models.AccessLog) {
	select {
	case w.logChan <- log:
	default:
		logging.DefaultLogger.Warn("[WAF] Log channel full, dropping log entry for site %s", log.SiteID)
	}
}

// Stop 停止写入器，刷写剩余日志
func (w *WafLogWriter) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}

func (w *WafLogWriter) processLoop() {
	defer w.wg.Done()

	batch := make([]models.AccessLog, 0, w.batchSize)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case log := <-w.logChan:
			batch = append(batch, log)
			if len(batch) >= w.batchSize {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-w.stopChan:
			// 刷写剩余日志
			for {
				select {
				case log := <-w.logChan:
					batch = append(batch, log)
				default:
					if len(batch) > 0 {
						w.flush(batch)
					}
					return
				}
			}
		}
	}
}

func (w *WafLogWriter) flush(batch []models.AccessLog) {
	if w.wafRepo == nil || len(batch) == 0 {
		return
	}
	for _, log := range batch {
		l := log
		if err := w.wafRepo.CreateAccessLog(&l); err != nil {
			logging.DefaultLogger.Error("[WAF] Failed to write access log: %v", err)
		}
	}
}
