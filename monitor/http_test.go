package monitor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestNewHTTPMonitor(t *testing.T) {
	monitor := NewHTTPMonitor()

	if monitor == nil {
		t.Fatal("Expected HTTPMonitor instance, got nil")
	}

	if monitor.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	// Verify client timeout is set
	if monitor.client.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout of 30s, got %v", monitor.client.Timeout)
	}
}

func TestHTTPMonitor_ValidateHTTPCheck(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Test valid HTTP check
	validCheck := types.HTTPCheck{
		URL:            "https://example.com",
		Method:         "GET",
		Timeout:        10,
		ExpectedStatus: 200,
		CheckInterval:  30,
	}

	err := monitor.ValidateHTTPCheck(validCheck)
	if err != nil {
		t.Errorf("Expected valid check to pass validation: %v", err)
	}

	// Test invalid checks
	tests := []struct {
		name     string
		check    types.HTTPCheck
		expected string
	}{
		{
			name:     "empty URL",
			check:    types.HTTPCheck{URL: "", Method: "GET"},
			expected: "URL cannot be empty",
		},
		{
			name:     "invalid URL scheme",
			check:    types.HTTPCheck{URL: "ftp://example.com", Method: "GET"},
			expected: "URL must start with http:// or https://",
		},
		{
			name:     "empty method",
			check:    types.HTTPCheck{URL: "https://example.com", Method: ""},
			expected: "HTTP method cannot be empty",
		},
		{
			name:     "invalid method",
			check:    types.HTTPCheck{URL: "https://example.com", Method: "INVALID"},
			expected: "invalid HTTP method: INVALID",
		},
		{
			name:     "zero timeout",
			check:    types.HTTPCheck{URL: "https://example.com", Method: "GET", Timeout: 0},
			expected: "timeout must be positive",
		},
		{
			name:     "invalid status code",
			check:    types.HTTPCheck{URL: "https://example.com", Method: "GET", Timeout: 10, ExpectedStatus: 99, CheckInterval: 30},
			expected: "expected status code must be between 100 and 599",
		},
		{
			name:     "zero check interval",
			check:    types.HTTPCheck{URL: "https://example.com", Method: "GET", Timeout: 10, ExpectedStatus: 200, CheckInterval: 0},
			expected: "check interval must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := monitor.ValidateHTTPCheck(tt.check)
			if err == nil {
				t.Error("Expected validation error, got nil")
			} else if err.Error() != tt.expected {
				t.Errorf("Expected error '%s', got '%s'", tt.expected, err.Error())
			}
		})
	}
}

func TestHTTPMonitor_CheckEndpoint_Success(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	check := types.HTTPCheck{
		URL:            server.URL,
		Method:         "GET",
		Timeout:        5,
		ExpectedStatus: 200,
		CheckInterval:  30,
	}

	result := monitor.CheckEndpoint(check)

	if !result.Success {
		t.Errorf("Expected successful check, got error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
	// Note: HTTPCheck doesn't have a Name field, so result.Name will be empty
	if result.URL != server.URL {
		t.Errorf("Expected URL '%s', got '%s'", server.URL, result.URL)
	}
	if result.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}
	if result.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestHTTPMonitor_CheckEndpoint_WrongStatusCode(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	check := types.HTTPCheck{
		URL:            server.URL,
		Method:         "GET",
		Timeout:        5,
		ExpectedStatus: 200,
		CheckInterval:  30,
	}

	result := monitor.CheckEndpoint(check)

	if result.Success {
		t.Error("Expected failed check due to wrong status code")
	}
	if result.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", result.StatusCode)
	}
	if result.Error == "" {
		t.Error("Expected error message for wrong status code")
	}
}

func TestHTTPMonitor_CheckEndpoint_ConnectionError(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Use an invalid URL that will cause connection error
	check := types.HTTPCheck{
		URL:            "http://invalid-host-that-does-not-exist.local",
		Method:         "GET",
		Timeout:        1, // Short timeout for faster test
		ExpectedStatus: 200,
		CheckInterval:  30,
	}

	result := monitor.CheckEndpoint(check)

	if result.Success {
		t.Error("Expected failed check due to connection error")
	}
	if result.Error == "" {
		t.Error("Expected error message for connection failure")
	}
}

func TestHTTPMonitor_CheckEndpoints(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	checks := []types.HTTPCheck{
		{
			URL:            server1.URL,
			Method:         "GET",
			Timeout:        5,
			ExpectedStatus: 200,
			CheckInterval:  30,
		},
		{
			URL:            server2.URL,
			Method:         "GET",
			Timeout:        5,
			ExpectedStatus: 200,
			CheckInterval:  30,
		},
	}

	results := monitor.CheckEndpoints(checks)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify first check succeeded
	if !results[0].Success {
		t.Errorf("Expected first check to succeed: %s", results[0].Error)
	}

	// Verify second check failed
	if results[1].Success {
		t.Error("Expected second check to fail")
	}
}

func TestHTTPMonitor_GetHTTPStats(t *testing.T) {
	monitor := NewHTTPMonitor()

	results := []types.HTTPCheckResult{
		{
			Name:         "test1",
			URL:          "http://example.com",
			StatusCode:   200,
			ResponseTime: 100 * time.Millisecond,
			Success:      true,
			Timestamp:    time.Now(),
		},
		{
			Name:         "test2",
			URL:          "http://example.com",
			StatusCode:   500,
			ResponseTime: 200 * time.Millisecond,
			Success:      false,
			Error:        "server error",
			Timestamp:    time.Now(),
		},
		{
			Name:         "test3",
			URL:          "http://example.com",
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			Timestamp:    time.Now(),
		},
	}

	stats := monitor.GetHTTPStats(results)

	expectedStats := map[string]interface{}{
		"total_checks":         3,
		"successful_checks":    2,
		"failed_checks":        1,
		"success_rate":         float64(2) / float64(3) * 100,
		"avg_response_time_ms": int64(150), // (100 + 200 + 150) / 3 = 150ms
		"min_response_time_ms": int64(100),
		"max_response_time_ms": int64(200),
	}

	for key, expected := range expectedStats {
		actual := stats[key]
		if actual != expected {
			t.Errorf("Stat %s: expected %v, got %v", key, expected, actual)
		}
	}
}

func TestHTTPMonitor_GetHTTPStats_Empty(t *testing.T) {
	monitor := NewHTTPMonitor()

	stats := monitor.GetHTTPStats([]types.HTTPCheckResult{})

	if len(stats) != 0 {
		t.Errorf("Expected empty stats for empty results, got %v", stats)
	}
}

func TestHTTPMonitor_CheckEndpointConcurrent(t *testing.T) {
	monitor := NewHTTPMonitor()

	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	check := types.HTTPCheck{
		URL:            server.URL,
		Method:         "GET",
		Timeout:        5,
		ExpectedStatus: 200,
		CheckInterval:  30,
	}

	result := monitor.CheckEndpointConcurrent(check)

	if !result.Success {
		t.Errorf("Expected successful check, got error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
	// Note: HTTPCheck doesn't have a Name field, so result.Name will be empty
	if result.URL != server.URL {
		t.Errorf("Expected URL '%s', got '%s'", server.URL, result.URL)
	}
	if result.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}
	if result.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}
