# Monic — Monitoring Service

Lightweight monitoring service written in Go. Runs in Docker and monitors host system resources, HTTP endpoints, and Docker containers via label-based discovery.

<img width="480" alt="image" src="https://github.com/user-attachments/assets/cfa23855-76db-4425-9d49-95689ea5b86a" />

## Features

- **System Resource Monitoring** — CPU, memory, and disk (root path `/`) with configurable thresholds
- **Docker Container Monitoring** — automatic label-based discovery; per-container HTTP health checks
- **HTTP Health Checks** — per-container endpoint monitoring with configurable intervals and expected status codes
- **Daily Digest** — summary report sent every day at midnight UTC via all configured alert channels
- **Alerting** — Email (SMTP), Mailgun, and Telegram; 3-consecutive-failure logic, recovery alerts, per-type cooldown
- **Stats Web UI** — `/stats` endpoint with JSON API and HTML dashboard, protected by HTTP Basic Auth

## Quick Start

```bash
# Clone
git clone https://github.com/mikhail-angelov/monic
cd monic

# Copy and edit config
cp .env.example .env

# Run
docker compose up -d
```

## Configuration

All settings are environment variables with the `MONIC_` prefix. Copy `.env.example` as a starting point.

### System Monitoring

| Variable | Default | Description |
|---|---|---|
| `MONIC_CHECK_SYSTEM_INTERVAL` | `60` | Check interval in seconds |
| `MONIC_CHECK_SYSTEM_CPU_THRESHOLD` | `80` | CPU % alert threshold |
| `MONIC_CHECK_SYSTEM_MEMORY_THRESHOLD` | `85` | Memory % alert threshold |
| `MONIC_CHECK_SYSTEM_DISK_THRESHOLD` | `90` | Disk % alert threshold (root `/`) |

### Docker Discovery

Docker monitoring is always enabled. If the Docker socket is unavailable at startup, Monic logs a warning and continues without it.

| Variable | Default | Description |
|---|---|---|
| `MONIC_CHECK_DOCKER_INTERVAL` | `300` | How often to poll the Docker API (seconds) |
| `MONIC_CONTAINER_ID` | — | Container ID of Monic itself — excluded from monitoring |

### HTTP Stats Server

Automatically enabled when any of these variables are set.

| Variable | Default | Description |
|---|---|---|
| `MONIC_HTTP_SERVER_PORT` | — | Port to listen on |
| `MONIC_HTTP_SERVER_USERNAME` | — | Basic auth username (optional) |
| `MONIC_HTTP_SERVER_PASSWORD` | — | Basic auth password (optional) |

### Daily Digest

Enabled by default. Sends a 24-hour summary at midnight UTC.

| Variable | Default | Description |
|---|---|---|
| `MONIC_DIGEST_ENABLED` | `true` | Set to `false` to disable |

### Alerting

Each channel is auto-enabled when its variables are present.

**Email (SMTP)**

| Variable | Description |
|---|---|
| `MONIC_ALERTING_EMAIL_SMTP_HOST` | SMTP hostname (e.g. `postbox.cloud.yandex.net`) |
| `MONIC_ALERTING_EMAIL_SMTP_PORT` | Port — `587` for STARTTLS, `465` for TLS |
| `MONIC_ALERTING_EMAIL_USERNAME` | SMTP username |
| `MONIC_ALERTING_EMAIL_PASSWORD` | SMTP password or API key |
| `MONIC_ALERTING_EMAIL_FROM` | Sender address |
| `MONIC_ALERTING_EMAIL_TO` | Recipient address |
| `MONIC_ALERTING_EMAIL_USE_TLS` | `true` for port 465 (direct TLS); `false` for port 587 (STARTTLS) |

**Mailgun**

| Variable | Description |
|---|---|
| `MONIC_ALERTING_MAILGUN_API_KEY` | Mailgun API key |
| `MONIC_ALERTING_MAILGUN_DOMAIN` | Mailgun domain |
| `MONIC_ALERTING_MAILGUN_FROM` | Sender address |
| `MONIC_ALERTING_MAILGUN_TO` | Recipient address |
| `MONIC_ALERTING_MAILGUN_BASE_URL` | API base URL (default: `https://api.mailgun.net/v3`) |

**Telegram**

| Variable | Description |
|---|---|
| `MONIC_ALERTING_TELEGRAM_BOT_TOKEN` | Bot token from `@BotFather` |
| `MONIC_ALERTING_TELEGRAM_CHAT_ID` | Target chat or channel ID |

## Docker Container Labels

Monic discovers containers by polling the Docker API every `MONIC_CHECK_DOCKER_INTERVAL` seconds. Add these labels to any container you want monitored:

| Label | Required | Default | Description |
|---|---|---|---|
| `monic.enabled` | yes | — | Set to `"true"` to enable monitoring |
| `monic.check` | no | `container` | `container` (status only) or `http` (+ health check) |
| `monic.check_http_url` | no | — | URL for HTTP health check; also implicitly sets `monic.check=http` |
| `monic.check_http_interval` | no | `30` | Seconds between HTTP checks |
| `monic.check_http_timeout` | no | `5` | HTTP request timeout in seconds |
| `monic.check_http_expected` | no | `200` | Expected HTTP status code |
| `monic.name` | no | container name | Display name in alerts and UI |

### Examples

Monitor container status only:

```yaml
services:
  postgres:
    image: postgres:16
    labels:
      monic.enabled: "true"
```

Monitor status + HTTP health check:

```yaml
services:
  my-app:
    image: my-app:latest
    labels:
      monic.enabled: "true"
      monic.check: "http"
      monic.check_http_url: "https://example.com/api/health"
      monic.check_http_interval: "30"
      monic.name: "My Web App"
```

## Host Monitoring

To monitor host resources from inside a container, run with:

```yaml
services:
  monic:
    privileged: true
    volumes:
      - /:/host:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

## Alert Logic

- Alerts fire after **3 consecutive failures** to suppress transient spikes
- A **recovery alert** is sent when the metric returns to normal (only if a critical alert was previously sent)
- Each alert type has a **1-minute cooldown** to prevent spam
- The daily digest summarises the past 24 hours across all monitors

## Web UI

The stats server serves:

- `GET /stats` — JSON response (with `Accept: application/json`) or HTML dashboard
- Protected by HTTP Basic Auth when credentials are configured

Dashboard shows: CPU / memory / disk usage, container statuses, HTTP check history, recent alerts.

## Project Structure

```
.
├── main.go                  # Entry point
├── types/types.go           # Shared data structures
├── config/config.go         # Configuration loading
├── discovery/watcher.go     # Docker label-based container discovery
├── monitor/
│   ├── system.go            # System resource collection
│   ├── http.go              # HTTP health check registry
│   └── docker.go            # Container state tracker
├── alert/
│   ├── alert_manager.go     # Alert sending (email, Mailgun, Telegram)
│   └── state_manager.go     # Deduplication and 3-failure logic
├── server/
│   ├── service.go           # Main monitoring service / goroutine orchestration
│   ├── server.go            # HTTP stats server
│   ├── storage.go           # In-memory ring-buffer storage
│   ├── digest.go            # Daily digest builder
│   └── template.go          # HTML rendering
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Building

```bash
# Build binary
make build

# Run tests
make test

# Build Docker image
make docker-build

# Deploy to remote host (requires HOST= in .env)
make deploy
```

## CI/CD

GitHub Actions builds and publishes on version tags:

```bash
git tag v1.2.0
git push origin v1.2.0
```

Images are pushed to `ghcr.io/mikhail-angelov/monic` with tags `latest`, `v1.2.0`, `v1.2`, `v1`.

## Troubleshooting

| Problem | Check |
|---|---|
| Docker monitoring shows no containers | Verify `/var/run/docker.sock` is mounted and containers have `monic.enabled=true` |
| Alerts not arriving | Confirm env vars are set; check logs for send errors |
| High CPU | Increase `MONIC_CHECK_SYSTEM_INTERVAL` |
| Permission denied | Run container with `--privileged` and `-v /:/host:ro` |

```bash
docker logs monic
```

## Version

```bash
./monic --version
```

## License

MIT
