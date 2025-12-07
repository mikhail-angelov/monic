package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// StatsServer represents the HTTP stats server
type StatsServer struct {
	config        *types.HTTPServerConfig
	systemMonitor *monitor.SystemMonitor
	storage       *StorageManager
	stateManager  interface{} // We'll use interface{} to avoid circular dependency
	startTime     time.Time
}

// NewStatsServer creates a new stats server instance
func NewStatsServer(config *types.HTTPServerConfig, systemMonitor *monitor.SystemMonitor, storage *StorageManager, stateManager interface{}) *StatsServer {
	return &StatsServer{
		config:        config,
		systemMonitor: systemMonitor,
		storage:       storage,
		stateManager:  stateManager,
		startTime:     time.Now(),
	}
}

// Start starts the HTTP stats server
func (s *StatsServer) Start() error {
	if !s.config.Enabled {
		slog.Info("HTTP stats server is disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stats", s.basicAuth(s.handleStats))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: mux,
	}

	slog.Info("Starting HTTP stats server", "port", s.config.Port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP stats server failed", "error", err)
		}
	}()

	return nil
}

// basicAuth middleware for HTTP basic authentication
func (s *StatsServer) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no credentials are configured
		if s.config.Username == "" || s.config.Password == "" {
			next(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Monic Stats"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if username != s.config.Username || password != s.config.Password {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// handleStats handles the /stats endpoint
func (s *StatsServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.getStatsResponse()
	
	// Check if client explicitly requests JSON
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("Error encoding stats response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Otherwise serve HTML
	renderStatsHTML(w, stats)
}

// getStatsResponse builds the complete stats response
func (s *StatsServer) getStatsResponse() map[string]interface{} {
	response := make(map[string]interface{})

	// Service status
	response["service_status"] = map[string]interface{}{
		"status":     "running",
		"started_at": s.startTime.Format(time.RFC3339),
		"uptime":     time.Since(s.startTime).String(),
	}

	// System information
	systemInfo := s.systemMonitor.GetSystemInfo()
	response["system_info"] = systemInfo

	// Current system stats
	latestStats := s.storage.GetLatestSystemStats()
	if latestStats != nil {
		response["current_system_stats"] = map[string]interface{}{
			"timestamp": latestStats.Timestamp.Format(time.RFC3339),
			"cpu_usage": latestStats.CPUUsage,
			"memory_usage": map[string]interface{}{
				"total":        latestStats.MemoryUsage.Total,
				"used":         latestStats.MemoryUsage.Used,
				"free":         latestStats.MemoryUsage.Free,
				"used_percent": latestStats.MemoryUsage.UsedPercent,
			},
			"disk_usage": latestStats.DiskUsage,
		}
	} else {
		response["current_system_stats"] = nil
	}

	// HTTP checks status
	response["http_checks"] = s.getHTTPChecksStatus()

	// Alert status
	alertsCount := s.storage.GetAlertsCount()
	response["alerts"] = map[string]interface{}{
		"active_alerts": alertsCount,
		"recent_alerts": s.getRecentAlerts(),
	}

	// Monitoring thresholds (from system monitor)
	response["thresholds"] = s.systemMonitor.GetThresholds()

	return response
}

// getHTTPChecksStatus returns the status of all HTTP checks
func (s *StatsServer) getHTTPChecksStatus() []map[string]interface{} {
	var checks []map[string]interface{}

	httpHistory := s.storage.GetHTTPCheckResults()
	if len(httpHistory) == 0 {
		return checks
	}

	// Group HTTP results by name to get latest status
	latestResults := make(map[string]types.HTTPCheckResult)
	for _, result := range httpHistory {
		if existing, exists := latestResults[result.Name]; !exists || result.Timestamp.After(existing.Timestamp) {
			latestResults[result.Name] = result
		}
	}

	// Find last failure for each check
	lastFailures := make(map[string]time.Time)
	for _, result := range httpHistory {
		if !result.Success {
			if existing, exists := lastFailures[result.Name]; !exists || result.Timestamp.After(existing) {
				lastFailures[result.Name] = result.Timestamp
			}
		}
	}

	// Build response for each check
	for name, result := range latestResults {
		check := map[string]interface{}{
			"name":          name,
			"url":           result.URL,
			"status":        "success",
			"last_check":    result.Timestamp.Format(time.RFC3339),
			"response_time": result.ResponseTime.String(),
			"status_code":   result.StatusCode,
		}

		if !result.Success {
			check["status"] = "failed"
			check["error"] = result.Error
		}

		if lastFailure, exists := lastFailures[name]; exists {
			check["last_failure"] = lastFailure.Format(time.RFC3339)
		}

		checks = append(checks, check)
	}

	return checks
}

// getRecentAlerts returns recent alerts
func (s *StatsServer) getRecentAlerts() []map[string]interface{} {
	var recentAlerts []map[string]interface{}

	alerts := s.storage.GetAlerts()
	if len(alerts) == 0 {
		return recentAlerts
	}

	// Get last 10 alerts (or all if less than 10)
	start := 0
	if len(alerts) > 10 {
		start = len(alerts) - 10
	}

	for i := start; i < len(alerts); i++ {
		alert := alerts[i]
		recentAlerts = append(recentAlerts, map[string]interface{}{
			"type":      alert.Type,
			"message":   alert.Message,
			"level":     alert.Level,
			"timestamp": alert.Timestamp.Format(time.RFC3339),
		})
	}

	return recentAlerts
}
