package types

import "time"

// Config represents the main configuration structure
type Config struct {
	AppName      string             `json:"app_name" envconfig:"APP_NAME"`
	SystemChecks SystemChecksConfig `json:"system_checks"`
	HTTPChecks   []HTTPCheck        `json:"http_checks"`
	Alerting     AlertingConfig     `json:"alerting"`
	DockerChecks DockerConfig       `json:"docker_checks"`
	HTTPServer   HTTPServerConfig   `json:"http_server"`
}

// SystemChecksConfig contains system monitoring settings
type SystemChecksConfig struct {
	Interval        int      `json:"interval" envconfig:"CHECK_SYSTEM_INTERVAL"`
	CPUThreshold    int      `json:"cpu_threshold" envconfig:"CHECK_SYSTEM_CPU_THRESHOLD"`
	MemoryThreshold int      `json:"memory_threshold" envconfig:"CHECK_SYSTEM_MEMORY_THRESHOLD"`
	DiskThreshold   int      `json:"disk_threshold" envconfig:"CHECK_SYSTEM_DISK_THRESHOLD"`
	DiskPaths       []string `json:"disk_paths" envconfig:"CHECK_SYSTEM_DISK_PATHS"`
}

// HTTPCheck defines a single HTTP/HTTPS endpoint to monitor
type HTTPCheck struct {
	Name           string    `json:"name"`
	URL            string    `json:"url" envconfig:"CHECK_HTTP_URL"`
	Method         string    `json:"method" envconfig:"CHECK_HTTP_METHOD"`
	Timeout        int       `json:"timeout" envconfig:"CHECK_HTTP_TIMEOUT"`
	ExpectedStatus int       `json:"expected_status" envconfig:"CHECK_HTTP_EXPECTED_STATUS"`
	CheckInterval  int       `json:"check_interval" envconfig:"CHECK_HTTP_INTERVAL"`
	LastCheck      time.Time `json:"-"`
}


// AlertingConfig contains alert notification settings
type AlertingConfig struct {
	Enabled     bool           `json:"enabled"`
	Email       EmailConfig    `json:"email"`
	Mailgun     MailgunConfig  `json:"mailgun"`
	Telegram    TelegramConfig `json:"telegram"`
	AlertLevels []string       `json:"alert_levels" envconfig:"ALERTING_LEVELS"` // info, warning, critical
	Cooldown    int            `json:"cooldown" envconfig:"ALERTING_COOLDOWN"`   // minutes between repeated alerts
}

// EmailConfig contains SMTP email settings
type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	SMTPHost string `json:"smtp_host" envconfig:"ALERTING_EMAIL_SMTP_HOST"`
	SMTPPort int    `json:"smtp_port" envconfig:"ALERTING_EMAIL_SMTP_PORT"`
	Username string `json:"username" envconfig:"ALERTING_EMAIL_USERNAME"`
	Password string `json:"password" envconfig:"ALERTING_EMAIL_PASSWORD"`
	From     string `json:"from" envconfig:"ALERTING_EMAIL_FROM"`
	To       string `json:"to" envconfig:"ALERTING_EMAIL_TO"`
	UseTLS   bool   `json:"use_tls" envconfig:"ALERTING_EMAIL_USE_TLS"`
}

// MailgunConfig contains Mailgun API settings
type MailgunConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key" envconfig:"API_KEY"`
	Domain  string `json:"domain" envconfig:"DOMAIN"`
	From    string `json:"from" envconfig:"FROM"`
	To      string `json:"to" envconfig:"TO"`
	BaseURL string `json:"base_url" envconfig:"BASE_URL"` // Default: "https://api.mailgun.net/v3"
}

// TelegramConfig contains Telegram bot settings
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
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
	Enabled       bool     `json:"enabled"`
	CheckInterval int      `json:"check_interval" envconfig:"CHECK_DOCKER_INTERVAL"`
	Containers    []string `json:"containers" envconfig:"CHECK_DOCKER_CONTAINERS"` // Specific containers to monitor, empty for all
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
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port" envconfig:"HTTP_SERVER_PORT"`
	Username string `json:"username" envconfig:"HTTP_SERVER_USERNAME"`
	Password string `json:"password" envconfig:"HTTP_SERVER_PASSWORD"`
}
