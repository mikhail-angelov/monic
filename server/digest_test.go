package server

import (
	"strings"
	"testing"
	"time"

	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// testStorage is a simplified in-memory storage for digest tests.
type testDigestStorage struct {
	stats []types.SystemStats
	alerts []types.Alert
	httpResults []types.HTTPCheckResult
}

func (s *testDigestStorage) GetLatestSystemStats() *types.SystemStats {
	if len(s.stats) == 0 {
		return nil
	}
	return &s.stats[len(s.stats)-1]
}

func (s *testDigestStorage) GetSystemStats() []types.SystemStats {
	return s.stats
}

func (s *testDigestStorage) GetAlertsCount() int {
	return len(s.alerts)
}

func (s *testDigestStorage) GetAlerts() []types.Alert {
	return s.alerts
}

func (s *testDigestStorage) GetHTTPCheckResults() []types.HTTPCheckResult {
	return s.httpResults
}

func (s *testDigestStorage) AddSystemStats(stats types.SystemStats) {
	s.stats = append(s.stats, stats)
}

func (s *testDigestStorage) AddAlert(alert types.Alert) {
	s.alerts = append(s.alerts, alert)
}

func (s *testDigestStorage) AddAlerts(alerts []types.Alert) {
	s.alerts = append(s.alerts, alerts...)
}

func (s *testDigestStorage) AddHTTPCheckResult(result types.HTTPCheckResult) {
	s.httpResults = append(s.httpResults, result)
}

func (s *testDigestStorage) AddDockerContainerStats(stats []types.DockerContainerStats) {
}

func (s *testDigestStorage) ClearAlerts() {
	s.alerts = nil
}

// testSystemMonitor is a stub for system monitor.
type testSystemMonitor struct{}

func (m *testSystemMonitor) GetSystemInfo() map[string]any {
	return map[string]any{"hostname": "test-host"}
}

func (m *testSystemMonitor) CollectStats() (*types.SystemStats, error) {
	return &types.SystemStats{}, nil
}

func (m *testSystemMonitor) GetThresholds() map[string]any {
	return map[string]any{
		"cpu_threshold":    float64(90),
		"memory_threshold": float64(85),
		"disk_threshold":   float64(90),
	}
}

func TestBuildDigest_WithContainerData(t *testing.T) {
	ct := monitor.NewContainerTracker()
	_ = ct.UpdateFromEvent("abc123", "nginx", "", true, "container")
	_ = ct.UpdateFromEvent("def456", "redis", "my-redis", false, "container")

	storage := &testDigestStorage{
		alerts: []types.Alert{
			{Type: "docker", Level: "critical", Message: "Container redis stopped", Timestamp: time.Now().Add(-1 * time.Hour)},
			{Type: "docker", Level: "info", Message: "Container redis recovered (now running)", Timestamp: time.Now().Add(-30 * time.Minute)},
			{Type: "system", Level: "critical", Message: "CPU usage is 95%", Timestamp: time.Now().Add(-2 * time.Hour)},
		},
		stats: []types.SystemStats{
			{
				Timestamp:   time.Now().Add(-2 * time.Hour),
				CPUUsage:    95.0,
				MemoryUsage: types.MemoryStats{UsedPercent: 70.0},
				DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 55.0}},
			},
			{
				Timestamp:   time.Now(),
				CPUUsage:    45.0,
				MemoryUsage: types.MemoryStats{UsedPercent: 50.0},
				DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 55.0}},
			},
		},
		httpResults: []types.HTTPCheckResult{
			{Name: "web", URL: "https://example.com", Success: true, Timestamp: time.Now().Add(-5 * time.Minute)},
			{Name: "web", URL: "https://example.com", Success: false, Timestamp: time.Now().Add(-10 * time.Minute)},
			{Name: "web", URL: "https://example.com", Success: true, Timestamp: time.Now().Add(-15 * time.Minute)},
		},
	}

	sm := &testSystemMonitor{}
	ds := NewDigestService(ct, storage, sm, "Monic")

	result := ds.BuildDigest()

	// Check header
	if !strings.Contains(result, "Monic Daily Digest") {
		t.Error("Expected header 'Monic Daily Digest'")
	}

	// Check container summary
	if !strings.Contains(result, "Total: 2") {
		t.Error("Expected 'Total: 2'")
	}
	if !strings.Contains(result, "Running: 1") {
		t.Error("Expected 'Running: 1'")
	}
	if !strings.Contains(result, "Stopped: 1") {
		t.Error("Expected 'Stopped: 1'")
	}

	// Check container incidents section
	if !strings.Contains(result, "Failures: 1") {
		t.Error("Expected 'Failures: 1' (1 docker critical alert)")
	}

	// Check HTTP checks section
	if !strings.Contains(result, "HTTP CHECKS") {
		t.Error("Expected 'HTTP CHECKS' section")
	}
	if !strings.Contains(result, "Passed: 2") {
		t.Errorf("Expected 'Passed: 2', got: %s", result)
	}

	// Check system health section
	if !strings.Contains(result, "SYSTEM HEALTH") {
		t.Error("Expected 'SYSTEM HEALTH' section")
	}
	if !strings.Contains(result, "peak 95.0%") {
		t.Errorf("Expected 'peak 95.0%', got: %s", result)
	}
	if !strings.Contains(result, "current 45.0%") {
		t.Errorf("Expected 'current 45.0%', got: %s", result)
	}

	// Check thresholds are listed
	if !strings.Contains(result, "CPU: > 90%") {
		t.Errorf("Expected threshold 'CPU: > 90%', got: %s", result)
	}

	t.Logf("Digest output:\n%s", result)
}

func TestBuildDigest_NoContainers(t *testing.T) {
	ct := monitor.NewContainerTracker()

	storage := &testDigestStorage{
		stats: []types.SystemStats{
			{
				Timestamp:   time.Now(),
				CPUUsage:    30.0,
				MemoryUsage: types.MemoryStats{UsedPercent: 40.0},
				DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 50.0}},
			},
		},
	}

	sm := &testSystemMonitor{}
	ds := NewDigestService(ct, storage, sm, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "Total: 0") {
		t.Errorf("Expected 'Total: 0', got: %s", result)
	}
	if !strings.Contains(result, "No HTTP checks configured") {
		t.Errorf("Expected 'No HTTP checks configured', got: %s", result)
	}
}

func TestBuildDigest_WithRecovery(t *testing.T) {
	ct := monitor.NewContainerTracker()
	// Add container that stopped, then recovered
	_ = ct.UpdateFromEvent("abc123", "nginx", "", true, "container")

	storage := &testDigestStorage{
		alerts: []types.Alert{
			{Type: "docker", Level: "critical", Message: "Container nginx stopped", Timestamp: time.Now().Add(-6 * time.Hour)},
			{Type: "docker", Level: "info", Message: "Container nginx recovered (now running)", Timestamp: time.Now().Add(-5 * time.Hour)},
		},
		stats: []types.SystemStats{
			{
				Timestamp:   time.Now(),
				CPUUsage:    25.0,
				MemoryUsage: types.MemoryStats{UsedPercent: 35.0},
				DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 45.0}},
			},
		},
	}

	sm := &testSystemMonitor{}
	ds := NewDigestService(ct, storage, sm, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "Failures: 1") {
		t.Errorf("Expected failures: 1, got: %s", result)
	}
	if !strings.Contains(result, "Recoveries: 1") {
		t.Errorf("Expected recoveries: 1, got: %s", result)
	}
}

func TestBuildDigest_NilContainerTracker(t *testing.T) {
	storage := &testDigestStorage{
		stats: []types.SystemStats{
			{
				Timestamp:   time.Now(),
				CPUUsage:    30.0,
				MemoryUsage: types.MemoryStats{UsedPercent: 40.0},
				DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 50.0}},
			},
		},
	}

	sm := &testSystemMonitor{}
	ds := NewDigestService(nil, storage, sm, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "Docker monitoring is disabled") {
		t.Errorf("Expected 'Docker monitoring is disabled', got: %s", result)
	}
}

func TestFormatDigestForAlert(t *testing.T) {
	alert := FormatDigestForAlert("test digest body")
	if alert.Type != "digest" {
		t.Errorf("Expected type 'digest', got '%s'", alert.Type)
	}
	if alert.Level != "info" {
		t.Errorf("Expected level 'info', got '%s'", alert.Level)
	}
	if alert.Message != "test digest body" {
		t.Errorf("Expected message 'test digest body', got '%s'", alert.Message)
	}
}
