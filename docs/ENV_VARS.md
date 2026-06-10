# Environment Variables Reference

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `JWT_SECRET` | auto-generated | no | JWT signing key (HMAC-SHA256). Auto-generated from config version if not set. **Must be set in production** |
| `PRERENDER_WORKER_COUNT` | 5 | no | Number of concurrent prerender workers |
| `PRERENDER_MASTER_KEY` | - | no | Master key for config encryption (AES-256-GCM). If not set, encryption is bypassed |
| `ENVIRONMENT` | - | no | Deployment environment label (development/staging/production) |
| `SERVICE_VERSION` | dev | no | Service version label for telemetry |
| `SERVICE_INSTANCE_ID` | hostname | no | Unique instance ID for telemetry |
| `HOSTNAME` | OS hostname | no | Hostname override for telemetry resource attributes |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | no | OpenTelemetry OTLP exporter endpoint (e.g., `localhost:4318`) |
| `OTEL_TRACES_EXPORTER` | - | no | Set to `console` to output traces to stdout instead of OTLP |
| `API_PUBLIC_URL` | `http://localhost:9598` | no | Public URL for the API, used by the admin console |
| `MONITORING_PROMETHEUS_ADDRESS` | `:9090` | no | Prometheus metrics server listen address |
| `REDIS_URL` | from config | no | Override Redis URL from environment |

## Usage

All environment variables can be set in the shell or in a `.env` file:

```bash
export JWT_SECRET="your-256-bit-secret"
export PRERENDER_WORKER_COUNT=10
```

YAML config files also support `${VAR:-default}` syntax for environment variable substitution:

```yaml
redis_url: "${REDIS_URL:-localhost:6379}"
public_api_url: "${API_PUBLIC_URL:-http://localhost:9598}"
```
