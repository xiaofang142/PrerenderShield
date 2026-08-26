#!/usr/bin/env python3
"""API 全端点验证脚本。

用法: 先手动启动服务(或由 CI 启动), 然后运行:
  python3 scripts/api_verify_full.py --base http://127.0.0.1:19598

流程:
  1. GET /auth/first-run 检查首次运行状态
  2. POST /auth/login 创建/登录管理员获取 JWT
  3. 遍历 docs/API.md 中全部端点, 校验 HTTP 状态与业务 code 字段
"""
import argparse
import json
import sys
import time
import urllib.error
import urllib.request

# 绕过系统代理(http_proxy等), 本地直连
urllib.request.install_opener(urllib.request.build_opener(urllib.request.ProxyHandler({})))

BASE = "http://127.0.0.1:9598/api/v1"
TOKEN = ""
RESULTS = []


def call(method, path, body=None, auth=True, expect_http=200, expect_code=200,
         accept_non_json=False):
    url = f"{BASE}{path}"
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if auth and TOKEN:
        req.add_header("Authorization", f"Bearer {TOKEN}")
    http_code, resp_code, note = 0, None, ""
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            http_code = r.status
            ctype = r.headers.get("Content-Type", "")
            raw = r.read().decode()
        if accept_non_json or "application/json" not in ctype:
            resp_code = None
            note = f"text({ctype.split(';')[0]},{len(raw)}b)"
        else:
            try:
                j = json.loads(raw)
                resp_code = j.get("code")
                note = str(j.get("message"))[:60]
            except json.JSONDecodeError:
                note = f"non-json({len(raw)}b)"
    except urllib.error.HTTPError as e:
        http_code = e.code
        try:
            j = json.loads(e.read().decode())
            resp_code = j.get("code")
            note = str(j.get("message"))[:60]
        except Exception:
            note = "http-error"
    except Exception as e:
        note = f"EXC:{e}"

    ok = http_code == expect_http and (
        expect_code is None or resp_code == expect_code
    )
    RESULTS.append((ok, method, path, http_code, resp_code, note))
    return ok


def main():
    global BASE, TOKEN
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default=BASE,
                    help="API 根地址，带或不带 /api/v1 前缀均可")
    args = ap.parse_args()
    base = args.base.rstrip("/")
    if not base.endswith("/api/v1"):
        base += "/api/v1"
    BASE = base

    # ── 公开端点 ──
    call("GET", "/health", auth=False)
    call("GET", "/version", auth=False)

    # ── 登录(首跑环境会自动建号) ──
    ok = False
    for attempt in range(3):
        ok = call("POST", "/auth/login",
                  {"username": "apitest", "password": "ApiTest#2026"},
                  auth=False, expect_code=None)
        idx = next(i for i, r in enumerate(RESULTS) if r[1] == "POST" and r[2] == "/auth/login")
        _, _, _, hc, rc, note = RESULTS[idx]
        if rc == 200 or hc == 200:
            break
        time.sleep(1)
    if not ok:
        print("FATAL: login failed, cannot continue authenticated tests")
        report()
        sys.exit(1)
    # 提取 token
    req = urllib.request.Request(f"{BASE}/auth/login",
                                 data=json.dumps({"username": "apitest", "password": "ApiTest#2026"}).encode(),
                                 method="POST")
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=10) as r:
        j = json.loads(r.read().decode())
    TOKEN = (j.get("data") or {}).get("token") or (j.get("data") or {}).get("access_token") or ""
    if not TOKEN:
        print("FATAL: no token in login response:", json.dumps(j)[:200])
        sys.exit(1)

    # ── 认证端点全量遍历 ──
    gets = [
        "/auth/first-run",
        "/system/config", "/system/backups",
        "/2fa/status",
        "/overview",
        "/monitoring/stats", "/monitoring/alerts/history",
        "/monitoring/alert-rules", "/monitoring/alerts/rules",
        "/monitoring/alerts/channels",
        "/firewall/rules?site_id=default",
        "/logs", "/firewall/attacks",
        "/firewall/blacklist?site_id=default", "/firewall/whitelist?site_id=default",
        "/crawler/logs", "/crawler/stats?startTime=2026-08-01&endTime=2026-08-26&granularity=day",
        "/seo/config",
        "/preheat/sites", "/preheat/stats", "/preheat/urls?siteId=default", "/preheat/task/status", "/preheat/crawler-headers",
        "/push/sites", "/push/stats", "/push/logs", "/push/trend", "/push/config?siteId=default",
        "/sites",
        "/ssl/certificates", "/ssl/certificates/expiring",
    ]
    for p in gets:
        call("GET", p)

    # 文本类端点(返回文件内容而非 JSON envelope)
    call("GET", "/seo/sitemap", accept_non_json=True, expect_code=None)
    call("GET", "/seo/robots", accept_non_json=True, expect_code=None)

    # 未认证应被拒
    saved = TOKEN
    TOKEN = ""
    call("GET", "/sites", auth=False, expect_http=401, expect_code=401)
    call("GET", "/overview", auth=False, expect_http=401, expect_code=401)
    TOKEN = saved

    # ── 写操作(幂等或自清理) ──
    # 建站: 成功(200)或免费版超限(402)均视为接口行为正确
    ok_add = call("POST", "/sites",
                  {"id": "api-test-site", "name": "API Test", "port": 29999,
                   "mode": "proxy", "domains": ["apitest.local"],
                   "proxy": {"backend_url": "http://127.0.0.1:1"}},
                  expect_code=None)
    idx = next(i for i, r in enumerate(RESULTS) if r[1] == "POST" and r[2] == "/sites")
    hc402 = RESULTS[idx][3] in (200, 201, 402)
    RESULTS[idx] = (hc402,) + RESULTS[idx][1:]

    # 若建站成功则测试站点级端点后删除
    req = urllib.request.Request(f"{BASE}/sites")
    req.add_header("Authorization", f"Bearer {TOKEN}")
    with urllib.request.urlopen(req, timeout=10) as r:
        site_ids = [s.get("id") for s in (json.loads(r.read().decode()).get("data") or [])]
    if "api-test-site" in site_ids:
        call("GET", "/sites/api-test-site")
        call("GET", "/sites/api-test-site/config?type=prerender")
        call("PUT", "/sites/api-test-site/prerender", {"enabled": False}, expect_code=None)
        call("POST", "/sites/api-test-site/stop", {}, expect_code=None)
        call("DELETE", "/sites/api-test-site", expect_code=None)

    # 默认站点的只读站点级端点
    if "default" in site_ids:
        call("GET", "/sites/default")
        call("GET", "/sites/default/config?type=prerender")

    call("GET", "/sites/nonexistent-xyz", expect_http=404, expect_code=404)
    call("DELETE", "/monitoring/alert-rules/nonexistent", expect_code=None)

    report()


def report():
    passed = sum(1 for r in RESULTS if r[0])
    failed = [r for r in RESULTS if not r[0]]
    print(f"\n===== API VERIFY: {passed}/{len(RESULTS)} passed =====")
    for ok, m, p, hc, rc, note in failed:
        print(f"  FAIL {m:6s} {p}  http={hc} code={rc}  {note}")
    if failed:
        sys.exit(1)


if __name__ == "__main__":
    main()
