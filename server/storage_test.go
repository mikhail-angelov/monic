package server

import (
	"sync"
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestStorageManager_ConcurrentAccess(t *testing.T) {
	storage := NewStorageManager(100)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Add system stats
				storage.AddSystemStats(types.SystemStats{
					Timestamp: time.Now(),
					CPUUsage:  float64(id*numOperations + j),
					MemoryUsage: types.MemoryStats{
						Total:       8192,
						Used:        2048,
						Free:        6144,
						UsedPercent: 25.0,
					},
				})

				// Add HTTP check result
				storage.AddHTTPCheckResult(types.HTTPCheckResult{
					Name:         "test",
					URL:          "http://localhost:8080/health",
					StatusCode:   200,
					ResponseTime: time.Duration(j) * time.Millisecond,
					Success:      true,
					Timestamp:    time.Now(),
				})

				// Add alert
				storage.AddAlert(types.Alert{
					Type:      "test",
					Message:   "Test alert",
					Level:     "warning",
					Timestamp: time.Now(),
				})

				// Read operations
				_ = storage.GetAlerts()
				_ = storage.GetSystemStats()
				_ = storage.GetHTTPCheckResults()
				_ = storage.GetAlertsCount()
			}
		}(i)
	}

	wg.Wait()

	// Note: Storage trims to maxHistorySize (100), so we can't check exact counts
	// But we can verify no panic occurred and storage is in valid state
	alertsCount := storage.GetAlertsCount()

	// Check that storage size doesn't exceed maxHistorySize
	if alertsCount > 100 {
		t.Errorf("Expected alerts count <= 100 due to trimming, got %d", alertsCount)
	}

	// Verify storage is in valid state
	status := storage.GetStatus()
	if status["alerts_count"] == nil || status["stats_history_count"] == nil || status["http_history_count"] == nil {
		t.Error("Expected status to contain all counts")
	}
}

func TestStorageManager_ClearAlerts(t *testing.T) {
	storage := NewStorageManager(100)

	// Add some alerts
	for i := 0; i < 5; i++ {
		storage.AddAlert(types.Alert{
			Type:      "test",
			Message:   "Test alert",
			Level:     "warning",
			Timestamp: time.Now(),
		})
	}

	if storage.GetAlertsCount() != 5 {
		t.Errorf("Expected 5 alerts, got %d", storage.GetAlertsCount())
	}

	// Clear alerts
	storage.ClearAlerts()

	if storage.GetAlertsCount() != 0 {
		t.Errorf("Expected 0 alerts after clear, got %d", storage.GetAlertsCount())
	}
}

func TestStorageManager_GetLatestSystemStats(t *testing.T) {
	storage := NewStorageManager(100)

	// Add multiple stats
	for i := 0; i < 5; i++ {
		storage.AddSystemStats(types.SystemStats{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			CPUUsage:  float64(i * 10),
			MemoryUsage: types.MemoryStats{
				Total:       8192,
				Used:        2048,
				Free:        6144,
				UsedPercent: 25.0,
			},
		})
	}

	latest := storage.GetLatestSystemStats()
	if latest == nil {
		t.Fatal("Expected latest stats, got nil")
	}

	// Latest should have CPUUsage = 40 (4 * 10)
	if latest.CPUUsage != 40.0 {
		t.Errorf("Expected latest CPU usage 40.0, got %f", latest.CPUUsage)
	}
}

func TestStorageManager_GetLatestHTTPCheckResult(t *testing.T) {
	storage := NewStorageManager(100)

	// Add HTTP results for different names
	storage.AddHTTPCheckResult(types.HTTPCheckResult{
		Name:      "service1",
		URL:       "http://service1/health",
		Success:   true,
		Timestamp: time.Now().Add(-10 * time.Second),
	})

	storage.AddHTTPCheckResult(types.HTTPCheckResult{
		Name:      "service2",
		URL:       "http://service2/health",
		Success:   true,
		Timestamp: time.Now().Add(-5 * time.Second),
	})

	storage.AddHTTPCheckResult(types.HTTPCheckResult{
		Name:      "service1",
		URL:       "http://service1/health",
		Success:   false,
		Timestamp: time.Now(),
	})

	// Get latest for service1
	latest := storage.GetLatestHTTPCheckResult("service1")
	if latest == nil {
		t.Fatal("Expected latest HTTP check result for service1, got nil")
	}

	if latest.Success != false {
		t.Errorf("Expected latest service1 check to be unsuccessful, got success=%v", latest.Success)
	}

	// Get latest for service2
	latest2 := storage.GetLatestHTTPCheckResult("service2")
	if latest2 == nil {
		t.Fatal("Expected latest HTTP check result for service2, got nil")
	}

	if latest2.Success != true {
		t.Errorf("Expected latest service2 check to be successful, got success=%v", latest2.Success)
	}

	// Get latest for non-existent service
	latest3 := storage.GetLatestHTTPCheckResult("service3")
	if latest3 != nil {
		t.Error("Expected nil for non-existent service")
	}
}
