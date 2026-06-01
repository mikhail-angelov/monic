package monitor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"bconf.com/monic/types"
)

// HealthCheckRegistry manages per-container health check goroutines.
// Each container with monic.check=http gets its own goroutine that periodically
// pings the configured URL.
type HealthCheckRegistry struct {
	httpMon    *HTTPMonitor
	results    chan types.HTTPCheckResult
	containers map[string]context.CancelFunc // containerID -> cancel

	mu sync.RWMutex
}

// NewHealthCheckRegistry creates a new health check registry.
func NewHealthCheckRegistry(httpMon *HTTPMonitor) *HealthCheckRegistry {
	return &HealthCheckRegistry{
		httpMon:    httpMon,
		results:    make(chan types.HTTPCheckResult, 128),
		containers: make(map[string]context.CancelFunc),
	}
}

// Results returns a channel of HTTP check results.
func (r *HealthCheckRegistry) Results() <-chan types.HTTPCheckResult {
	return r.results
}

// Add starts health checks for a monitored container.
// If the container only has status monitoring (check=container), no goroutine is started.
func (r *HealthCheckRegistry) Add(ctx context.Context, mc types.MonitoredContainer) {
	if mc.CheckType != types.CheckTypeHTTP || mc.CheckHTTPURL == "" {
		slog.Debug("No HTTP health check needed for container",
			"name", mc.Name, "check_type", mc.CheckType)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop existing check if somehow already running
	if cancel, exists := r.containers[mc.ID]; exists {
		cancel()
	}

	checkCtx, cancel := context.WithCancel(ctx)
	r.containers[mc.ID] = cancel

	target := types.HTTPCheckTarget{
		ContainerID:   mc.ID,
		Name:          mc.CustomName,
		URL:           mc.CheckHTTPURL,
		Method:        "GET",
		Timeout:       mc.CheckHTTPTimeout,
		ExpectedCode:  mc.CheckHTTPExpectedCode,
		CheckInterval: mc.CheckHTTPInterval,
	}

	interval := time.Duration(target.CheckInterval) * time.Second

	go r.runHealthCheck(checkCtx, target, interval)

	slog.Info("Started HTTP health check for container",
		"name", target.Name, "url", target.URL, "interval", interval)
}

// Remove stops health checks for a container.
func (r *HealthCheckRegistry) Remove(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cancel, exists := r.containers[containerID]; exists {
		cancel()
		delete(r.containers, containerID)
		slog.Debug("Stopped health check for container", "id", containerID[:12])
	}
}

// RemoveAll stops all health checks.
func (r *HealthCheckRegistry) RemoveAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, cancel := range r.containers {
		cancel()
		delete(r.containers, id)
	}
}

// runHealthCheck periodically performs an HTTP health check for a target.
func (r *HealthCheckRegistry) runHealthCheck(ctx context.Context, target types.HTTPCheckTarget, interval time.Duration) {
	// Do an immediate first check
	r.doCheck(target)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.doCheck(target)
		}
	}
}

// doCheck performs a single HTTP health check and sends the result to the channel.
func (r *HealthCheckRegistry) doCheck(target types.HTTPCheckTarget) {
	check := types.HTTPCheck{
		URL:            target.URL,
		Method:         target.Method,
		Timeout:        target.Timeout,
		ExpectedStatus: target.ExpectedCode,
		CheckInterval:  target.CheckInterval,
	}

	result := r.httpMon.CheckEndpoint(check)
	result.Name = target.Name

	select {
	case r.results <- result:
	default:
		slog.Warn("Health check result channel full, dropping result",
			"name", target.Name)
	}
}

// HTTPMonitor handles HTTP/HTTPS endpoint monitoring
type HTTPMonitor struct {
	client *http.Client
}

// NewHTTPMonitor creates a new HTTP monitor instance
func NewHTTPMonitor() *HTTPMonitor {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &HTTPMonitor{
		client: client,
	}
}

// CheckEndpoint performs a single HTTP/HTTPS health check.
func (hm *HTTPMonitor) CheckEndpoint(check types.HTTPCheck) types.HTTPCheckResult {
	result := types.HTTPCheckResult{
		URL:       check.URL,
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(check.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(check.Method), check.URL, http.NoBody)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Success = false
		return result
	}

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

	_, err = io.CopyN(io.Discard, resp.Body, 1024)
	if err != nil && !errors.Is(err, io.EOF) {
		result.Error = fmt.Sprintf("failed to read response body: %v", err)
		result.Success = false
		return result
	}

	result.StatusCode = resp.StatusCode

	if resp.StatusCode == check.ExpectedStatus {
		result.Success = true
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("unexpected status code: %d (expected: %d)", resp.StatusCode, check.ExpectedStatus)
	}

	return result
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
func (hm *HTTPMonitor) GetHTTPStats(results []types.HTTPCheckResult) map[string]any {
	stats := make(map[string]any)

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
