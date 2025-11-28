package types

import "time"

// Config represents the main configuration structure
type Config struct {
	AppName      string             `envconfig:"APP_NAME"`
	SystemChecks SystemChecksConfig `envconfig:"CHECK_SYSTEM"`
	HTTPChecks   HTTPCheck          `envconfig:"CHECK_HTTP"`
	Alerting     AlertingConfig     `envconfig:"ALERTING"`
	DockerChecks DockerConfig       `envconfig:"CHECK_DOCKER"`
	HTTPServer   HTTPServerConfig   `envconfig:"HTTP_SERVER"`
}

// SystemChecksConfig contains system monitoring settings
type SystemChecksConfig struct {
	Interval        int      `envconfig:"INTERVAL"`
	CPUThreshold    int      `envconfig:"CPU_THRESHOLD"`
	MemoryThreshold int      `envconfig:"MEMORY_THRESHOLD"`
	DiskThreshold   int      `envconfig:"DISK_THRESHOLD"`
	DiskPaths       []string `envconfig:"DISK_PATHS"`
}

// HTTPCheck defines a single HTTP/HTTPS endpoint to monitor
type HTTPCheck struct {
	URL            string    `envconfig:"URL"`
	Method         string    `envconfig:"METHOD"`
	Timeout        int       `envconfig:"TIMEOUT"`
	ExpectedStatus int       `envconfig:"EXPECTED_STATUS"`
	CheckInterval  int       `envconfig:"INTERVAL"`
	LastCheck      time.Time ``
}

// AlertingConfig contains alert notification settings
type AlertingConfig struct {
	Email    EmailConfig    `envconfig:"EMAIL"`
	Mailgun  MailgunConfig  `envconfig:"EMAILGUN"`
	Telegram TelegramConfig `envconfig:"TELEGRAM"`
}

// EmailConfig contains SMTP email settings
type EmailConfig struct {
	Enabled  bool
	SMTPHost string `envconfig:"SMTP_HOST"`
	SMTPPort int    `envconfig:"SMTP_PORT"`
	Username string `envconfig:"USERNAME"`
	Password string `envconfig:"PASSWORD"`
	From     string `envconfig:"FROM"`
	To       string `envconfig:"TO"`
	UseTLS   bool   `envconfig:"USE_TLS"`
}

// MailgunConfig contains Mailgun API settings
type MailgunConfig struct {
	Enabled bool
	APIKey  string `envconfig:"API_KEY"`
	Domain  string `envconfig:"DOMAIN"`
	From    string `envconfig:"FROM"`
	To      string `envconfig:"TO"`
	BaseURL string `envconfig:"BASE_URL"` // Default: "https://api.mailgun.net/v3"
}

// TelegramConfig contains Telegram bot settings
type TelegramConfig struct {
	Enabled  bool
	BotToken string `envconfig:"BOT_TOKEN"`
	ChatID   string `envconfig:"CHAT_ID"`
}

// SystemStats contains collected system statistics
type SystemStats struct {
	Timestamp   time.Time
	CPUUsage    float64
	MemoryUsage MemoryStats
	DiskUsage   map[string]DiskStats
}

// MemoryStats contains memory usage information
type MemoryStats struct {
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// DiskStats contains disk usage information for a specific path
type DiskStats struct {
	Path        string
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// HTTPCheckResult contains the result of an HTTP check
type HTTPCheckResult struct {
	Name         string
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	Error        string
	Timestamp    time.Time
}

// Alert represents a monitoring alert
type Alert struct {
	Type      string
	Message   string
	Level     string // info, warning, critical
	Timestamp time.Time
}

// AlertState tracks the state of alerts for deduplication
type AlertState struct {
	Type              string
	CurrentState      string // "ok", "warning", "critical"
	ConsecutiveChecks int
	LastAlertSent     time.Time
	LastStateChange   time.Time
}

// DockerConfig contains Docker container monitoring settings
type DockerConfig struct {
	Enabled       bool
	CheckInterval int      `envconfig:"INTERVAL"`
	Containers    []string `envconfig:"CONTAINERS"` // Specific containers to monitor, empty for all
}

// DockerContainerStats contains Docker container status information
type DockerContainerStats struct {
	ContainerID string
	Name        string
	Status      string
	State       string
	Running     bool
	Created     time.Time
	StartedAt   string
	FinishedAt  string
	ExitCode    int
	Error       string
	Timestamp   time.Time
}

// HTTPServerConfig contains HTTP server settings for stats endpoint
type HTTPServerConfig struct {
	Enabled  bool
	Port     int    `envconfig:"PORT"`
	Username string `envconfig:"USERNAME"`
	Password string `envconfig:"PASSWORD"`
}
