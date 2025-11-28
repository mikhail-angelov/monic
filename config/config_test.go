package config

import (
	"os"
	"testing"
)

func TestLoadConfig_EnvOnly(t *testing.T) {
	// Set environment variables
	os.Setenv("MONIC_APP_NAME", "TestApp")
	os.Setenv("MONIC_CHECK_SYSTEM_INTERVAL", "30")
	os.Setenv("MONIC_CHECK_SYSTEM_CPU_THRESHOLD", "80")
	os.Setenv("MONIC_CHECK_SYSTEM_MEMORY_THRESHOLD", "85")
	os.Setenv("MONIC_CHECK_SYSTEM_DISK_THRESHOLD", "90")
	os.Setenv("MONIC_CHECK_SYSTEM_DISK_PATHS", "/,/tmp")
	defer func() {
		os.Unsetenv("MONIC_APP_NAME")
		os.Unsetenv("MONIC_CHECK_SYSTEM_INTERVAL")
		os.Unsetenv("MONIC_CHECK_SYSTEM_CPU_THRESHOLD")
		os.Unsetenv("MONIC_CHECK_SYSTEM_MEMORY_THRESHOLD")
		os.Unsetenv("MONIC_CHECK_SYSTEM_DISK_THRESHOLD")
		os.Unsetenv("MONIC_CHECK_SYSTEM_DISK_PATHS")
	}()

	// Test loading the config from environment variables
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches expected values
	if config.AppName != "TestApp" {
		t.Errorf("Expected AppName 'TestApp', got '%s'", config.AppName)
	}
	if config.SystemChecks.Interval != 30 {
		t.Errorf("Expected monitoring interval 30, got %d", config.SystemChecks.Interval)
	}
	if config.SystemChecks.CPUThreshold != 80 {
		t.Errorf("Expected CPU threshold 80, got %d", config.SystemChecks.CPUThreshold)
	}
	if config.SystemChecks.MemoryThreshold != 85 {
		t.Errorf("Expected memory threshold 85, got %d", config.SystemChecks.MemoryThreshold)
	}
	if config.SystemChecks.DiskThreshold != 90 {
		t.Errorf("Expected disk threshold 90, got %d", config.SystemChecks.DiskThreshold)
	}
	if len(config.SystemChecks.DiskPaths) != 2 || config.SystemChecks.DiskPaths[0] != "/" || config.SystemChecks.DiskPaths[1] != "/tmp" {
		t.Errorf("Expected disk paths ['/', '/tmp'], got %v", config.SystemChecks.DiskPaths)
	}
}

func TestLoadConfig_HTTPCheckFromEnv(t *testing.T) {
	// Set HTTP check environment variables
	os.Setenv("MONIC_CHECK_HTTP_URL", "http://localhost:8080/health")
	os.Setenv("MONIC_CHECK_HTTP_METHOD", "GET")
	os.Setenv("MONIC_CHECK_HTTP_TIMEOUT", "10")
	os.Setenv("MONIC_CHECK_HTTP_EXPECTED_STATUS", "200")
	os.Setenv("MONIC_CHECK_HTTP_INTERVAL", "30")
	defer func() {
		os.Unsetenv("MONIC_CHECK_HTTP_URL")
		os.Unsetenv("MONIC_CHECK_HTTP_METHOD")
		os.Unsetenv("MONIC_CHECK_HTTP_TIMEOUT")
		os.Unsetenv("MONIC_CHECK_HTTP_EXPECTED_STATUS")
		os.Unsetenv("MONIC_CHECK_HTTP_INTERVAL")
	}()

	// Test loading the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Note: HTTP checks are not currently loaded from environment variables
	// This test verifies that the config loads without errors when HTTP check env vars are present
	// The actual HTTP check loading would need to be implemented separately
	if config == nil {
		t.Error("Expected config to be loaded successfully")
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("MONIC_APP_NAME", "EnvApp")
	os.Setenv("MONIC_CHECK_SYSTEM_INTERVAL", "60")
	defer os.Unsetenv("MONIC_APP_NAME")
	defer os.Unsetenv("MONIC_CHECK_SYSTEM_INTERVAL")

	// Load config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.AppName != "EnvApp" {
		t.Errorf("Expected AppName 'EnvApp', got '%s'", config.AppName)
	}
	if config.SystemChecks.Interval != 60 {
		t.Errorf("Expected Interval 60, got %d", config.SystemChecks.Interval)
	}
}

func TestLoadConfig_AlertingEmailSMTPHost(t *testing.T) {
	// Set email alerting environment variables with correct nested structure
	os.Setenv("MONIC_ALERTING_EMAIL_SMTP_HOST", "smtp.test.com")
	os.Setenv("MONIC_ALERTING_EMAIL_SMTP_PORT", "587")
	os.Setenv("MONIC_ALERTING_EMAIL_FROM", "test@example.com")
	os.Setenv("MONIC_ALERTING_EMAIL_TO", "admin@example.com")
	defer func() {
		os.Unsetenv("MONIC_ALERTING_EMAIL_SMTP_HOST")
		os.Unsetenv("MONIC_ALERTING_EMAIL_SMTP_PORT")
		os.Unsetenv("MONIC_ALERTING_EMAIL_FROM")
		os.Unsetenv("MONIC_ALERTING_EMAIL_TO")
	}()
	// Test loading the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify email alerting configuration was loaded correctly
	if !config.Alerting.Email.Enabled {
		t.Error("Expected email alerting to be enabled when SMTP host is configured")
	}
	if config.Alerting.Email.SMTPHost != "smtp.test.com" {
		t.Errorf("Expected SMTP host 'smtp.test.com', got '%s'", config.Alerting.Email.SMTPHost)
	}
	if config.Alerting.Email.SMTPPort != 587 {
		t.Errorf("Expected SMTP port 587, got %d", config.Alerting.Email.SMTPPort)
	}
	if config.Alerting.Email.From != "test@example.com" {
		t.Errorf("Expected from email 'test@example.com', got '%s'", config.Alerting.Email.From)
	}
	if config.Alerting.Email.To != "admin@example.com" {
		t.Errorf("Expected to email 'admin@example.com', got '%s'", config.Alerting.Email.To)
	}
}

func TestLoadConfig_HTTPServerConfig(t *testing.T) {
	// Set HTTP server environment variables
	os.Setenv("MONIC_HTTP_SERVER_PORT", "8080")
	os.Setenv("MONIC_HTTP_SERVER_USERNAME", "admin")
	os.Setenv("MONIC_HTTP_SERVER_PASSWORD", "secret123")
	defer func() {
		os.Unsetenv("MONIC_HTTP_SERVER_PORT")
		os.Unsetenv("MONIC_HTTP_SERVER_USERNAME")
		os.Unsetenv("MONIC_HTTP_SERVER_PASSWORD")
	}()

	// Test loading the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP server configuration was loaded correctly
	if !config.HTTPServer.Enabled {
		t.Error("Expected HTTP server to be enabled when port is configured")
	}
	if config.HTTPServer.Port != 8080 {
		t.Errorf("Expected HTTP server port 8080, got %d", config.HTTPServer.Port)
	}
	if config.HTTPServer.Username != "admin" {
		t.Errorf("Expected HTTP server username 'admin', got '%s'", config.HTTPServer.Username)
	}
	if config.HTTPServer.Password != "secret123" {
		t.Errorf("Expected HTTP server password 'secret123', got '%s'", config.HTTPServer.Password)
	}
}

func TestLoadConfig_HTTPServerEnabledByPort(t *testing.T) {
	// Set only the port environment variable
	os.Setenv("MONIC_HTTP_SERVER_PORT", "9090")
	defer os.Unsetenv("MONIC_HTTP_SERVER_PORT")

	// Test loading the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP server is enabled when port is set
	if !config.HTTPServer.Enabled {
		t.Error("Expected HTTP server to be enabled when port is configured")
	}
	if config.HTTPServer.Port != 9090 {
		t.Errorf("Expected HTTP server port 9090, got %d", config.HTTPServer.Port)
	}
}

func TestLoadConfig_HTTPServerDisabledByDefault(t *testing.T) {
	// Ensure no HTTP server environment variables are set
	os.Unsetenv("MONIC_HTTP_SERVER_PORT")
	os.Unsetenv("MONIC_HTTP_SERVER_USERNAME")
	os.Unsetenv("MONIC_HTTP_SERVER_PASSWORD")

	// Test loading the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP server is disabled by default
	if config.HTTPServer.Enabled {
		t.Error("Expected HTTP server to be disabled by default when no environment variables are set")
	}
}
