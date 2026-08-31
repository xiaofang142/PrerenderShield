# Environment Variables Reference

完整对照代码：配置覆盖见 [`internal/config/loader.go`](../internal/config/loader.go) `loadFromEnv()`，其余见 `internal/di/container.go`、`internal/prerender/engine.go` 等。

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `JWT_SECRET` | auto-generated | no | JWT signing key (HMAC-SHA256). If not set, a random 32-byte hex key is generated per process (random sessions are invalidated on restart). **Must be set in production** |
| `PRERENDER_WORKER_COUNT` | max site `pool_size`, else 5 | no | Number of concurrent prerender workers |
| `PRERENDER_MAX_INSTANCES` | 10 | no | Maximum Chrome browser instances. **Reduce to 2-3 in Docker with <4GB memory** |
| `PRERENDER_MIN_INSTANCES` | 2 | no | Minimum idle Chrome instances kept ready |
| `PRERENDER_MASTER_KEY` | - | no | Master key for config encryption (AES-256-GCM). If not set, encryption is bypassed |
| `PRERENDER_CHROMIUM_PATH` | auto-detect | no | Explicit Chromium/Chrome binary path (takes priority over system lookup) |
| `CHROME_PATH` | auto-detect | no | Fallback browser path if `PRERENDER_CHROMIUM_PATH` unset |
| `PRERENDER_GEOIP_CACHE` | `data/geoip_cache.json` | no | GeoIP disk cache file path (persistent fallback layer) |
| `PRERENDER_BOTVERIFY_CACHE` | `data/botverify_cache.json` | no | Bot verification (rDNS) disk cache file path; verified results cached 7d, negative 1h |
| `PRERENDER_PROCESS_CAP` | `MaxInstances*8+16` | no | Override the hard chromium process cap. Intended for tests running multiple binaries concurrently; do not lower in production |
| `FIREWALL_CACHE_KEY_SECRET` | derived per-site | no | Global HMAC secret for firewall cache key derivation (per-site keys derived from it) |
| `SERVER_ADDRESS` | from config (`0.0.0.0`) | no | Override API/console listen address |
| `SERVER_API_PORT` | from config (`9598`) | no | Override API port |
| `SERVER_CONSOLE_PORT` | from config (`9597`) | no | Override console port |
| `API_PUBLIC_URL` | `http://localhost:9598` | no | Public URL for the API, used by the admin console |
| `DIRS_DATA_DIR` | from config (`./data`) | no | Override data directory |
| `DIRS_STATIC_DIR` | from config (`./static`) | no | Override static files directory |
| `DIRS_CERTS_DIR` | from config (`./certs`) | no | Override certificates directory |
| `DIRS_ADMIN_STATIC_DIR` | from config (`./web`) | no | Override admin console static files directory |
| `CACHE_TYPE` | from config (`memory`) | no | Override cache type (`memory` / `redis`) |
| `REDIS_HOST` | - | no | With `REDIS_PORT`/`REDIS_PASSWORD`/`REDIS_DB`, builds the Redis URL `redis://[password@]host:port/db` |
| `REDIS_PORT` | 6379 | no | Redis port (only used together with `REDIS_HOST`) |
| `REDIS_PASSWORD` | - | no | Redis password (only used together with `REDIS_HOST`) |
| `REDIS_DB` | 0 | no | Redis DB index (only used together with `REDIS_HOST`) |
| `CACHE_REDIS_URL` | from config (`localhost:6379`) | no | Override the Redis URL (ignored when `REDIS_HOST` is set). Accepts `host:port` or `redis://[password@]host:port/db` |
| `CACHE_MEMORY_SIZE` | from config (`1000`) | no | Override L1 memory cache size (entries) |
| `STORAGE_TYPE` | from config (`redis`) | no | Override storage backend (`redis` / `memory`) |
| `MONITORING_ENABLED` | from config (`true`) | no | Enable/disable monitoring |
| `MONITORING_PROMETHEUS_ADDRESS` | `:9090` | no | Prometheus metrics server listen address |
| `COMMERCIAL_PLAN` | from config (`free`) | no | Licensing plan override (`free` / `per-site` / `private-source`) |
| `COMMERCIAL_MAX_SITES` | from config (`1`) | no | Commercial licensing site limit override (`-1` = unlimited) |
| `COMMERCIAL_SITE_PRICE_USD_PER_YEAR` | from config (`99`) | no | Commercial per-site yearly price override |
| `COMMERCIAL_PRIVATE_DEPLOY_PRICE_USD` | from config (`9999`) | no | Commercial private-deployment price override |
| `ACME_DIRECTORY_URL` | Let's Encrypt staging/production | no | Custom ACME directory URL (private CA / local Pebble testing); takes priority over `ssl.production` |
| `ACME_TLS_INSECURE` | off | no | Set to `1` to skip TLS verification against a custom ACME directory (testing only) |
| `ENVIRONMENT` | - | no | Deployment environment label (development/staging/production), attached to telemetry |
| `SERVICE_VERSION` | `dev` | no | Service version label for telemetry |
| `SERVICE_INSTANCE_ID` | hostname | no | Unique instance ID for telemetry |
| `HOSTNAME` | OS hostname | no | Hostname override for telemetry resource attributes |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | no | OpenTelemetry OTLP exporter endpoint (e.g., `localhost:4318`) |
| `OTEL_TRACES_EXPORTER` | - | no | Set to `console` to output traces to stdout instead of OTLP |

> **注意**：程序不读取 `REDIS_URL` 环境变量——Redis 连接请用 `CACHE_REDIS_URL`（或 `REDIS_HOST` 系列变量）。
> `REDIS_URL` 仅 Docker 入口脚本（`docker/docker-entrypoint.sh`）用于改写 `config.yml` 中的 `redis_url`。

## Usage

All environment variables can be set in the shell or in a `.env` file:

```bash
export JWT_SECRET="your-256-bit-secret"
export PRERENDER_WORKER_COUNT=10
```

YAML config files also support `${VAR:-default}` syntax for environment variable substitution:

```yaml
redis_url: "${CACHE_REDIS_URL:-localhost:6379}"
public_api_url: "${API_PUBLIC_URL:-http://localhost:9598}"
```
