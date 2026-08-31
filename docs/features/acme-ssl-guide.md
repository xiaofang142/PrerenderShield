# SSL / ACME 证书管理使用指南

> Prerender Shield 集成 Let's Encrypt 自动申请、续期与证书导入。本文面向**使用者**：三种证书来源（HTTP-01 自动 / DNS-01 通配符 / 手动导入）怎么配、怎么验证、有哪些坑。
> 键位说明见 [CONFIG_REFERENCE.md](../CONFIG_REFERENCE.md) 的 `ssl`（全局）与 `sites[].ssl`（站点）。

---

## 三种证书来源

| 来源 | 适用 | 需要 |
|------|------|------|
| **HTTP-01 自动** | 单域名 / 多域名（不含通配符 `*.`） | 80 端口可达、域名解析到本机 |
| **DNS-01 自动** | **通配符证书**（`*.example.com`）、80 不可达 | DNS 服务商 API 凭证 |
| **手动导入** | 已有证书 / 企业 CA / 跨端口 | `cert_file` + `key_file` 的 PEM 路径 |

---

## 一、HTTP-01 自动申请（最简单）

先开**全局开关**，再在**每个需要证书的站点**开启 `auto_cert`：

```yaml
# 全局（你的 ACME 账户）
ssl:
  enabled: true
  email: "admin@example.com"          # ACME 账户邮箱，必填
  production: false                    # false=ACME staging（联调）；上线改 true
  http_port: 80                        # HTTP-01 挑战端口，80 端口路径必需可访问
  auto_renew: true
  renew_before_days: 30
  webhook_url: ""                      # 证书事件通知（可选）

# 站点级
sites:
  - id: "site1"
    ssl:
      enabled: true
      auto_cert: true                  # 为该站点域名自动申请
      force_https: true
      hsts: true
      hsts_max_age: 31536000
```

### 验证

证书是**写入 `dirs.certs_dir` 目录的 PEM 文件**（`<域名>.crt` / `<域名>.key` / `<域名>.issuer.crt`），Redis 仅存续期状态标记（`ssl:renewal:<域名>`，非证书本体）：

```bash
ls -l ./certs/<你的域名>.key ./certs/<你的域名>.crt   # 证书文件已落地
openssl x509 -in ./certs/<你的域名>.crt -noout -subject -dates  # 查看有效期
curl -I https://你的域名/                                # 证书有效即 200/301
```

> 要点：域名 A 记录须指向本机、80 端口对外可达、`production: true` 才签发能被浏览器信任的正式证书（`false` 仅为 let's encrypt staging 联调，浏览器会报警）。

---

## 二、DNS-01 自动申请（通配符 + 无需 80 端口）

通配符（`*.example.com`）**必须**用 DNS-01。选一个支持的 DNS 提供商并放 API 凭证：

```yaml
ssl:
  enabled: true
  email: "admin@example.com"
  production: true
  dns:
    provider: "cloudflare"    # cloudflare / aliyun / tencentcloud / aws / godaddy
    credentials:
      CLOUDFLARE_DNS_API_TOKEN: "<token>"     # cloudflare
      # 阿里云：ALIBABA_CLOUD_ACCESS_KEY_ID: "xxx"
      #         ALIBABA_CLOUD_ACCESS_KEY_SECRET: "xxx"
      # 腾讯云：TENCENTCLOUD_SECRET_ID: "xxx"
      #         TENCENTCLOUD_SECRET_KEY: "xxx"
```

站点侧触发 `RequestWildcardCertificate`（`internal/ssl/dns_challenge.go`）：

```yaml
sites:
  - id: "site1"
    ssl:
      enabled: true
      auto_cert: true        # 对 base domain 自动申请通配符
```

### 权限要求

DNS 提供商凭证需具备创建/删除 **TXT 记录**的权限（ACME 通过临时 `_acme-challenge` TXT 记录验证）。

### 验证

```bash
ls -l ./certs/*.key ./certs/*.crt                     # 通配符证书已落地
openssl x509 -in ./certs/<你的域名>.crt -noout -subject -ext subjectAltName   # 应含 *.example.com
# 日志确认：Wildcard certificate obtained for: example.com
```

---

## 三、手动导入已有证书

```yaml
sites:
  - id: "site1"
    ssl:
      enabled: true
      cert_file: "./certs/example.com.fullchain.pem"   # 含完整链
      key_file:  "./certs/example.com.key.pem"
      force_https: true
```

> 建议证书链完整（leaf + intermediate），否则客户端可能报链不完整。可用
> `openssl verify -CAfile <ca.pem> example.com.fullchain.pem` 校验。

---

## 四、自动续期

- 全局 `auto_renew: true` 后，`check_interval`（默认 24h）周期检查，`renew_before_days`（默认 30 天）内到期自动续签。
- 续签失败按 `max_retries` / `retry_delay` 重试；成功/失败可经 `webhook_url` 通知。
- 手动导入的证书**不参与自动续期**（无 ACME 账户），到期前需自行替换。

### 验证续期

```bash
# 监控日志中 AutoRenew 相关记录
grep -i "renew\|expir" data/prerender-shield.log
```

---

## 常见坑

| 症状 | 原因 / 处置 |
|------|-----------|
| 浏览器提示证书无效 | `production: false` 用了 staging；改 `true` 重签 |
| HTTP-01 一直失败 | 80 端口未开放 / 域名未解析到本机 / 被防火墙/反代占用 |
| 通配符申请失败 | 未用 DNS-01；或 DNS 凭证缺少 TXT 记录权限 |
| 自动续期不生效 | `auto_renew` 未开，或证书为手动导入 |
| `certs_dir` 写不进去 | 检查 `dirs.certs_dir` 目录权限（Docker 下挂载卷） |

> 通配符证书优先用于多子域名站点（拦截所有 `sub.*.example.com`）；普通单域名走 HTTP-01 更省配置。
