package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	configData := types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/", "/tmp"},
		},
		HTTPChecks: []types.HTTPCheck{
			{
				Name:           "test",
				URL:            "http://localhost:8080/health",
				Method:         "GET",
				Timeout:        10,
				ExpectedStatus: 200,
				CheckInterval:  30,
			},
		},
	}

	// Write config to temporary file
	configFile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(configFile.Name())

	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(configData); err != nil {
		t.Fatalf("Failed to write config to file: %v", err)
	}
	configFile.Close()

	// Test loading the config
	loadedConfig, err := loadConfig(configFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches expected values
	if loadedConfig.SystemChecks.Interval != 30 {
		t.Errorf("Expected monitoring interval 30, got %d", loadedConfig.SystemChecks.Interval)
	}
	if loadedConfig.SystemChecks.CPUThreshold != 80 {
		t.Errorf("Expected CPU threshold 80, got %d", loadedConfig.SystemChecks.CPUThreshold)
	}
	if len(loadedConfig.SystemChecks.DiskPaths) != 2 {
		t.Errorf("Expected 2 disk paths, got %d", len(loadedConfig.SystemChecks.DiskPaths))
	}
	if len(loadedConfig.HTTPChecks) != 1 {
		t.Errorf("Expected 1 HTTP check, got %d", len(loadedConfig.HTTPChecks))
	}
	if loadedConfig.HTTPChecks[0].Name != "test" {
		t.Errorf("Expected HTTP check name 'test', got '%s'", loadedConfig.HTTPChecks[0].Name)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("/non/existent/path/config.json")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	// Create a temporary file with invalid JSON
	configFile, err := os.CreateTemp("", "test-invalid-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(configFile.Name())

	// Write invalid JSON
	configFile.WriteString("{ invalid json }")
	configFile.Close()

	_, err = loadConfig(configFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON config")
	}
}

func TestNewMonitorService(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: []types.HTTPCheck{
			{
				Name:           "test",
				URL:            "http://localhost:8080/health",
				Method:         "GET",
				Timeout:        10,
				ExpectedStatus: 200,
				CheckInterval:  30,
			},
		},
	}

	service := NewMonitorService(config)

	if service == nil {
		t.Fatal("Expected MonitorService instance, got nil")
	}

	if service.config != config {
		t.Error("Expected config to be set correctly")
	}

	if service.systemMonitor == nil {
		t.Error("Expected system monitor to be initialized")
	}

	if service.httpMonitor == nil {
		t.Error("Expected HTTP monitor to be initialized")
	}

	if service.stopChan == nil {
		t.Error("Expected stop channel to be initialized")
	}

	if len(service.alerts) != 0 {
		t.Error("Expected alerts to be empty initially")
	}

	if len(service.statsHistory) != 0 {
		t.Error("Expected stats history to be empty initially")
	}

	if len(service.httpHistory) != 0 {
		t.Error("Expected HTTP history to be empty initially")
	}
}

func TestMonitorService_GetStatus(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: []types.HTTPCheck{},
	}

	service := NewMonitorService(config)

	status := service.GetStatus()

	// Verify basic status structure
	if status["service"] != "running" {
		t.Errorf("Expected service status 'running', got '%s'", status["service"])
	}

	if status["system_info"] == nil {
		t.Error("Expected system info in status")
	}

	if status["active_alerts"] == nil {
		t.Error("Expected active alerts count in status")
	}
}

func TestMonitorService_GetDiskUsageSummary(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: []types.HTTPCheck{},
	}

	service := NewMonitorService(config)

	diskUsage := map[string]types.DiskStats{
		"/": {
			Path:        "/",
			UsedPercent: 25.5,
		},
		"/tmp": {
			Path:        "/tmp",
			UsedPercent: 10.2,
		},
	}

	summary := service.getDiskUsageSummary(diskUsage)

	// Since map iteration order is random, we need to check that both entries are present
	// rather than expecting a specific order
	expectedEntries := []string{"/:25.5%", "/tmp:10.2%"}
	for _, entry := range expectedEntries {
		if !contains(summary, entry) {
			t.Errorf("Expected disk summary to contain '%s', got '%s'", entry, summary)
		}
	}

	// Also verify the format starts and ends with brackets
	if !strings.HasPrefix(summary, "[") || !strings.HasSuffix(summary, "]") {
		t.Errorf("Expected disk summary to be in format [entry1, entry2], got '%s'", summary)
	}
}

func TestMonitorService_GetDiskUsageSummary_Empty(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: []types.HTTPCheck{},
	}

	service := NewMonitorService(config)

	diskUsage := map[string]types.DiskStats{}

	summary := service.getDiskUsageSummary(diskUsage)

	expected := "[]"
	if summary != expected {
		t.Errorf("Expected disk summary '%s', got '%s'", expected, summary)
	}
}

func TestStringJoin(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		sep      string
		expected string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			sep:      ", ",
			expected: "",
		},
		{
			name:     "single element",
			input:    []string{"hello"},
			sep:      ", ",
			expected: "hello",
		},
		{
			name:     "multiple elements",
			input:    []string{"a", "b", "c"},
			sep:      ", ",
			expected: "a, b, c",
		},
		{
			name:     "different separator",
			input:    []string{"a", "b", "c"},
			sep:      "-",
			expected: "a-b-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringJoin(tt.input, tt.sep)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestMonitorService_ProcessAlerts(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: []types.HTTPCheck{},
	}

	service := NewMonitorService(config)

	// Add some test alerts
	service.alerts = []types.Alert{
		{
			Type:      "cpu",
			Message:   "CPU usage high",
			Level:     "warning",
			Timestamp: time.Now(),
		},
		{
			Type:      "memory",
			Message:   "Memory usage high",
			Level:     "warning",
			Timestamp: time.Now(),
		},
	}

	// Process alerts (this would normally log them)
	service.processAlerts()

	// After processing, alerts should be cleared
	if len(service.alerts) != 0 {
		t.Errorf("Expected alerts to be cleared after processing, got %d", len(service.alerts))
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
