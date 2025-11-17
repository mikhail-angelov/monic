package monitor

import (
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestNewSystemMonitor(t *testing.T) {
	config := &types.SystemChecksConfig{
		DiskPaths: []string{"/", "/tmp"},
		Interval:  60,
	}

	monitor := NewSystemMonitor(config)

	if monitor == nil {
		t.Fatal("Expected SystemMonitor instance, got nil")
	}

	if monitor.config != config {
		t.Error("Expected config to be set correctly")
	}
}

func TestSystemMonitor_CollectStats(t *testing.T) {
	config := &types.SystemChecksConfig{
		DiskPaths: []string{"/"},
		Interval:  60,
	}

	monitor := NewSystemMonitor(config)

	stats, err := monitor.CollectStats()
	if err != nil {
		t.Fatalf("Failed to collect stats: %v", err)
	}

	// Basic validation of collected stats
	if stats.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// CPU usage should be between 0 and 100 (or could be slightly higher on some systems)
	if stats.CPUUsage < 0 || stats.CPUUsage > 200 {
		t.Errorf("CPU usage out of expected range: %f", stats.CPUUsage)
	}

	// Memory usage validation
	if stats.MemoryUsage.Total == 0 {
		t.Error("Expected total memory to be non-zero")
	}
	if stats.MemoryUsage.UsedPercent < 0 || stats.MemoryUsage.UsedPercent > 100 {
		t.Errorf("Memory usage percentage out of range: %f", stats.MemoryUsage.UsedPercent)
	}

	// Disk usage validation
	if len(stats.DiskUsage) == 0 {
		t.Error("Expected at least one disk usage entry")
	}

	for path, diskStats := range stats.DiskUsage {
		if path == "" {
			t.Error("Disk path should not be empty")
		}
		if diskStats.UsedPercent < 0 || diskStats.UsedPercent > 100 {
			t.Errorf("Disk usage percentage out of range for %s: %f", path, diskStats.UsedPercent)
		}
	}
}

func TestSystemMonitor_CheckThresholds(t *testing.T) {
	config := &types.SystemChecksConfig{
		DiskPaths: []string{"/"},
		Interval:  60,
	}

	monitor := NewSystemMonitor(config)

	// Create test stats that exceed thresholds
	stats := &types.SystemStats{
		Timestamp: time.Now(),
		CPUUsage:  85.0, // Above 80% threshold
		MemoryUsage: types.MemoryStats{
			UsedPercent: 90.0, // Above 85% threshold
		},
		DiskUsage: map[string]types.DiskStats{
			"/": {
				Path:        "/",
				UsedPercent: 95.0, // Above 90% threshold
			},
		},
	}

	thresholds := &types.SystemChecksConfig{
		CPUThreshold:    80,
		MemoryThreshold: 85,
		DiskThreshold:   90,
	}

	alerts := monitor.CheckThresholds(stats, thresholds)

	// Should generate 3 alerts (CPU, Memory, Disk)
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts, got %d", len(alerts))
	}

	// Verify alert types
	alertTypes := make(map[string]bool)
	for _, alert := range alerts {
		alertTypes[alert.Type] = true
	}

	expectedTypes := []string{"cpu", "memory", "disk"}
	for _, expectedType := range expectedTypes {
		if !alertTypes[expectedType] {
			t.Errorf("Expected alert type %s not found", expectedType)
		}
	}

	// Test with stats below thresholds
	stats.CPUUsage = 50.0
	stats.MemoryUsage.UsedPercent = 60.0
	// Create a new disk usage map for below-threshold test
	stats.DiskUsage = map[string]types.DiskStats{
		"/": {
			Path:        "/",
			UsedPercent: 70.0, // Below 90% threshold
		},
	}

	alerts = monitor.CheckThresholds(stats, thresholds)
	if len(alerts) != 0 {
		t.Errorf("Expected no alerts for stats below thresholds, got %d", len(alerts))
	}
}

func TestSystemMonitor_GetSystemInfo(t *testing.T) {
	config := &types.SystemChecksConfig{
		DiskPaths: []string{"/"},
		Interval:  60,
	}

	monitor := NewSystemMonitor(config)

	info := monitor.GetSystemInfo()

	// Basic validation of system info
	if info["host"] == nil {
		t.Error("Expected host information")
	}

	if info["runtime"] == nil {
		t.Error("Expected runtime information")
	}

	// Validate runtime info structure
	runtimeInfo, ok := info["runtime"].(map[string]interface{})
	if !ok {
		t.Error("Expected runtime info to be a map")
	}

	expectedRuntimeKeys := []string{"go_version", "num_cpu", "goroutines", "go_max_procs"}
	for _, key := range expectedRuntimeKeys {
		if runtimeInfo[key] == nil {
			t.Errorf("Expected runtime info to contain key: %s", key)
		}
	}
}

func TestSystemMonitor_InvalidDiskPath(t *testing.T) {
	config := &types.SystemChecksConfig{
		DiskPaths: []string{"/invalid/path/that/does/not/exist"},
		Interval:  60,
	}

	monitor := NewSystemMonitor(config)

	stats, err := monitor.CollectStats()
	if err != nil {
		t.Fatalf("Failed to collect stats despite invalid disk path: %v", err)
	}

	// Should still collect other stats even if disk path is invalid
	if stats.CPUUsage < 0 {
		t.Error("Should still collect CPU stats")
	}
	if stats.MemoryUsage.Total == 0 {
		t.Error("Should still collect memory stats")
	}
	// Disk usage for invalid path should be empty or not present
	if len(stats.DiskUsage) > 0 {
		t.Log("Note: Disk usage collected despite invalid path (this might be system-dependent)")
	}
}
