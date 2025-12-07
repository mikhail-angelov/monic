package server

import (
	"strings"
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestNewMonitorService(t *testing.T) {
	config := &types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/"},
		},
		HTTPChecks: types.HTTPCheck		{
				URL:            "http://localhost:8080/health",
				Method:         "GET",
				Timeout:        10,
				ExpectedStatus: 200,
				CheckInterval:  30,
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

	if service.storage == nil {
		t.Error("Expected storage manager to be initialized")
	}

	// Check storage is empty initially
	if service.storage.GetAlertsCount() != 0 {
		t.Error("Expected alerts to be empty initially")
	}

	if service.storage.GetSystemStatsCount() != 0 {
		t.Error("Expected stats history to be empty initially")
	}

	if service.storage.GetHTTPCheckResultsCount() != 0 {
		t.Error("Expected HTTP history to be empty initially")
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
		HTTPChecks: types.HTTPCheck{},
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
		HTTPChecks: types.HTTPCheck{},
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
		HTTPChecks: types.HTTPCheck{},
	}

	service := NewMonitorService(config)

	// Add some test alerts using storage manager
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
			Level:     "warning",
			Timestamp: time.Now(),
		},
	}
	service.storage.AddAlerts(alerts)

	// Verify alerts were added
	if service.storage.GetAlertsCount() != 2 {
		t.Errorf("Expected 2 alerts before processing, got %d", service.storage.GetAlertsCount())
	}

	// Process alerts (this would normally log them)
	service.processAlerts()

	// After processing, alerts should be cleared
	if service.storage.GetAlertsCount() != 0 {
		t.Errorf("Expected alerts to be cleared after processing, got %d", service.storage.GetAlertsCount())
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
