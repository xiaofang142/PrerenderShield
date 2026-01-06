package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"

	"github.com/go-redis/redis/v8"
)

type LogProcessor struct {
	crawlerLogMgr *logging.CrawlerLogManager
	visitLogMgr   *logging.VisitLogManager
	geoIP         *GeoIPService
	configMgr     *config.ConfigManager
	redisClient   *redis.Client
	ctx           context.Context
}

func NewLogProcessor(
	crawlerLogMgr *logging.CrawlerLogManager,
	visitLogMgr *logging.VisitLogManager,
	geoIP *GeoIPService,
	configMgr *config.ConfigManager,
	redisClient *redis.Client,
) *LogProcessor {
	return &LogProcessor{
		crawlerLogMgr: crawlerLogMgr,
		visitLogMgr:   visitLogMgr,
		geoIP:         geoIP,
		configMgr:     configMgr,
		redisClient:   redisClient,
		ctx:           context.Background(),
	}
}

func (p *LogProcessor) Start() {
	go func() {
		for {
			p.processCrawlerLogs()
			p.processVisitLogs()
			time.Sleep(5 * time.Second)
		}
	}()
}

func (p *LogProcessor) processCrawlerLogs() {
	logs, err := p.crawlerLogMgr.GetUnwashedLogs(10)
	if err != nil || len(logs) == 0 {
		return
	}

	logsByIP := make(map[string][]*logging.CrawlerLog)
	for i := range logs {
		logsByIP[logs[i].IP] = append(logsByIP[logs[i].IP], &logs[i])
	}

	for ip, ipLogs := range logsByIP {
		location, err := p.geoIP.GetLocation(ip)
		if err != nil {
			logging.DefaultLogger.Warn("GeoIP failed for %s: %v", ip, err)
		}

		for _, logEntry := range ipLogs {
			oldLog := *logEntry
			if location != nil {
				logEntry.Country = location.Country
				logEntry.CountryCode = location.CountryCode
				logEntry.City = location.City
				logEntry.Latitude = location.Latitude
				logEntry.Longitude = location.Longitude
			}
			logEntry.Washed = true

			if err := p.crawlerLogMgr.UpdateLog(oldLog, *logEntry); err != nil {
				logging.DefaultLogger.Error("Failed to update crawler log: %v", err)
			}

			if location != nil {
				p.checkAndBan(logEntry.Site, ip, location.CountryCode)
			}
		}
	}
}

func (p *LogProcessor) processVisitLogs() {
	logs, err := p.visitLogMgr.GetUnwashedLogs(10)
	if err != nil || len(logs) == 0 {
		return
	}

	logsByIP := make(map[string][]*logging.VisitLog)
	for i := range logs {
		logsByIP[logs[i].IP] = append(logsByIP[logs[i].IP], &logs[i])
	}

	for ip, ipLogs := range logsByIP {
		location, err := p.geoIP.GetLocation(ip)
		if err != nil {
			logging.DefaultLogger.Warn("GeoIP failed for %s: %v", ip, err)
		}

		for _, logEntry := range ipLogs {
			oldLog := *logEntry
			if location != nil {
				logEntry.Country = location.Country
				logEntry.CountryCode = location.CountryCode
				logEntry.City = location.City
				logEntry.Latitude = location.Latitude
				logEntry.Longitude = location.Longitude
			}
			logEntry.Washed = true

			if err := p.visitLogMgr.UpdateLog(oldLog, *logEntry); err != nil {
				logging.DefaultLogger.Error("Failed to update visit log: %v", err)
			}

			if location != nil {
				p.checkAndBan(logEntry.Site, ip, location.CountryCode)
			}
		}
	}
}

func (p *LogProcessor) checkAndBan(siteID, ip, countryCode string) {
	cfg := p.configMgr.GetConfig()
	var siteConfig *config.SiteConfig
	for _, s := range cfg.Sites {
		if s.ID == siteID {
			siteConfig = &s
			break
		}
	}

	if siteConfig == nil {
		return
	}

	for _, blocked := range siteConfig.Firewall.GeoIPConfig.BlockList {
		if strings.EqualFold(blocked, countryCode) {
			p.addToBlacklist(siteID, ip)
			return
		}
	}
}

func (p *LogProcessor) addToBlacklist(siteID, ip string) {
	key := fmt.Sprintf("firewall:%s:blacklist", siteID)
	p.redisClient.SAdd(p.ctx, key, ip)
	logging.DefaultLogger.Info("Banned IP %s for site %s due to GeoIP rule", ip, siteID)
}
