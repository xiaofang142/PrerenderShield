package ssl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
)

// AutoRenewConfig 自动续签配置
type AutoRenewConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	CheckInterval   time.Duration `yaml:"check_interval" json:"check_interval"`
	RenewBeforeDays int           `yaml:"renew_before_days" json:"renew_before_days"`
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`
	WebhookURL      string        `yaml:"webhook_url" json:"webhook_url"`
}

// AutoRenewer 自动续签器
type AutoRenewer struct {
	acmeClient  *ACMEClient
	redisClient *redis.Client
	config      AutoRenewConfig
	cancel      context.CancelFunc
	ctx         context.Context
}

// NewAutoRenewer 创建自动续签器
func NewAutoRenewer(acmeClient *ACMEClient, redisClient *redis.Client, config AutoRenewConfig) *AutoRenewer {
	return &AutoRenewer{
		acmeClient:  acmeClient,
		redisClient: redisClient,
		config:      config,
	}
}

// Start 启动自动续签
func (r *AutoRenewer) Start() {
	if !r.config.Enabled {
		logging.DefaultLogger.Info("Auto renewal is disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel

	go r.run()
	logging.DefaultLogger.Info("Auto renewal started (check interval: %v)", r.config.CheckInterval)
}

// Stop 停止自动续签
func (r *AutoRenewer) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	logging.DefaultLogger.Info("Auto renewal stopped")
}

func (r *AutoRenewer) run() {
	ticker := time.NewTicker(r.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAndRenew()
		}
	}
}

func (r *AutoRenewer) checkAndRenew() {
	if r.acmeClient == nil {
		logging.DefaultLogger.Warn("Auto-renewal skipped: ACME client not initialized")
		return
	}

	logging.DefaultLogger.Info("Running scheduled certificate renewal check")

	certs, err := r.acmeClient.ListCertificates()
	if err != nil {
		logging.DefaultLogger.Error("Failed to list certificates: %v", err)
		return
	}

	for _, cert := range certs {
		domain, ok := cert["domain"].(string)
		if !ok {
			continue
		}

		expiresIn, ok := cert["expires_in"].(int)
		if !ok {
			continue
		}

		// 检查是否需要续签
		if expiresIn <= r.config.RenewBeforeDays && expiresIn > 0 {
			logging.DefaultLogger.Info("Certificate %s expires in %d days, starting renewal", domain, expiresIn)
			r.renewCertificate(domain)
		} else if expiresIn <= 0 {
			logging.DefaultLogger.Warn("Certificate %s has expired, attempting emergency renewal", domain)
			r.renewCertificate(domain)
		}
	}
}

func (r *AutoRenewer) renewCertificate(domain string) {
	var err error

	// 尝试续签（带重试）
	for attempt := 0; attempt < r.config.MaxRetries; attempt++ {
		_, err = r.acmeClient.RenewCertificate(domain)
		if err == nil {
			logging.DefaultLogger.Info("Certificate %s renewed successfully", domain)

			// 保存续签记录到 Redis
			r.saveRenewalRecord(domain, "success", "")

			// 发送 webhook 通知
			if r.config.WebhookURL != "" {
				sendWebhook(r.config.WebhookURL, "renewal_success", domain)
			}
			return
		}

		logging.DefaultLogger.Warn("Renewal attempt %d failed for %s: %v", attempt+1, domain, err)
		time.Sleep(r.config.RetryDelay)
	}

	// 所有重试失败
	logging.DefaultLogger.Error("Failed to renew certificate %s after %d attempts: %v", domain, r.config.MaxRetries, err)

	// 保存失败记录到 Redis
	r.saveRenewalRecord(domain, "failed", err.Error())

	// 发送失败 webhook 通知
	if r.config.WebhookURL != "" {
		sendWebhook(r.config.WebhookURL, "renewal_failed", domain)
	}
}

func (r *AutoRenewer) saveRenewalRecord(domain, status, errorMsg string) {
	key := fmt.Sprintf("ssl:renewal:%s", domain)
	record := map[string]interface{}{
		"domain":     domain,
		"status":     status,
		"error":      errorMsg,
		"timestamp":  time.Now().Unix(),
		"retry_time": time.Now().Add(24 * time.Hour).Unix(),
	}

	if r.redisClient != nil {
		r.redisClient.SaveJSON(key, record, 7*24*time.Hour)
	}
}

func sendWebhook(url, event, domain string) {
	payload := map[string]interface{}{
		"event":     event,
		"domain":    domain,
		"timestamp": time.Now().Unix(),
	}

	logging.DefaultLogger.Info("Sending webhook notification: %s - %s", event, domain)

	body, err := json.Marshal(payload)
	if err != nil {
		logging.DefaultLogger.Error("Failed to marshal webhook payload: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logging.DefaultLogger.Warn("Webhook request failed for %s: %v", event, err)
		return
	}
	resp.Body.Close()
	logging.DefaultLogger.Info("Webhook sent successfully: %s - %s (status: %d)", event, domain, resp.StatusCode)
}

// GetRenewalHistory 获取续签历史
func (r *AutoRenewer) GetRenewalHistory(domain string) ([]map[string]interface{}, error) {
	if r.redisClient == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	key := fmt.Sprintf("ssl:renewal:%s", domain)
	var record map[string]interface{}
	err := r.redisClient.GetJSON(key, &record)
	if err != nil {
		return nil, err
	}

	return []map[string]interface{}{record}, nil
}
