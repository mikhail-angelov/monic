package alert

import (
	"bconf.com/monic/v2/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAlertManager(t *testing.T) {
	config := &types.AlertingConfig{
		Enabled:     true,
		AlertLevels: []string{"warning", "critical"},
		Cooldown:    30,
	}

	manager := NewAlertManager(config)

	if manager == nil {
		t.Fatal("Expected AlertManager instance, got nil")
	}

	if manager.config != config {
		t.Error("Expected config to be set correctly")
	}

	if len(manager.lastSent) != 0 {
		t.Error("Expected lastSent map to be empty initially")
	}
}

func TestAlertManager_ValidateConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   types.AlertingConfig
		expected string
	}{
		{
			name: "valid disabled config",
			config: types.AlertingConfig{
				Enabled: false,
			},
			expected: "",
		},
		{
			name: "valid email config",
			config: types.AlertingConfig{
				Enabled: true,
				Email: types.EmailConfig{
					Enabled:  true,
					SMTPHost: "smtp.example.com",
					SMTPPort: 587,
					From:     "monic@example.com",
					To:       "admin@example.com",
				},
			},
			expected: "",
		},
		{
			name: "valid mailgun config",
			config: types.AlertingConfig{
				Enabled: true,
				Mailgun: types.MailgunConfig{
					Enabled: true,
					APIKey:  "test-key",
					Domain:  "example.com",
					From:    "monic@example.com",
					To:      "admin@example.com",
				},
			},
			expected: "",
		},
		{
			name: "enabled but no methods configured",
			config: types.AlertingConfig{
				Enabled: true,
			},
			expected: "alerting is enabled but no alerting methods are configured",
		},
		{
			name: "email enabled but missing SMTP host",
			config: types.AlertingConfig{
				Enabled: true,
				Email: types.EmailConfig{
					Enabled:  true,
					SMTPPort: 587,
					From:     "monic@example.com",
					To:       "admin@example.com",
				},
			},
			expected: "SMTP host is required for email alerts",
		},
		{
			name: "email enabled but missing from address",
			config: types.AlertingConfig{
				Enabled: true,
				Email: types.EmailConfig{
					Enabled:  true,
					SMTPHost: "smtp.example.com",
					SMTPPort: 587,
					To:       "admin@example.com",
				},
			},
			expected: "from email address is required",
		},
		{
			name: "mailgun enabled but missing API key",
			config: types.AlertingConfig{
				Enabled: true,
				Mailgun: types.MailgunConfig{
					Enabled: true,
					Domain:  "example.com",
					From:    "monic@example.com",
					To:      "admin@example.com",
				},
			},
			expected: "API key is required for Mailgun alerts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewAlertManager(&tt.config)
			err := manager.ValidateConfig()

			if tt.expected == "" {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error '%s', got nil", tt.expected)
				} else if err.Error() != tt.expected {
					t.Errorf("Expected error '%s', got '%s'", tt.expected, err.Error())
				}
			}
		})
	}
}

func TestAlertManager_ShouldSendLevel(t *testing.T) {
	tests := []struct {
		name     string
		config   types.AlertingConfig
		level    string
		expected bool
	}{
		{
			name: "no levels configured - send all",
			config: types.AlertingConfig{
				AlertLevels: []string{},
			},
			level:    "info",
			expected: true,
		},
		{
			name: "level in configured list",
			config: types.AlertingConfig{
				AlertLevels: []string{"warning", "critical"},
			},
			level:    "warning",
			expected: true,
		},
		{
			name: "level not in configured list",
			config: types.AlertingConfig{
				AlertLevels: []string{"warning", "critical"},
			},
			level:    "info",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewAlertManager(&tt.config)
			result := manager.shouldSendLevel(tt.level)

			if result != tt.expected {
				t.Errorf("Expected shouldSendLevel(%s) = %v, got %v", tt.level, tt.expected, result)
			}
		})
	}
}

func TestAlertManager_ShouldSendCooldown(t *testing.T) {
	config := &types.AlertingConfig{
		Cooldown: 5, // 5 minutes
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "cpu",
		Message:   "CPU usage high",
		Level:     "warning",
		Timestamp: time.Now(),
	}

	// First alert should always be sent
	if !manager.shouldSendCooldown(alert) {
		t.Error("First alert should be sent (no cooldown)")
	}

	// Mark as sent
	manager.lastSent[alert.Type] = time.Now()

	// Immediately after sending, should not send again
	if manager.shouldSendCooldown(alert) {
		t.Error("Alert should not be sent immediately after previous one")
	}

	// Test with no cooldown configured
	configNoCooldown := &types.AlertingConfig{
		Cooldown: 0,
	}
	managerNoCooldown := NewAlertManager(configNoCooldown)
	managerNoCooldown.lastSent[alert.Type] = time.Now()

	if !managerNoCooldown.shouldSendCooldown(alert) {
		t.Error("Alert should be sent when no cooldown is configured")
	}
}

func TestAlertManager_SendAlert_Disabled(t *testing.T) {
	config := &types.AlertingConfig{
		Enabled: false,
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "test",
		Message:   "Test alert",
		Level:     "warning",
		Timestamp: time.Now(),
	}

	err := manager.SendAlert(alert)
	if err != nil {
		t.Errorf("Expected no error when alerting is disabled, got: %v", err)
	}
}

func TestAlertManager_SendAlert_LevelFiltered(t *testing.T) {
	config := &types.AlertingConfig{
		Enabled:     true,
		AlertLevels: []string{"critical"}, // Only send critical alerts
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "test",
		Message:   "Test alert",
		Level:     "warning", // This should be filtered out
		Timestamp: time.Now(),
	}

	err := manager.SendAlert(alert)
	if err != nil {
		t.Errorf("Expected no error when alert level is filtered, got: %v", err)
	}
}

func TestAlertManager_BuildEmailBody(t *testing.T) {
	config := &types.AlertingConfig{
		Enabled: true,
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "cpu",
		Message:   "CPU usage is 95%",
		Level:     "critical",
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	body := manager.buildEmailBody(alert)

	expectedStrings := []string{
		"MONIC MONITORING ALERT",
		"Alert Level: CRITICAL",
		"Alert Type: cpu",
		"Message: CPU usage is 95%",
		"Timestamp: Wed, 01 Jan 2025 12:00:00 UTC",
		"This alert was generated by the Monic monitoring service",
	}

	for _, expected := range expectedStrings {
		if !contains(body, expected) {
			t.Errorf("Expected email body to contain '%s', but it didn't", expected)
		}
	}
}

func TestAlertManager_SendAlerts(t *testing.T) {
	config := &types.AlertingConfig{
		Enabled: false, // Disabled to avoid actual sending
	}

	manager := NewAlertManager(config)

	alerts := []types.Alert{
		{
			Type:      "cpu",
			Message:   "CPU usage high",
			Level:     "warning",
			Timestamp: time.Now(),
		},
		{
			Type:      "memory",
			Message:   "Memory usage high",
			Level:     "critical",
			Timestamp: time.Now(),
		},
	}

	err := manager.SendAlerts(alerts)
	if err != nil {
		t.Errorf("Expected no error when sending multiple alerts, got: %v", err)
	}
}

// Test helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestAlertManager_SendMailgun_MockServer(t *testing.T) {
	// Create a mock Mailgun server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "api" || password != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "test-id", "message": "Queued. Thank you."}`))
	}))
	defer server.Close()

	config := &types.AlertingConfig{
		Enabled: true,
		Mailgun: types.MailgunConfig{
			Enabled: true,
			APIKey:  "test-key",
			Domain:  "example.com",
			From:    "monic@example.com",
			To:      "admin@example.com",
			BaseURL: server.URL, // Use mock server URL
		},
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "test",
		Message:   "Test alert",
		Level:     "warning",
		Timestamp: time.Now(),
	}

	err := manager.sendMailgun(alert)
	if err != nil {
		t.Errorf("Expected no error from mock Mailgun server, got: %v", err)
	}
}

func TestAlertManager_SendMailgun_Error(t *testing.T) {
	// Create a mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &types.AlertingConfig{
		Enabled: true,
		Mailgun: types.MailgunConfig{
			Enabled: true,
			APIKey:  "test-key",
			Domain:  "example.com",
			From:    "monic@example.com",
			To:      "admin@example.com",
			BaseURL: server.URL,
		},
	}

	manager := NewAlertManager(config)

	alert := types.Alert{
		Type:      "test",
		Message:   "Test alert",
		Level:     "warning",
		Timestamp: time.Now(),
	}

	err := manager.sendMailgun(alert)
	if err == nil {
		t.Error("Expected error from mock Mailgun server, got nil")
	}
}
