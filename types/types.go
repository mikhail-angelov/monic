package types

import "time"

// Config represents the main configuration structure
type Config struct {
	AppName      string             `json:"app_name" envconfig:"APP_NAME"`
	SystemChecks SystemChecksConfig `json:"system_checks" envconfig:"SYSTEM_CHECKS"`
	HTTPChecks   []HTTPCheck        `json:"http_checks" envconfig:"HTTP_CHECKS"`
	Alerting     AlertingConfig     `json:"alerting" envconfig:"ALERTING"`
	DockerChecks DockerConfig       `json:"docker_checks" envconfig:"DOCKER_CHECKS"`
	HTTPServer   HTTPServerConfig   `json:"http_server" envconfig:"HTTP_SERVER"`
}

// SystemChecksConfig contains system monitoring settings
type SystemChecksConfig struct {
	Interval        int      `json:"interval" envconfig:"INTERVAL"`
	CPUThreshold    int      `json:"cpu_threshold" envconfig:"CPU_THRESHOLD"`
	MemoryThreshold int      `json:"memory_threshold" envconfig:"MEMORY_THRESHOLD"`
	DiskThreshold   int      `json:"disk_threshold" envconfig:"DISK_THRESHOLD"`
	DiskPaths       []string `json:"disk_paths" envconfig:"DISK_PATHS"`
}

// HTTPCheck defines a single HTTP/HTTPS endpoint to monitor
type HTTPCheck struct {
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Method         string    `json:"method"`
	Timeout        int       `json:"timeout"`
	ExpectedStatus int       `json:"expected_status"`
	CheckInterval  int       `json:"check_interval"`
	LastCheck      time.Time `json:"-"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level string `json:"level" envconfig:"LEVEL"`
	File  string `json:"file" envconfig:"FILE"`
}

// AlertingConfig contains alert notification settings
type AlertingConfig struct {
	Enabled     bool           `json:"enabled" envconfig:"ENABLED"`
	Email       EmailConfig    `json:"email" envconfig:"EMAIL"`
	Mailgun     MailgunConfig  `json:"mailgun" envconfig:"MAILGUN"`
	Telegram    TelegramConfig `json:"telegram" envconfig:"TELEGRAM"`
	AlertLevels []string       `json:"alert_levels" envconfig:"ALERT_LEVELS"` // info, warning, critical
	Cooldown    int            `json:"cooldown" envconfig:"COOLDOWN"`         // minutes between repeated alerts
}

// EmailConfig contains SMTP email settings
type EmailConfig struct {
	Enabled  bool   `json:"enabled" envconfig:"ENABLED"`
	SMTPHost string `json:"smtp_host" envconfig:"SMTP_HOST"`
	SMTPPort int    `json:"smtp_port" envconfig:"SMTP_PORT"`
	Username string `json:"username" envconfig:"USERNAME"`
	Password string `json:"password" envconfig:"PASSWORD"`
	From     string `json:"from" envconfig:"FROM"`
	To       string `json:"to" envconfig:"TO"`
	UseTLS   bool   `json:"use_tls" envconfig:"USE_TLS"`
}

// MailgunConfig contains Mailgun API settings
type MailgunConfig struct {
	Enabled bool   `json:"enabled" envconfig:"ENABLED"`
	APIKey  string `json:"api_key" envconfig:"API_KEY"`
	Domain  string `json:"domain" envconfig:"DOMAIN"`
	From    string `json:"from" envconfig:"FROM"`
	To      string `json:"to" envconfig:"TO"`
	BaseURL string `json:"base_url" envconfig:"BASE_URL"` // Default: "https://api.mailgun.net/v3"
}

// TelegramConfig contains Telegram bot settings
type TelegramConfig struct {
	Enabled  bool   `json:"enabled" envconfig:"ENABLED"`
	BotToken string `json:"bot_token" envconfig:"BOT_TOKEN"`
	ChatID   string `json:"chat_id" envconfig:"CHAT_ID"`
}

// SystemStats contains collected system statistics
type SystemStats struct {
	Timestamp   time.Time            `json:"timestamp"`
	CPUUsage    float64              `json:"cpu_usage"`
	MemoryUsage MemoryStats          `json:"memory_usage"`
	DiskUsage   map[string]DiskStats `json:"disk_usage"`
}

// MemoryStats contains memory usage information
type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskStats contains disk usage information for a specific path
type DiskStats struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// HTTPCheckResult contains the result of an HTTP check
type HTTPCheckResult struct {
	Name         string        `json:"name"`
	URL          string        `json:"url"`
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// Alert represents a monitoring alert
type Alert struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Level     string    `json:"level"` // info, warning, critical
	Timestamp time.Time `json:"timestamp"`
}

// AlertState tracks the state of alerts for deduplication
type AlertState struct {
	Type              string    `json:"type"`
	CurrentState      string    `json:"current_state"` // "ok", "warning", "critical"
	ConsecutiveChecks int       `json:"consecutive_checks"`
	LastAlertSent     time.Time `json:"last_alert_sent"`
	LastStateChange   time.Time `json:"last_state_change"`
}

// DockerConfig contains Docker container monitoring settings
type DockerConfig struct {
	Enabled       bool     `json:"enabled" envconfig:"ENABLED"`
	CheckInterval int      `json:"check_interval" envconfig:"CHECK_INTERVAL"`
	Containers    []string `json:"containers" envconfig:"CONTAINERS"` // Specific containers to monitor, empty for all
}

// DockerContainerStats contains Docker container status information
type DockerContainerStats struct {
	ContainerID  string    `json:"container_id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	State        string    `json:"state"`
	Running      bool      `json:"running"`
	Created      time.Time `json:"created"`
	StartedAt    string    `json:"started_at,omitempty"`
	FinishedAt   string    `json:"finished_at,omitempty"`
	ExitCode     int       `json:"exit_code,omitempty"`
	Error        string    `json:"error,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// HTTPServerConfig contains HTTP server settings for stats endpoint
type HTTPServerConfig struct {
	Enabled  bool   `json:"enabled" envconfig:"ENABLED"`
	Port     int    `json:"port" envconfig:"PORT"`
	Username string `json:"username" envconfig:"USERNAME"`
	Password string `json:"password" envconfig:"PASSWORD"`
}
