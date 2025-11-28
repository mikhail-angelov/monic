# Monic - Monitoring Service (inspired by [monit service](https://mmonit.com/monit/))

Pure **vibe-coded** lightweight monitoring service written in Go that runs in Docker containers and monitors host system resources, HTTP endpoints, and Docker containers.

<img width="480" alt="image" src="https://github.com/user-attachments/assets/cfa23855-76db-4425-9d49-95689ea5b86a" />

## Features

- **System Resource Monitoring**
  - CPU usage monitoring with configurable thresholds
  - Memory (RAM) usage monitoring
  - Disk space monitoring for multiple paths
  - Configurable alert thresholds
  - Efficient collection with minimal resource usage

- **HTTP/HTTPS Monitoring**
  - Monitor multiple HTTP/HTTPS endpoints concurrently
  - Configurable timeouts and expected status codes
  - Response time tracking and success rate calculation
  - Concurrent checking for better performance

- **Docker Container Monitoring**
  - Monitor Docker container status and resource usage
  - Track running/stopped containers
  - Configurable container filtering
  - Works inside Docker containers while monitoring the host

- **Advanced Alerting System**
  - Email alerts via SMTP
  - Mailgun API integration
  - Configurable alert levels (warning, critical)
  - Alert cooldown and deduplication
  - 3 consecutive failures logic to prevent false alerts
  - Recovery alerts when issues are resolved

- **HTTP Stats Server**
  - RESTful API for monitoring data
  - Basic authentication support
  - Real-time system statistics and health checks
  - Historical data and alert status

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
  --network host \
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

The service uses a JSON configuration file (`config.json`). See `config-example.json` for a complete example.

### Configuration Structure

```json
{
  "system_checks": {
    "interval": 30,
    "cpu_threshold": 80,
    "memory_threshold": 85,
    "disk_threshold": 90,
    "disk_paths": ["/", "/var", "/home"]
  },
  "http_checks": [
    {
      "name": "local_service",
      "url": "http://localhost:8080/health",
      "method": "GET",
      "timeout": 10,
      "expected_status": 200,
      "check_interval": 30
    }
  ],
  "alerting": {
    "enabled": false,
    "alert_levels": ["warning", "critical"],
    "cooldown": 30,
    "email": {
      "enabled": false,
      "smtp_host": "smtp.gmail.com",
      "smtp_port": 587,
      "username": "your-email@gmail.com",
      "password": "your-app-password",
      "from": "monic@yourdomain.com",
      "to": "admin@yourdomain.com",
      "use_tls": true
    },
    "mailgun": {
      "enabled": false,
      "api_key": "your-mailgun-api-key",
      "domain": "your-domain.com",
      "from": "monic@yourdomain.com",
      "to": "admin@yourdomain.com",
      "base_url": "https://api.mailgun.net/v3"
    }
  },
  "docker_checks": {
    "enabled": true,
    "check_interval": 60,
    "containers": []
  },
  "http_server": {
    "enabled": true,
    "port": 8080,
    "username": "admin",
    "password": "monic123"
  }
}
```

### Configuration Options

- **system_checks**: System resource monitoring
  - `interval`: System check interval in seconds
  - `cpu_threshold`: CPU usage percentage threshold for alerts
  - `memory_threshold`: Memory usage percentage threshold for alerts
  - `disk_threshold`: Disk usage percentage threshold for alerts
  - `disk_paths`: List of disk paths to monitor

- **http_checks**: HTTP endpoint monitoring
  - `name`: Unique name for the check
  - `url`: HTTP/HTTPS URL to monitor
  - `method`: HTTP method (GET, POST, etc.)
  - `timeout`: Request timeout in seconds
  - `expected_status`: Expected HTTP status code
  - `check_interval`: Check interval in seconds

- **alerting**: Alert configuration
  - `enabled`: Enable/disable alerting
  - `alert_levels`: Alert levels to send (warning, critical)
  - `cooldown`: Minimum time between alerts for the same issue
  - `email`: SMTP email configuration
  - `mailgun`: Mailgun API configuration

- **docker_checks**: Docker container monitoring
  - `enabled`: Enable/disable Docker monitoring
  - `check_interval`: Docker check interval in seconds
  - `containers`: Specific containers to monitor (empty for all)

- **http_server**: HTTP stats server
  - `enabled`: Enable/disable HTTP server
  - `port`: HTTP server port
  - `username`: Basic auth username
  - `password`: Basic auth password

## Docker Configuration

### Host Monitoring

To monitor host system resources from within a Docker container:

1. Run with `--privileged` flag
2. Use `--network host` for network monitoring
3. Mount host filesystem: `-v /:/host:ro`
4. Mount Docker socket for container monitoring: `-v /var/run/docker.sock:/var/run/docker.sock:ro`



## HTTP Stats API

When the HTTP server is enabled, you can access monitoring data via REST API:

```bash
# Get monitoring stats
curl -u admin:monic123 http://localhost:8080/stats
```

### API Response Structure

```json
{
  "service_status": {
    "status": "running",
    "started_at": "2025-11-16T20:48:59+03:00"
  },
  "system_info": {
    "host": {
      "num_cpus": 8,
      "total_memory": 17179869184,
      "available_memory": 8589934592
    },
    "runtime": {
      "go_version": "go1.21.0",
      "num_cpu": 8,
      "goroutines": 15,
      "go_max_procs": 8
    }
  },
  "current_system_stats": {
    "timestamp": "2025-11-16T20:48:59+03:00",
    "cpu_usage": 15.23,
    "memory_usage": {
      "total": 17179869184,
      "used": 8589934592,
      "free": 8589934592,
      "used_percent": 50.0
    },
    "disk_usage": {
      "/": {
        "path": "/",
        "total": 107374182400,
        "used": 26843545600,
        "free": 80530636800,
        "used_percent": 25.0
      }
    }
  },
  "http_checks": {
    "total_checks": 2,
    "successful_checks": 2,
    "failed_checks": 0,
    "success_rate": 100.0
  },
  "alerts": {
    "active_alerts": 0
  },
  "thresholds": {
    "cpu_threshold": 80,
    "memory_threshold": 85,
    "disk_threshold": 90
  }
}
```

## Monitoring Output

The service logs monitoring information in the following format:

```
System Stats - CPU: 15.23%, Memory: 45.67%, Disk: [/:25.1%, /var:12.3%]
HTTP Stats - Total: 2, Success: 2, Failed: 0, Success Rate: 100.0%
Docker Stats - Total: 5, Running: 4, Stopped: 1, Running: 80.0%
ALERT [warning] cpu: CPU usage is 85.23% (threshold: 80%)
ALERT [critical] http_local_service: HTTP check failed for local_service: connection refused
```

## Alert Types

- **CPU**: CPU usage exceeds threshold
- **Memory**: Memory usage exceeds threshold  
- **Disk**: Disk usage exceeds threshold on any monitored path
- **HTTP**: HTTP check fails (wrong status code or connection error)
- **Docker**: Container status changes or resource issues

### Alert Logic

- **3 Consecutive Failures**: Alerts are only sent after 3 consecutive failures to prevent false alerts
- **Recovery Alerts**: Notifications are sent when issues are resolved
- **Alert Cooldown**: Prevents alert spam with configurable cooldown periods
- **State Management**: Tracks alert states to avoid duplicate notifications

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
│   └── server.go           # HTTP stats server
├── config.json             # Configuration file
├── config-example.json     # Example configuration
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
   - Monitor fewer disk paths

4. **Docker monitoring not working**
   - Ensure Docker socket is mounted correctly
   - Check container has access to Docker daemon

5. **Alerts not sending**
   - Verify alerting is enabled in configuration
   - Check SMTP/Mailgun credentials
   - Verify network connectivity for email sending

6. **Docker monitoring permission denied**
   - Check Docker socket permissions on host
   - Ensure user is in docker group
   - See detailed guide in `docker-permission-fix.md`

### Logs

Check container logs:
```bash
docker logs monic-monitor
```

## Docker Permission Issues

If you encounter Docker permission errors when trying to monitor Docker containers, please refer to the detailed troubleshooting guide:

[**Docker Permission Fix Guide**](docker-permission-fix.md)

This guide provides step-by-step solutions for common Docker socket permission issues, including:

- Checking and fixing Docker socket permissions
- Adding users to the docker group
- User ID mapping in containers
- Alternative solutions for different scenarios

## License

MIT License - see LICENSE file for details.
