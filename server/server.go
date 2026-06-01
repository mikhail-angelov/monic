package server

import (
	"context"
	"crypto/subtle"
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
	config         *types.HTTPServerConfig
	systemMonitor  *monitor.SystemMonitor
	storage        Storage
	startTime      time.Time
	containerTrack *monitor.ContainerTracker
	httpServer     *http.Server // nil when disabled
}

// NewStatsServer creates a new stats server instance
func NewStatsServer(config *types.HTTPServerConfig, systemMonitor *monitor.SystemMonitor, storage Storage, containerTrack *monitor.ContainerTracker) *StatsServer {
	return &StatsServer{
		config:         config,
		systemMonitor:  systemMonitor,
		storage:        storage,
		startTime:      time.Now(),
		containerTrack: containerTrack,
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

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.config.Port),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	slog.Info("Starting HTTP stats server", "port", s.config.Port)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP stats server failed", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP stats server.
func (s *StatsServer) Stop() {
	if s.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP stats server shutdown error", "error", err)
	}
}

// basicAuth middleware for HTTP basic authentication
func (s *StatsServer) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Constant-time comparison to prevent timing attacks
		userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(s.config.Username))
		passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.config.Password))
		if userMatch != 1 || passMatch != 1 {
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

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("Error encoding stats response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	renderStatsHTML(w, stats)
}

func (s *StatsServer) getStatsResponse() map[string]any {
	response := make(map[string]any)

	response["service_status"] = map[string]any{
		"status":     "running",
		"started_at": s.startTime.Format(time.RFC3339),
		"uptime":     time.Since(s.startTime).String(),
	}

	response["system_info"] = s.systemMonitor.GetSystemInfo()

	latestStats := s.storage.GetLatestSystemStats()
	if latestStats != nil {
		response["current_system_stats"] = map[string]any{
			"timestamp": latestStats.Timestamp.Format(time.RFC3339),
			"cpu_usage": latestStats.CPUUsage,
			"memory_usage": map[string]any{
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

	response["http_checks"] = s.getHTTPChecksStatus()

	response["alerts"] = map[string]any{
		"active_alerts": s.storage.GetAlertsCount(),
		"recent_alerts": s.getRecentAlerts(),
	}

	response["thresholds"] = s.systemMonitor.GetThresholds()

	if s.containerTrack != nil {
		response["containers"] = map[string]any{
			"summary": s.containerTrack.GetSummary(),
			"list":    s.containerTrack.GetContainerStatuses(),
		}
	}

	return response
}

func (s *StatsServer) getHTTPChecksStatus() []map[string]any {
	httpHistory := s.storage.GetHTTPCheckResults()
	if len(httpHistory) == 0 {
		return []map[string]any{}
	}

	latestResults := make(map[string]types.HTTPCheckResult)
	for _, result := range httpHistory {
		if existing, exists := latestResults[result.Name]; !exists || result.Timestamp.After(existing.Timestamp) {
			latestResults[result.Name] = result
		}
	}

	lastFailures := make(map[string]time.Time)
	for _, result := range httpHistory {
		if !result.Success {
			if existing, exists := lastFailures[result.Name]; !exists || result.Timestamp.After(existing) {
				lastFailures[result.Name] = result.Timestamp
			}
		}
	}

	checks := make([]map[string]any, 0, len(latestResults))
	for name, result := range latestResults {
		check := map[string]any{
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

func (s *StatsServer) getRecentAlerts() []map[string]any {
	alerts := s.storage.GetAlerts()
	if len(alerts) == 0 {
		return nil
	}

	start := 0
	if len(alerts) > 10 {
		start = len(alerts) - 10
	}

	recent := make([]map[string]any, 0, len(alerts)-start)
	for _, alert := range alerts[start:] {
		recent = append(recent, map[string]any{
			"type":      alert.Type,
			"message":   alert.Message,
			"level":     alert.Level,
			"timestamp": alert.Timestamp.Format(time.RFC3339),
		})
	}
	return recent
}
