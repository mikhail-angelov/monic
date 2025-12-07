package server

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"bconf.com/monic/alert"
	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// MonitorService represents the main monitoring service
type MonitorService struct {
	config        *types.Config
	systemMonitor *monitor.SystemMonitor
	httpMonitor   *monitor.HTTPMonitor
	dockerMonitor *monitor.DockerMonitor
	alertManager  *alert.AlertManager
	stateManager  *alert.StateManager
	statsServer   *StatsServer
	storage       *StorageManager
	stopChan      chan struct{}
	wg            sync.WaitGroup
	startTime     time.Time
}

// NewMonitorService creates a new monitoring service instance
func NewMonitorService(config *types.Config) *MonitorService {
	service := &MonitorService{
		config:        config,
		systemMonitor: monitor.NewSystemMonitor(&config.SystemChecks),
		httpMonitor:   monitor.NewHTTPMonitor(),
		dockerMonitor: monitor.NewDockerMonitor(&config.DockerChecks),
		alertManager:  alert.NewAlertManager(&config.Alerting, config.AppName),
		stateManager:  alert.NewStateManager(),
		storage:       NewStorageManager(100),
		stopChan:      make(chan struct{}),
		startTime:     time.Now(),
	}

	// Initialize stats server
	service.statsServer = NewStatsServer(
		&config.HTTPServer,
		service.systemMonitor,
		service.storage,
		service.stateManager,
	)

	return service
}

// Start begins the monitoring service
func (ms *MonitorService) Start() error {
	slog.Info("Starting Monic monitoring service...")

	// Validate HTTP checks configuration
	if err := ms.httpMonitor.ValidateHTTPCheck(ms.config.HTTPChecks); err != nil {
		return fmt.Errorf("invalid HTTP check configuration for %s: %w", ms.config.HTTPChecks.URL, err)
	}

	// Validate alerting configuration
	if err := ms.alertManager.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid alerting configuration: %w", err)
	}

	// Start HTTP stats server
	if err := ms.statsServer.Start(); err != nil {
		return fmt.Errorf("failed to start HTTP stats server: %w", err)
	}

	// Print system information
	systemInfo := ms.systemMonitor.GetSystemInfo()
	slog.Info("System Info", "info", systemInfo)

	// Initialize Docker monitor if enabled
	if ms.config.DockerChecks.Enabled {
		if err := ms.dockerMonitor.Initialize(); err != nil {
			slog.Warn("Failed to initialize Docker monitor", "error", err)
		} else {
			ms.wg.Add(1)
			go ms.dockerMonitoringLoop()
		}
	}

	// Start monitoring goroutines
	ms.wg.Add(3)
	go ms.systemMonitoringLoop()
	go ms.httpMonitoringLoop()
	go ms.alertProcessingLoop()

	slog.Info("Monic monitoring service started successfully")
	return nil
}

// Stop gracefully stops the monitoring service
func (ms *MonitorService) Stop() {
	slog.Info("Stopping Monic monitoring service...")
	close(ms.stopChan)
	ms.wg.Wait()
	slog.Info("Monic monitoring service stopped")
}

// systemMonitoringLoop handles system resource monitoring
func (ms *MonitorService) systemMonitoringLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(time.Duration(ms.config.SystemChecks.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ms.stopChan:
			return
		case <-ticker.C:
			ms.collectSystemStats()
		}
	}
}

// httpMonitoringLoop handles HTTP endpoint monitoring
func (ms *MonitorService) httpMonitoringLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ms.stopChan:
			return
		case <-ticker.C:
			ms.collectHTTPStats()
		}
	}
}

// dockerMonitoringLoop handles Docker container monitoring
func (ms *MonitorService) dockerMonitoringLoop() {
	defer ms.wg.Done()

	interval := ms.config.DockerChecks.CheckInterval
	if interval == 0 {
		interval = 60 // Default to 60 seconds
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ms.stopChan:
			return
		case <-ticker.C:
			ms.collectDockerStats()
		}
	}
}

// alertProcessingLoop handles alert processing and reporting
func (ms *MonitorService) alertProcessingLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(60 * time.Second) // Process alerts every minute
	defer ticker.Stop()

	for {
		select {
		case <-ms.stopChan:
			return
		case <-ticker.C:
			ms.processAlerts()
		}
	}
}

// collectSystemStats collects and processes system statistics
func (ms *MonitorService) collectSystemStats() {
	stats, err := ms.systemMonitor.CollectStats()
	if err != nil {
		slog.Error("Error collecting system stats", "error", err)
		return
	}

	// Add to history (keep last 100 entries)
	ms.storage.AddSystemStats(*stats)

	// Use state manager to generate alerts with 3 consecutive failures logic
	alerts := ms.stateManager.UpdateSystemState(stats, &ms.config.SystemChecks)
	if len(alerts) > 0 {
		ms.storage.AddAlerts(alerts)
		slog.Info("System alerts generated", "count", len(alerts))
	}

	// Log current stats (in production, this would go to a proper logging system)
	slog.Info("System Stats",
		"cpu", fmt.Sprintf("%.2f%%", stats.CPUUsage),
		"memory", fmt.Sprintf("%.2f%%", stats.MemoryUsage.UsedPercent),
		"disk", ms.getDiskUsageSummary(stats.DiskUsage))
}

// collectHTTPStats collects and processes HTTP monitoring statistics
func (ms *MonitorService) collectHTTPStats() {
	result := ms.httpMonitor.CheckEndpointConcurrent(ms.config.HTTPChecks)
	results := []types.HTTPCheckResult{result}

	// Add to history (keep last 100 entries)
	ms.storage.AddHTTPCheckResult(result)

	// Use state manager to generate alerts with 3 consecutive failures logic
	alerts := ms.stateManager.UpdateHTTPState(results)
	if len(alerts) > 0 {
		ms.storage.AddAlerts(alerts)
		slog.Info("HTTP alerts generated", "count", len(alerts))
	}

	// Log HTTP stats
	httpStats := ms.httpMonitor.GetHTTPStats(results)
	slog.Info("HTTP Stats",
		"total", httpStats["total_checks"],
		"success", httpStats["successful_checks"],
		"failed", httpStats["failed_checks"],
		"rate", fmt.Sprintf("%.1f%%", httpStats["success_rate"]))
}

// collectDockerStats collects and processes Docker container statistics
func (ms *MonitorService) collectDockerStats() {
	stats, err := ms.dockerMonitor.CheckContainers()
	if err != nil {
		slog.Error("Error collecting Docker stats", "error", err)
		return
	}

	// Add to history (keep last 100 entries)
	ms.storage.AddDockerContainerStats(stats)

	// Check for container status alerts
	alerts, err := ms.dockerMonitor.CheckContainerStatus()
	if err != nil {
		slog.Error("Error checking Docker container status", "error", err)
	} else if len(alerts) > 0 {
		ms.storage.AddAlerts(alerts)
		slog.Info("Docker alerts generated", "count", len(alerts))
	}

	// Log Docker stats
	summary := ms.dockerMonitor.GetContainerSummary(stats)
	slog.Info("Docker Stats",
		"total", summary["total_containers"],
		"running", summary["running_containers"],
		"stopped", summary["stopped_containers"],
		"percentage", fmt.Sprintf("%.1f%%", summary["running_percentage"]))
}

// processAlerts processes and reports alerts
func (ms *MonitorService) processAlerts() {
	alerts := ms.storage.GetAlerts()
	if len(alerts) == 0 {
		return
	}

	// Log alerts to console
	for _, alert := range alerts {
		slog.Info("ALERT", "level", alert.Level, "type", alert.Type, "message", alert.Message)
	}

	// Send alerts via configured channels (email, Mailgun, etc.)
	if err := ms.alertManager.SendAlerts(alerts); err != nil {
		slog.Error("Failed to send some alerts", "error", err)
	}

	// Clear processed alerts
	ms.storage.ClearAlerts()
}

// getDiskUsageSummary creates a summary of disk usage
func (ms *MonitorService) getDiskUsageSummary(diskUsage map[string]types.DiskStats) string {
	var summary []string
	for path, stats := range diskUsage {
		summary = append(summary, fmt.Sprintf("%s:%.1f%%", path, stats.UsedPercent))
	}
	return fmt.Sprintf("[%s]", stringJoin(summary, ", "))
}


// stringJoin is a helper function to join strings
func stringJoin(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	n := len(sep) * (len(elems) - 1)
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}

	var b []byte
	b = make([]byte, n)
	bp := copy(b, elems[0])
	for _, s := range elems[1:] {
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], s)
	}
	return string(b)
}
