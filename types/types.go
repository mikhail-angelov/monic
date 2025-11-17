package types

import "time"

// Config represents the main configuration structure
type Config struct {
	AppName      string             `json:"app_name"`
	SystemChecks SystemChecksConfig `json:"system_checks"`
	HTTPChecks   []HTTPCheck        `json:"http_checks"`
	Alerting     AlertingConfig     `json:"alerting"`
	DockerChecks DockerConfig       `json:"docker_checks"`
	HTTPServer   HTTPServerConfig   `json:"http_server"`
}

// SystemChecksConfig contains system monitoring settings
type SystemChecksConfig struct {
	Interval        int      `json:"interval"`
	CPUThreshold    int      `json:"cpu_threshold"`
	MemoryThreshold int      `json:"memory_threshold"`
	DiskThreshold   int      `json:"disk_threshold"`
	DiskPaths       []string `json:"disk_paths"`
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
	Level string `json:"level"`
	File  string `json:"file"`
}

// AlertingConfig contains alert notification settings
type AlertingConfig struct {
	Enabled     bool          `json:"enabled"`
	Email       EmailConfig   `json:"email"`
	Mailgun     MailgunConfig `json:"mailgun"`
	AlertLevels []string      `json:"alert_levels"` // info, warning, critical
	Cooldown    int           `json:"cooldown"`     // minutes between repeated alerts
}

// EmailConfig contains SMTP email settings
type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
	UseTLS   bool   `json:"use_tls"`
}

// MailgunConfig contains Mailgun API settings
type MailgunConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key"`
	Domain  string `json:"domain"`
	From    string `json:"from"`
	To      string `json:"to"`
	BaseURL string `json:"base_url"` // Default: "https://api.mailgun.net/v3"
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
	CheckInterval int      `json:"check_interval"`
	Containers    []string `json:"containers"` // Specific containers to monitor, empty for all
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
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}
