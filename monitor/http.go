package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bconf.com/monic/types"
)

// HTTPMonitor handles HTTP/HTTPS endpoint monitoring
type HTTPMonitor struct {
	client *http.Client
}

// NewHTTPMonitor creates a new HTTP monitor instance
func NewHTTPMonitor() *HTTPMonitor {
	// Create a custom HTTP client with timeouts and TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false, // Verify SSL certificates
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Default timeout
	}

	return &HTTPMonitor{
		client: client,
	}
}

// CheckEndpoint performs a single HTTP/HTTPS check
func (hm *HTTPMonitor) CheckEndpoint(check types.HTTPCheck) types.HTTPCheckResult {
	result := types.HTTPCheckResult{
		URL:       check.URL,
		Timestamp: time.Now(),
	}

	// Create request with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(check.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(check.Method), check.URL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Success = false
		return result
	}

	// Set common headers
	req.Header.Set("User-Agent", "Monic-Monitor/1.0")
	req.Header.Set("Accept", "*/*")

	startTime := time.Now()
	resp, err := hm.client.Do(req)
	responseTime := time.Since(startTime)

	result.ResponseTime = responseTime

	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Success = false
		return result
	}
	defer resp.Body.Close()

	// Read a small portion of the response body to ensure connection is working
	_, err = io.CopyN(io.Discard, resp.Body, 1024) // Read up to 1KB
	if err != nil && err != io.EOF {
		result.Error = fmt.Sprintf("failed to read response body: %v", err)
		result.Success = false
		return result
	}

	result.StatusCode = resp.StatusCode

	// Check if status code matches expected
	if resp.StatusCode == check.ExpectedStatus {
		result.Success = true
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("unexpected status code: %d (expected: %d)", resp.StatusCode, check.ExpectedStatus)
	}

	return result
}

// CheckEndpoints performs checks on multiple HTTP endpoints
func (hm *HTTPMonitor) CheckEndpoints(checks []types.HTTPCheck) []types.HTTPCheckResult {
	var results []types.HTTPCheckResult

	for _, check := range checks {
		// Skip if it's too soon to check again
		if !check.LastCheck.IsZero() {
			timeSinceLastCheck := time.Since(check.LastCheck)
			if timeSinceLastCheck < time.Duration(check.CheckInterval)*time.Second {
				continue
			}
		}

		result := hm.CheckEndpoint(check)
		results = append(results, result)
	}

	return results
}

// CheckEndpointsConcurrent performs HTTP checks concurrently for better performance
func (hm *HTTPMonitor) CheckEndpointsConcurrent(checks []types.HTTPCheck) []types.HTTPCheckResult {
	results := make([]types.HTTPCheckResult, 0, len(checks))
	resultChan := make(chan types.HTTPCheckResult, len(checks))

	// Launch goroutines for each check
	for _, check := range checks {
		go func(c types.HTTPCheck) {
			result := hm.CheckEndpoint(c)
			resultChan <- result
		}(check)
	}

	// Collect results
	for i := 0; i < len(checks); i++ {
		result := <-resultChan
		results = append(results, result)
	}

	return results
}

// CheckEndpointConcurrent performs a single HTTP check concurrently
func (hm *HTTPMonitor) CheckEndpointConcurrent(check types.HTTPCheck) types.HTTPCheckResult {
	resultChan := make(chan types.HTTPCheckResult, 1)

	go func(c types.HTTPCheck) {
		result := hm.CheckEndpoint(c)
		resultChan <- result
	}(check)

	return <-resultChan
}

// ValidateHTTPCheck validates if an HTTP check configuration is valid
func (hm *HTTPMonitor) ValidateHTTPCheck(check types.HTTPCheck) error {
	
	if check.URL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if !strings.HasPrefix(check.URL, "http://") && !strings.HasPrefix(check.URL, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}

	if check.Method == "" {
		return fmt.Errorf("HTTP method cannot be empty")
	}

	method := strings.ToUpper(check.Method)
	validMethods := map[string]bool{
		"GET":     true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"HEAD":    true,
		"OPTIONS": true,
		"PATCH":   true,
	}

	if !validMethods[method] {
		return fmt.Errorf("invalid HTTP method: %s", check.Method)
	}

	if check.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if check.ExpectedStatus < 100 || check.ExpectedStatus >= 600 {
		return fmt.Errorf("expected status code must be between 100 and 599")
	}

	if check.CheckInterval <= 0 {
		return fmt.Errorf("check interval must be positive")
	}

	return nil
}

// GetHTTPStats returns statistics about HTTP monitoring performance
func (hm *HTTPMonitor) GetHTTPStats(results []types.HTTPCheckResult) map[string]interface{} {
	stats := make(map[string]interface{})

	if len(results) == 0 {
		return stats
	}

	var totalResponseTime time.Duration
	successCount := 0
	failedCount := 0
	minResponseTime := results[0].ResponseTime
	maxResponseTime := results[0].ResponseTime

	for _, result := range results {
		totalResponseTime += result.ResponseTime

		if result.Success {
			successCount++
		} else {
			failedCount++
		}

		if result.ResponseTime < minResponseTime {
			minResponseTime = result.ResponseTime
		}
		if result.ResponseTime > maxResponseTime {
			maxResponseTime = result.ResponseTime
		}
	}

	avgResponseTime := totalResponseTime / time.Duration(len(results))

	stats["total_checks"] = len(results)
	stats["successful_checks"] = successCount
	stats["failed_checks"] = failedCount
	stats["success_rate"] = float64(successCount) / float64(len(results)) * 100
	stats["avg_response_time_ms"] = avgResponseTime.Milliseconds()
	stats["min_response_time_ms"] = minResponseTime.Milliseconds()
	stats["max_response_time_ms"] = maxResponseTime.Milliseconds()

	return stats
}
