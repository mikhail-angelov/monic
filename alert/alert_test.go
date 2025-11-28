package alert

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestNewAlertManager(t *testing.T) {
	config := &types.AlertingConfig{}

	manager := NewAlertManager(config, "TestApp")

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
			name: "valid email config",
			config: types.AlertingConfig{
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
			name: "valid telegram config",
			config: types.AlertingConfig{
				Telegram: types.TelegramConfig{
					Enabled:  true,
					BotToken: "test-token",
					ChatID:   "test-chat-id",
				},
			},
			expected: "",
		},
		{
			name: "no methods configured",
			config: types.AlertingConfig{},
			expected: "alerting is enabled but no alerting methods are configured",
		},
		{
			name: "email enabled but missing SMTP host",
			config: types.AlertingConfig{
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
			manager := NewAlertManager(&tt.config, "TestApp")
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
	// Since shouldSendLevel always returns true in the current implementation,
	// we'll test that it always allows sending regardless of level
	config := &types.AlertingConfig{}

	manager := NewAlertManager(config, "TestApp")

	levels := []string{"info", "warning", "critical"}
	for _, level := range levels {
		result := manager.shouldSendLevel(level)
		if !result {
			t.Errorf("Expected shouldSendLevel(%s) = true, got false", level)
		}
	}
}

func TestAlertManager_ShouldSendCooldown(t *testing.T) {
	config := &types.AlertingConfig{}

	manager := NewAlertManager(config, "TestApp")

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

	// Immediately after sending, should not send again (1 minute cooldown is hardcoded)
	if manager.shouldSendCooldown(alert) {
		t.Error("Alert should not be sent immediately after previous one")
	}
}

func TestAlertManager_SendAlert_NoMethods(t *testing.T) {
	config := &types.AlertingConfig{} // No alerting methods configured

	manager := NewAlertManager(config, "TestApp")

	alert := types.Alert{
		Type:      "test",
		Message:   "Test alert",
		Level:     "warning",
		Timestamp: time.Now(),
	}

	err := manager.SendAlert(alert)
	if err != nil {
		t.Errorf("Expected no error when no alerting methods are configured, got: %v", err)
	}
}

func TestAlertManager_BuildEmailBody(t *testing.T) {
	config := &types.AlertingConfig{}

	manager := NewAlertManager(config, "TestApp")

	alert := types.Alert{
		Type:      "cpu",
		Message:   "CPU usage is 95%",
		Level:     "critical",
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	body := manager.buildEmailBody(alert)

	expectedStrings := []string{
		"TESTAPP MONITORING ALERT",
		"Alert Level: CRITICAL",
		"Alert Type: cpu",
		"Message: CPU usage is 95%",
		"Timestamp: Wed, 01 Jan 2025 12:00:00 UTC",
		"This alert was generated by the TestApp monitoring service",
	}

	for _, expected := range expectedStrings {
		if !contains(body, expected) {
			t.Errorf("Expected email body to contain '%s', but it didn't", expected)
		}
	}
}

func TestAlertManager_SendAlerts(t *testing.T) {
	config := &types.AlertingConfig{} // No alerting methods configured

	manager := NewAlertManager(config, "TestApp")

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
		Mailgun: types.MailgunConfig{
			Enabled: true,
			APIKey:  "test-key",
			Domain:  "example.com",
			From:    "monic@example.com",
			To:      "admin@example.com",
			BaseURL: server.URL, // Use mock server URL
		},
	}

	manager := NewAlertManager(config, "TestApp")

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
		Mailgun: types.MailgunConfig{
			Enabled: true,
			APIKey:  "test-key",
			Domain:  "example.com",
			From:    "monic@example.com",
			To:      "admin@example.com",
			BaseURL: server.URL,
		},
	}

	manager := NewAlertManager(config, "TestApp")

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
