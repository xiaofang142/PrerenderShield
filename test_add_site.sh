#!/bin/bash

# 新增站点
echo "新增站点..."
curl -X POST -H "Content-Type: application/json" -d '{"Name":"test-site","Domain":"127.0.0.1","Port":8083,"Proxy":{"Enabled":false,"TargetURL":"","Type":"direct"},"Firewall":{"Enabled":true,"RulesPath":"/etc/prerender-shield/rules","ActionConfig":{"DefaultAction":"block","BlockMessage":"Request blocked by firewall"}},"Prerender":{"Enabled":true,"PoolSize":5,"MinPoolSize":2,"MaxPoolSize":20,"Timeout":30,"CacheTTL":3600,"IdleTimeout":300,"DynamicScaling":true,"ScalingFactor":0.5,"ScalingInterval":60,"Preheat":{"Enabled":false,"SitemapURL":"","Schedule":"0 0 * * *","Concurrency":5,"DefaultPriority":0}},"Routing":{"Rules":[]},"SSL":{"Enabled":false,"LetEncrypt":false,"Domains":[],"ACMEEmail":"","ACMEServer":"https://acme-v02.api.letsencrypt.org/directory","ACMEChallenge":"http01","CertPath":"/etc/prerender-shield/certs/cert.pem","KeyPath":"/etc/prerender-shield/certs/key.pem"}}' http://localhost:8080/api/v1/sites/

# 等待1秒，让站点服务器启动
sleep 1

# 测试访问新增的站点
echo "测试访问新增的站点..."
curl -v http://localhost:8083
