package server

import (
	"strings"
	"testing"
	"time"

	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// testDigestStorage is a simplified in-memory storage for digest tests.
type testDigestStorage struct {
	stats       []types.SystemStats
	alerts      []types.Alert
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


func (s *testDigestStorage) ClearAlerts() {
	s.alerts = nil
}

// testDigestSystemMonitor is a stub for system monitor.
type testDigestSystemMonitor struct{}

func (m *testDigestSystemMonitor) GetSystemInfo() map[string]any {
	return map[string]any{"hostname": "test-host"}
}

func (m *testDigestSystemMonitor) CollectStats() (*types.SystemStats, error) {
	return &types.SystemStats{}, nil
}

func (m *testDigestSystemMonitor) GetThresholds() map[string]any {
	return map[string]any{
		"cpu_threshold":    float64(90),
		"memory_threshold": float64(85),
		"disk_threshold":   float64(90),
	}
}

func TestBuildDigest_WithData(t *testing.T) {
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

	sm := &testDigestSystemMonitor{}
	ds := NewDigestService(storage, sm, nil, "Monic")

	result := ds.BuildDigest()

	// Check header
	if !strings.Contains(result, "Monic Daily Digest") {
		t.Error("Expected header 'Monic Daily Digest'")
	}

	// Check Docker disabled message
	if !strings.Contains(result, "Docker monitoring is disabled") {
		t.Error("Expected 'Docker monitoring is disabled'")
	}

	// Check container incidents section
	if !strings.Contains(result, "Failures: 1") {
		t.Errorf("Expected 'Failures: 1', got: %s", result)
	}
	if !strings.Contains(result, "Recoveries: 1") {
		t.Errorf("Expected 'Recoveries: 1', got: %s", result)
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
		t.Error("Expected 'peak 95.0%' in digest")
	}
	if !strings.Contains(result, "current 45.0%") {
		t.Error("Expected 'current 45.0%' in digest")
	}

	// Check thresholds
	if !strings.Contains(result, "CPU: > 90%") {
		t.Error("Expected threshold 'CPU: > 90%' in digest")
	}

	t.Logf("Digest output:\n%s", result)
}

func TestBuildDigest_NoData(t *testing.T) {
	storage := &testDigestStorage{}
	sm := &testDigestSystemMonitor{}
	ds := NewDigestService(storage, sm, nil, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "No HTTP checks configured") {
		t.Errorf("Expected 'No HTTP checks configured', got: %s", result)
	}
	if !strings.Contains(result, "No system stats recorded yet") {
		t.Errorf("Expected 'No system stats recorded yet', got: %s", result)
	}
}

func TestBuildDigest_WithRecovery(t *testing.T) {
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
	sm := &testDigestSystemMonitor{}
	ds := NewDigestService(storage, sm, nil, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "Failures: 1") {
		t.Errorf("Expected failures: 1, got: %s", result)
	}
	if !strings.Contains(result, "Recoveries: 1") {
		t.Errorf("Expected recoveries: 1, got: %s", result)
	}
}

func TestBuildDigest_WithContainerTracker(t *testing.T) {
	// Test with a real ContainerTracker — container summary should appear
	ct := monitor.NewContainerTracker()
	// Seed some container data
	ct.UpdateFromEvent("abc123", "redis", "My Redis", true, "container")
	ct.UpdateFromEvent("def456", "nginx", "", false, "http")

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
	sm := &testDigestSystemMonitor{}
	ds := NewDigestService(storage, sm, ct, "Monic")

	result := ds.BuildDigest()

	if !strings.Contains(result, "📊 MONITORED CONTAINERS") {
		t.Errorf("Expected container section, got: %s", result)
	}
	if !strings.Contains(result, "Total: 2") {
		t.Errorf("Expected Total: 2, got: %s", result)
	}
	if !strings.Contains(result, "Running: 1") {
		t.Errorf("Expected Running: 1, got: %s", result)
	}
	if !strings.Contains(result, "Stopped: 1") {
		t.Errorf("Expected Stopped: 1, got: %s", result)
	}
	t.Logf("Digest output:\n%s", result)
}
