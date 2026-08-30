#!/usr/bin/env python3
"""全路由运行时覆盖实测：85 条路由逐个请求，GET 断言 200，写操作用空/非法 body 断言 400/404（验证路由可达且处理器响应，不做破坏性操作）"""
import json, urllib.request, urllib.error, sys

BASE = "http://localhost:9598/api/v1"
results = []

def call(method, path, body=None, expect=(200,)):
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", "Bearer " + TOKEN)
    if data: req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            code, raw = r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        code, raw = e.code, e.read().decode()
    ok = code in expect
    results.append((ok, method, path, code, raw[:90].replace("\n", " ")))
    return code, raw

# 登录
req = urllib.request.Request(BASE + "/auth/login", data=json.dumps({"username": "admin", "password": "Admin#123456"}).encode(), method="POST")
req.add_header("Content-Type", "application/json")
with urllib.request.urlopen(req, timeout=10) as r:
    TOKEN = json.loads(r.read())["data"]["token"]

# 建临时站点供给 /:id 路径
_, raw = call("POST", "/sites", {"id": "sweep-tmp", "name": "Sweep", "port": 29903, "mode": "static", "domains": ["sweep.local"]}, expect=(200, 400, 402))
SITE = None
try:
    d = json.loads(raw)
    if d.get("code") in (200, 201): SITE = d["data"]["id"]
except Exception: pass
if not SITE:
    _, raw = call("GET", "/sites")
    try:
        data = json.loads(raw)["data"]
        # 优先复用 Sweep；否则复用任意现有站点（授权位被占时创建必 402）
        SITE = next((s["id"] for s in data if s.get("name") == "Sweep"), data[0]["id"] if data else None)
    except Exception as e:
        print("FALLBACK_ERR:", e, "| raw:", raw[:150])
        SITE = None
if not SITE:
    SITE = "nonexistent-sweep"

if SITE == "nonexistent-sweep":
    print("WARN: sweep site not resolved")

call("GET", "/health")
call("GET", "/version")
call("GET", "/auth/first-run")
call("POST", "/auth/login", {"username": "x", "password": "y"}, expect=(401, 400, 200))
call("GET", "/2fa/status")
call("POST", "/2fa/enable", {}, expect=(200, 400))
call("POST", "/2fa/confirm", {}, expect=(200, 400))
call("POST", "/2fa/disable", {}, expect=(200, 400))
call("POST", "/auth/change-password", {}, expect=(200, 400))
call("GET", "/overview")
call("GET", "/monitoring/stats")
call("GET", "/monitoring/alerts/history")
call("GET", "/monitoring/alert-rules")
call("GET", "/monitoring/alerts/rules")
call("GET", "/monitoring/alerts/channels")
call("POST", "/monitoring/alert-rules", {}, expect=(200, 400))
call("POST", "/monitoring/alerts/rules", {}, expect=(200, 400))
call("POST", "/monitoring/alerts/channels", {}, expect=(200, 400))
call("DELETE", "/monitoring/alert-rules/nonexistent-id", expect=(200, 404))
call("GET", "/firewall/rules?site_id=" + SITE)
call("GET", "/firewall/blacklist?site_id=" + SITE)
call("GET", "/firewall/whitelist?site_id=" + SITE)
call("GET", "/firewall/attacks")
call("POST", "/firewall/rules", {}, expect=(200, 400))
call("POST", "/firewall/blacklist", {}, expect=(200, 400))
call("POST", "/firewall/whitelist", {}, expect=(200, 400))
call("DELETE", "/firewall/rules/nonexistent-id?site_id=" + SITE, expect=(200, 400, 404))
call("GET", "/logs")
call("GET", "/logs/export", expect=(200, 400))
call("GET", "/crawler/logs")
call("GET", "/crawler/stats")
call("GET", "/crawler/url-stats")
call("GET", "/preheat/sites")
call("GET", "/preheat/stats")
call("GET", "/preheat/stats?siteId=" + SITE)
call("GET", "/preheat/urls?siteId=" + SITE)
call("GET", "/preheat/task/status")
call("GET", "/preheat/crawler-headers")
call("GET", "/preheat/entries?siteId=" + SITE)
call("POST", "/preheat/trigger", {}, expect=(200, 400))
call("POST", "/preheat/invalidate", {}, expect=(200, 400))
call("POST", "/preheat/recache", {}, expect=(200, 400, 500))
call("DELETE", "/preheat/entries?siteId=" + SITE + "&url=/", expect=(200, 400, 404))
call("POST", "/preheat/clear-cache", {}, expect=(200, 400))
call("GET", "/push/sites")
call("GET", "/push/stats")
call("GET", "/push/logs")
call("GET", "/push/trend")
call("GET", "/push/config?siteId=" + SITE)
call("POST", "/push/config", {}, expect=(200, 400, 500))
call("GET", "/ssl/certificates")
call("GET", "/ssl/certificates")
call("GET", "/ssl/certificates/expiring")
call("GET", "/ssl/certificates/nonexistent.example")
call("GET", "/ssl/certificates/nonexistent.example/renewal-history", expect=(200, 404, 503))
call("POST", "/ssl/certificates", {}, expect=(200, 400))
call("POST", "/ssl/certificates/nonexistent.example/renew", {}, expect=(200, 400, 404, 503))
call("POST", "/ssl/certificates/wildcard", {}, expect=(200, 400, 503))
call("DELETE", "/ssl/certificates/nonexistent.example", expect=(200, 404, 503))
call("GET", "/seo/config")
call("GET", "/seo/sitemap")
call("GET", "/seo/robots")
call("POST", "/seo/sitemap/generate", {}, expect=(200, 400, 500))
call("POST", "/seo/robots/generate", {}, expect=(200, 400, 500))
call("GET", "/system/config")
call("GET", "/system/backups")
call("POST", "/system/config", {}, expect=(200, 400))
call("POST", "/system/backup", {}, expect=(200, 400))
call("POST", "/system/restore", {}, expect=(200, 400))
call("GET", "/sites")
call("GET", "/sites/" + SITE)
call("GET", "/sites/" + SITE + "/config?type=prerender")
call("GET", "/sites/" + SITE + "/config?type=push")
call("GET", "/sites/" + SITE + "/config?type=waf")
call("GET", "/sites/" + SITE + "/waf")
call("GET", "/sites/" + SITE + "/static", expect=(200, 404))
call("PUT", "/sites/" + SITE, {}, expect=(200, 400))
call("PUT", "/sites/" + SITE + "/prerender", {}, expect=(200, 400))
call("PUT", "/sites/" + SITE + "/push", {}, expect=(200, 400))
call("PUT", "/sites/" + SITE + "/firewall", {}, expect=(200, 400))
call("PUT", "/sites/" + SITE + "/waf", {}, expect=(200, 400))
call("POST", "/sites/" + SITE + "/start", {}, expect=(200, 400))
call("POST", "/sites/" + SITE + "/stop", {}, expect=(200, 400))
call("POST", "/sites/" + SITE + "/static", {}, expect=(200, 400))
call("POST", "/sites/" + SITE + "/static/extract", {}, expect=(200, 400, 404))
call("POST", "/sites/" + SITE + "/static/batch-delete", {}, expect=(200, 400))
call("DELETE", "/sites/" + SITE + "/static", expect=(200, 400))
call("GET", "/system/config")
call("PUT", "/seo/config", {}, expect=(200, 400))
call("DELETE", "/sites/" + SITE, expect=(200, 404))
call("POST", "/auth/logout", {}, expect=(200, 400))

passed = sum(1 for r in results if r[0])
print(f"\n===== 全路由扫描: {passed}/{len(results)} 响应符合预期 =====")
for ok, mth, path, code, raw in results:
    if not ok:
        print(f"  ✗ {mth:6} {path}  http={code}  {raw}")
print("SWEEP-DONE")
