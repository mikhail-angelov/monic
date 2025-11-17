package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

func TestStatsServer_HandleStats(t *testing.T) {
	config := &types.HTTPServerConfig{
		Enabled:  true,
		Port:     8080,
		Username: "admin",
		Password: "monic123",
	}

	systemMonitor := monitor.NewSystemMonitor(&types.SystemChecksConfig{
		DiskPaths: []string{"/"},
		Interval:  60,
	})

	statsHistory := []types.SystemStats{
		{
			Timestamp: time.Now(),
			CPUUsage:  25.5,
			MemoryUsage: types.MemoryStats{
				Total:       8192,
				Used:        2048,
				Free:        6144,
				UsedPercent: 25.0,
			},
			DiskUsage: map[string]types.DiskStats{
				"/": {
					Path:        "/",
					Total:       1000000,
					Used:        250000,
					Free:        750000,
					UsedPercent: 25.0,
				},
			},
		},
	}

	httpHistory := []types.HTTPCheckResult{
		{
			Name:         "test_service",
			URL:          "http://localhost:8080/health",
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			Timestamp:    time.Now(),
		},
	}

	alerts := []types.Alert{
		{
			Type:      "cpu",
			Message:   "CPU usage high",
			Level:     "warning",
			Timestamp: time.Now(),
		},
	}

	server := NewStatsServer(config, systemMonitor, &statsHistory, &httpHistory, &alerts, nil)

	// Create a test request
	req := httptest.NewRequest("GET", "/stats", nil)
	req.SetBasicAuth("admin", "monic123")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Call the handler directly
	server.handleStats(w, req)

	// Check the response status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Check the content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}

	// Parse the response body
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify response structure
	expectedKeys := []string{"service_status", "system_info", "current_system_stats", "http_checks", "alerts", "thresholds"}
	for _, key := range expectedKeys {
		if _, exists := response[key]; !exists {
			t.Errorf("Expected key '%s' in response", key)
		}
	}

	// Verify service status
	serviceStatus, ok := response["service_status"].(map[string]interface{})
	if !ok {
		t.Error("Expected service_status to be a map")
	}
	if serviceStatus["status"] != "running" {
		t.Errorf("Expected service status 'running', got '%s'", serviceStatus["status"])
	}

	// Verify HTTP checks
	httpChecks, ok := response["http_checks"].([]interface{})
	if !ok {
		t.Error("Expected http_checks to be an array")
	}
	if len(httpChecks) != 1 {
		t.Errorf("Expected 1 HTTP check, got %d", len(httpChecks))
	}

	// Verify alerts
	alertsData, ok := response["alerts"].(map[string]interface{})
	if !ok {
		t.Error("Expected alerts to be a map")
	}
	if int(alertsData["active_alerts"].(float64)) != 1 {
		t.Errorf("Expected 1 active alert, got %v", alertsData["active_alerts"])
	}
}

func TestStatsServer_BasicAuth(t *testing.T) {
	config := &types.HTTPServerConfig{
		Enabled:  true,
		Port:     8080,
		Username: "admin",
		Password: "monic123",
	}

	server := NewStatsServer(config, nil, nil, nil, nil, nil)

	// Test without authentication
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	server.basicAuth(server.handleStats)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d for missing auth, got %d", http.StatusUnauthorized, w.Code)
	}

	// Test with wrong credentials
	req = httptest.NewRequest("GET", "/stats", nil)
	req.SetBasicAuth("admin", "wrongpassword")
	w = httptest.NewRecorder()

	server.basicAuth(server.handleStats)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d for wrong credentials, got %d", http.StatusUnauthorized, w.Code)
	}

	// Test with correct credentials
	req = httptest.NewRequest("GET", "/stats", nil)
	req.SetBasicAuth("admin", "monic123")
	w = httptest.NewRecorder()

	server.basicAuth(server.handleStats)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d for correct credentials, got %d", http.StatusOK, w.Code)
	}
}

func TestStatsServer_NoAuthWhenDisabled(t *testing.T) {
	config := &types.HTTPServerConfig{
		Enabled: true,
		Port:    8080,
		// No username/password configured
	}

	server := NewStatsServer(config, nil, nil, nil, nil, nil)

	// Test without authentication when no credentials are configured
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	server.basicAuth(server.handleStats)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d when no auth configured, got %d", http.StatusOK, w.Code)
	}
}

func TestStatsServer_MethodNotAllowed(t *testing.T) {
	config := &types.HTTPServerConfig{
		Enabled: true,
		Port:    8080,
	}

	server := NewStatsServer(config, nil, nil, nil, nil, nil)

	// Test with POST method (should be rejected)
	req := httptest.NewRequest("POST", "/stats", nil)
	w := httptest.NewRecorder()

	server.handleStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d for POST method, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
