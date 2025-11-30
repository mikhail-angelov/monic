# Monic - Monitoring Service (inspired by [monit service](https://mmonit.com/monit/))

Pure **vibe-coded** lightweight monitoring service written in Go that runs in Docker containers and monitors host system resources, HTTP endpoints, and Docker containers.

<img width="480" alt="image" src="https://github.com/user-attachments/assets/cfa23855-76db-4425-9d49-95689ea5b86a" />

## Features

- **System Resource Monitoring**
  - CPU usage monitoring with configurable thresholds
  - Memory (RAM) usage monitoring
  - Disk space monitoring for root path ("/")
  - Configurable alert thresholds
  - Efficient collection with minimal resource usage

- **HTTP/HTTPS Monitoring**
  - Monitor HTTP/HTTPS endpoint
  - Configurable timeouts and expected status codes
  - Response time tracking
  - Concurrent checking (internal support)

- **Docker Container Monitoring**
  - Monitor Docker container status and resource usage
  - Track running/stopped containers
  - Configurable container filtering
  - Works inside Docker containers while monitoring the host

- **Advanced Alerting System**
  - Email alerts via SMTP
  - Mailgun API integration
  - Telegram bot notifications
  - Configurable alert levels (warning, critical)
  - Alert cooldown and deduplication
  - 3 consecutive failures logic to prevent false alerts
  - Recovery alerts when issues are resolved

- **HTTP Stats Server**
  - RESTful API for monitoring data
  - Basic authentication support
  - Real-time system statistics and health checks
  - Historical data and alert status
  - Web interface with disk size information

- **Container Ready**
  - Runs efficiently in Docker containers
  - Monitors host system resources when running in privileged mode
  - Lightweight Alpine-based Docker image
  - Health checks and graceful shutdown

- **CPU Efficient**
  - Optimized goroutines for concurrent monitoring
  - Configurable check intervals to reduce resource usage
  - Minimal memory footprint
  - Efficient system calls using gopsutil
  - State management to prevent redundant alerts

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone and run
git clone <repository>
cd monic
docker-compose up -d
```

### Using Docker

```bash
# Build the image
docker build -t monic .

# Run the container
docker run -d \
  --name monic \
  --privileged \
  -v /:/host:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  monic
```

### Running Locally

```bash
# Install dependencies
go mod download

# Build and run
go build -o monic main.go
./monic

# Or use Makefile
make build
./monic
```

## Configuration

The service uses environment variables for configuration. See `.env.example` for a complete example.

### Environment Variables

The application uses environment variables with the `MONIC_` prefix. Here are the main configuration options:

```bash
# Basic Configuration
MONIC_APP_NAME="Monic Monitoring"

# System Monitoring
MONIC_CHECK_SYSTEM_INTERVAL=30
MONIC_CHECK_SYSTEM_CPU_THRESHOLD=80
MONIC_CHECK_SYSTEM_MEMORY_THRESHOLD=85
MONIC_CHECK_SYSTEM_DISK_THRESHOLD=90

# HTTP Monitoring
MONIC_CHECK_HTTP_URL="https://google.com"
MONIC_CHECK_HTTP_METHOD="GET"
MONIC_CHECK_HTTP_TIMEOUT=5
MONIC_CHECK_HTTP_EXPECTED_STATUS=200
MONIC_CHECK_HTTP_INTERVAL=30

# HTTP Server (Stats Endpoint)
MONIC_HTTP_SERVER_PORT=8080
MONIC_HTTP_SERVER_USERNAME="admin"
MONIC_HTTP_SERVER_PASSWORD="monic123"

# Email Alerting (SMTP)
MONIC_ALERTING_EMAIL_SMTP_HOST="smtp.gmail.com"
MONIC_ALERTING_EMAIL_SMTP_PORT=587
MONIC_ALERTING_EMAIL_USERNAME="your-email@gmail.com"
MONIC_ALERTING_EMAIL_PASSWORD="your-app-password"
MONIC_ALERTING_EMAIL_FROM="monic@yourdomain.com"
MONIC_ALERTING_EMAIL_TO="admin@yourdomain.com"
MONIC_ALERTING_EMAIL_USE_TLS=true

# Mailgun Alerting
MONIC_ALERTING_MAILGUN_API_KEY="your-mailgun-api-key"
MONIC_ALERTING_MAILGUN_DOMAIN="your-domain.com"
MONIC_ALERTING_MAILGUN_FROM="monic@yourdomain.com"
MONIC_ALERTING_MAILGUN_TO="admin@yourdomain.com"
MONIC_ALERTING_MAILGUN_BASE_URL="https://api.mailgun.net/v3"

# Telegram Alerting
MONIC_ALERTING_TELEGRAM_BOT_TOKEN="your-bot-token"
MONIC_ALERTING_TELEGRAM_CHAT_ID="your-chat-id"

# Docker Monitoring
MONIC_CHECK_DOCKER_INTERVAL=60
MONIC_CHECK_DOCKER_CONTAINERS="container1,container2"
```

### Configuration Options

- **System Monitoring** (`MONIC_CHECK_SYSTEM_*`)
  - `INTERVAL`: System check interval in seconds (default: 30)
  - `CPU_THRESHOLD`: CPU usage percentage threshold for alerts (default: 80)
  - `MEMORY_THRESHOLD`: Memory usage percentage threshold for alerts (default: 85)
  - `DISK_THRESHOLD`: Disk usage percentage threshold for alerts (default: 90)
  - **Note**: Disk monitoring now only checks the root path ("/") for simplicity

- **HTTP Monitoring** (`MONIC_CHECK_HTTP_*`)
  - `URL`: Target URL to monitor
  - `METHOD`: HTTP method (GET, POST, etc.)
  - `TIMEOUT`: Request timeout in seconds
  - `EXPECTED_STATUS`: Expected HTTP status code (e.g., 200)
  - `INTERVAL`: Check interval in seconds

- **HTTP Server** (`MONIC_HTTP_SERVER_*`)
  - `PORT`: HTTP server port for stats endpoint (default: 8080)
  - `USERNAME`: Basic auth username (optional)
  - `PASSWORD`: Basic auth password (optional)
  - **Note**: Server is automatically enabled when port is configured

- **Email Alerting** (`MONIC_ALERTING_EMAIL_*`)
  - `SMTP_HOST`: SMTP server hostname
  - `SMTP_PORT`: SMTP server port
  - `USERNAME`: SMTP username
  - `PASSWORD`: SMTP password
  - `FROM`: Sender email address
  - `TO`: Recipient email address
  - `USE_TLS`: Enable TLS (true/false)

- **Mailgun Alerting** (`MONIC_ALERTING_MAILGUN_*`)
  - `API_KEY`: Mailgun API key
  - `DOMAIN`: Mailgun domain
  - `FROM`: Sender email address
  - `TO`: Recipient email address
  - `BASE_URL`: Mailgun API base URL

- **Telegram Alerting** (`MONIC_ALERTING_TELEGRAM_*`)
  - `BOT_TOKEN`: Telegram bot token
  - `CHAT_ID`: Telegram chat ID

- **Docker Monitoring** (`MONIC_CHECK_DOCKER_*`)
  - `INTERVAL`: Docker check interval in seconds (default: 60)
  - `CONTAINERS`: Comma-separated list of specific containers to monitor (empty for all)

## Docker Configuration

### Host Monitoring

To monitor host system resources from within a Docker container:

1. Run with `--privileged` flag
2. Mount host filesystem: `-v /:/host:ro`
3. Mount Docker socket for container monitoring: `-v /var/run/docker.sock:/var/run/docker.sock:ro`


## Web Interface

The HTTP stats server provides a web interface at `/stats` that displays:

- **System Resources**: CPU, memory, and disk usage with progress bars
- **Disk Information**: Total size, used space, free space in GB
- **HTTP Checks**: Status of monitored endpoints
- **Recent Alerts**: Active and recent alerts
- **System Details**: Host information and runtime stats

The interface automatically refreshes every 30 seconds and shows disk size information with color-coded thresholds.

## Monitoring Output

The service logs monitoring information in the following format:

```
System Stats - CPU: 15.23%, Memory: 45.67%, Disk: /:25.1%
HTTP Stats - Total: 2, Success: 2, Failed: 0, Success Rate: 100.0%
Docker Stats - Total: 5, Running: 4, Stopped: 1, Running: 80.0%
ALERT [warning] cpu: CPU usage is 85.23% (threshold: 80%)
ALERT [critical] http_local_service: HTTP check failed for local_service: connection refused
```

## Alert Types

- **CPU**: CPU usage exceeds threshold
- **Memory**: Memory usage exceeds threshold  
- **Disk**: Disk usage exceeds threshold on root path
- **HTTP**: HTTP check fails (wrong status code or connection error)
- **Docker**: Container status changes or resource issues

### Alert Logic

- **3 Consecutive Failures**: Alerts are only sent after 3 consecutive failures to prevent false alerts
- **Recovery Alerts**: Notifications are sent when issues are resolved
- **Alert Cooldown**: Prevents alert spam with configurable cooldown periods
- **State Management**: Tracks alert states to avoid duplicate notifications
- **Automatic Feature Detection**: Features are automatically enabled when their configuration is provided

## Performance Considerations

- **CPU Efficient**: Uses goroutines and configurable intervals to minimize CPU usage
- **Memory Efficient**: Limited history retention (last 100 entries)
- **Concurrent Monitoring**: Performs system, HTTP, and Docker checks concurrently
- **Graceful Shutdown**: Properly handles SIGINT/SIGTERM signals
- **Efficient State Tracking**: Minimal memory usage for alert state management

## Development

### Project Structure

```
.
├── main.go                 # Main application entry point
├── types/
│   └── types.go            # Data structures and types
├── monitor/
│   ├── system.go           # System resource monitoring
│   ├── http.go             # HTTP endpoint monitoring
│   └── docker_simple.go    # Docker container monitoring
├── alert/
│   ├── alert.go            # Alert management and sending
│   └── state_manager.go    # Alert state tracking
├── server/
│   ├── server.go           # HTTP stats server
│   ├── template.go         # HTML template rendering
│   └── templates/
│       └── stats.html      # Web interface template
├── config/
│   ├── config.go           # Configuration loading
│   └── config_test.go      # Configuration tests
├── .env.example            # Example environment variables
├── Dockerfile              # Container build configuration
├── docker-compose.yml      # Docker Compose configuration
├── Makefile                # Build and test commands
└── README.md               # This file
```

### Building and Testing

```bash
# Install dependencies
go mod download

# Build for current platform
go build -o monic main.go

# Run tests
go test ./...

# Or use Makefile
make build
make test
```

### Makefile Commands

- `make build` - Build the application
- `make test` - Run all tests
- `make clean` - Clean build artifacts
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container

## CI/CD

The project includes GitHub Actions workflows for automated testing and Docker releases.

### Automated Testing

The `test.yml` workflow runs on every push to `main` and `develop` branches, and on pull requests to `main`. It:

- Runs all Go tests
- Builds the application
- Verifies the build output

### Docker Releases

The `docker-release.yml` workflow automatically builds and publishes Docker images when version tags are pushed:

- **Trigger**: Pushes to tags matching `v*.*.*` (e.g., `v1.0.0`)
- **Actions**:
  - Runs tests
  - Builds multi-arch Docker images (linux/amd64, linux/arm64)
  - Pushes to GitHub Container Registry (GHCR)
  - Creates GitHub releases with release notes

### Creating a Release

1. Create and push a version tag:
```bash
git tag v1.0.0
git push origin v1.0.0
```

2. The workflow will automatically:
   - Build and test the code
   - Create multi-arch Docker images
   - Push to `ghcr.io/yourusername/monic`
   - Create a GitHub release

### Docker Image Tags

Images are tagged with:
- `latest` (for the default branch)
- `v1.0.0` (specific version)
- `v1.0` (major.minor)
- `v1` (major)

### Pulling Docker Images

```bash
# Latest version
docker pull ghcr.io/yourusername/monic:latest

# Specific version
docker pull ghcr.io/yourusername/monic:v1.0.0
```

## Version Information

The application includes version information that can be accessed via:

```bash
# Check version
./monic --version
# or
./monic -v
```

The version is set during build and is included in Docker images when built via GitHub Actions.

## Troubleshooting

### Common Issues

1. **Permission denied errors**
   - Ensure container runs with `--privileged` flag
   - Check mounted volume permissions

2. **HTTP checks failing**
   - Verify URLs are accessible from container network
   - Check firewall rules and network configuration

3. **High CPU usage**
   - Increase check intervals in configuration
   - Reduce number of HTTP checks

4. **Docker monitoring not working**
   - Ensure Docker socket is mounted correctly
   - Check container has access to Docker daemon

5. **Alerts not sending**
   - Verify alerting environment variables are set correctly
   - Check SMTP/Mailgun/Telegram credentials
   - Verify network connectivity for alert sending

### Logs

Check container logs:
```bash
docker logs monic-monitor
```

## License

MIT License - see LICENSE file for details.
