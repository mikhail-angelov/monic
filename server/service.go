package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"bconf.com/monic/alert"
	"bconf.com/monic/discovery"
	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// MonitorService represents the main monitoring service
type MonitorService struct {
	config         *types.Config
	systemMonitor  *monitor.SystemMonitor
	httpMonitor    *monitor.HTTPMonitor
	alertManager   *alert.Manager
	stateManager   *alert.StateManager
	statsServer    *StatsServer
	storage        Storage
	stopChan       chan struct{}
	wg             sync.WaitGroup
	startTime      time.Time

	// New components
	dockerWatcher   *discovery.Watcher
	healthRegistry  *monitor.HealthCheckRegistry
	containerTrack  *monitor.ContainerTracker
}

// NewMonitorService creates a new monitoring service instance with injected dependencies
func NewMonitorService(
	config *types.Config,
	systemMonitor *monitor.SystemMonitor,
	httpMonitor *monitor.HTTPMonitor,
	alertManager *alert.Manager,
	stateManager *alert.StateManager,
	storage Storage,
	statsServer *StatsServer,
) *MonitorService {
	return &MonitorService{
		config:        config,
		systemMonitor: systemMonitor,
		httpMonitor:   httpMonitor,
		alertManager:  alertManager,
		stateManager:  stateManager,
		storage:       storage,
		statsServer:   statsServer,
		stopChan:      make(chan struct{}),
		startTime:     time.Now(),

		containerTrack: monitor.NewContainerTracker(),
	}
}

// SetDockerWatcher sets the Docker watcher (called after Docker client init).
func (ms *MonitorService) SetDockerWatcher(w *discovery.Watcher) {
	ms.dockerWatcher = w
}

// SetHealthRegistry sets the health check registry.
func (ms *MonitorService) SetHealthRegistry(r *monitor.HealthCheckRegistry) {
	ms.healthRegistry = r
}

// Start begins the monitoring service
func (ms *MonitorService) Start() error {
	slog.Info("Starting Monic monitoring service...")

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

	// Start monitoring goroutines
	ms.wg.Add(3)
	go ms.systemMonitoringLoop()
	go ms.dockerWatcherLoop()
	go ms.healthCheckLoop()

	slog.Info("Monic monitoring service started successfully")
	return nil
}

// Stop gracefully stops the monitoring service
func (ms *MonitorService) Stop() {
	slog.Info("Stopping Monic monitoring service...")

	if ms.dockerWatcher != nil {
		ms.dockerWatcher.Stop()
	}
	if ms.healthRegistry != nil {
		ms.healthRegistry.RemoveAll()
	}

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

// dockerWatcherLoop runs the Docker discovery watcher and processes container events.
func (ms *MonitorService) dockerWatcherLoop() {
	defer ms.wg.Done()

	if ms.dockerWatcher == nil {
		slog.Warn("Docker watcher not initialized, skipping discovery")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Process events in a goroutine
	eventDone := make(chan struct{})
	go func() {
		defer close(eventDone)
		for {
			select {
			case evt, ok := <-ms.dockerWatcher.Events():
				if !ok {
					return
				}
				ms.handleContainerEvent(ctx, evt)
			case <-ms.stopChan:
				return
			}
		}
	}()

	// Start the watcher (blocks until stop)
	if err := ms.dockerWatcher.Start(ctx); err != nil {
		if err != context.Canceled {
			slog.Error("Docker watcher stopped unexpectedly", "error", err)
		}
	}

	<-eventDone
}

// handleContainerEvent processes a single container discovery event.
func (ms *MonitorService) handleContainerEvent(ctx context.Context, evt discovery.ContainerEvent) {
	mc := evt.Container

	switch evt.Type {
	case discovery.EventAdded:
		// Start tracking
		alerts := ms.containerTrack.UpdateFromEvent(mc.ID, mc.Name, mc.CustomName, mc.Running, mc.CheckType)
		ms.processAlertsIfAny(alerts)

		// Start health checks if needed
		if ms.healthRegistry != nil {
			ms.healthRegistry.Add(ctx, mc)
		}

	case discovery.EventUpdated:
		// Update tracking
		alerts := ms.containerTrack.UpdateFromEvent(mc.ID, mc.Name, mc.CustomName, mc.Running, mc.CheckType)
		ms.processAlertsIfAny(alerts)

		// Restart health check if parameters changed
		if ms.healthRegistry != nil && mc.CheckType == types.CheckTypeHTTP {
			ms.healthRegistry.Remove(mc.ID)
			ms.healthRegistry.Add(ctx, mc)
		}

	case discovery.EventRemoved:
		// Stop tracking
		alerts := ms.containerTrack.Remove(mc.ID)
		ms.processAlertsIfAny(alerts)

		// Stop health checks
		if ms.healthRegistry != nil {
			ms.healthRegistry.Remove(mc.ID)
		}
	}
}

// healthCheckLoop processes HTTP health check results from the registry.
func (ms *MonitorService) healthCheckLoop() {
	defer ms.wg.Done()

	if ms.healthRegistry == nil {
		slog.Debug("Health check registry not initialized, skipping")
		return
	}

	// Also run periodic alert processing
	alertTicker := time.NewTicker(60 * time.Second)
	defer alertTicker.Stop()

	for {
		select {
		case <-ms.stopChan:
			return

		case result := <-ms.healthRegistry.Results():
			// Store result
			ms.storage.AddHTTPCheckResult(result)

			// Generate alerts using state manager
			alerts := ms.stateManager.UpdateHTTPState([]types.HTTPCheckResult{result})
			ms.processAlertsIfAny(alerts)

			// Log
			status := "success"
			if !result.Success {
				status = "failed"
			}
			slog.Debug("Health check result",
				"name", result.Name,
				"url", result.URL,
				"status", status,
				"code", result.StatusCode,
				"time_ms", result.ResponseTime.Milliseconds())

		case <-alertTicker.C:
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

	ms.storage.AddSystemStats(*stats)

	alerts := ms.stateManager.UpdateSystemState(stats, &ms.config.SystemChecks)
	ms.processAlertsIfAny(alerts)

	slog.Info("System Stats",
		"cpu", fmt.Sprintf("%.2f%%", stats.CPUUsage),
		"memory", fmt.Sprintf("%.2f%%", stats.MemoryUsage.UsedPercent),
		"disk", ms.getDiskUsageSummary(stats.DiskUsage))
}

// processAlertsIfAny is a helper to store and log alerts inline.
func (ms *MonitorService) processAlertsIfAny(alerts []types.Alert) {
	if len(alerts) == 0 {
		return
	}
	ms.storage.AddAlerts(alerts)
	for _, a := range alerts {
		slog.Info("ALERT", "level", a.Level, "type", a.Type, "message", a.Message)
	}
}

// processAlerts sends collected alerts through the alerting channels.
func (ms *MonitorService) processAlerts() {
	alerts := ms.storage.GetAlerts()
	if len(alerts) == 0 {
		return
	}

	if err := ms.alertManager.SendAlerts(alerts); err != nil {
		slog.Error("Failed to send some alerts", "error", err)
	}

	ms.storage.ClearAlerts()
}

// getDiskUsageSummary creates a summary of disk usage
func (ms *MonitorService) getDiskUsageSummary(diskUsage map[string]types.DiskStats) string {
	summary := make([]string, 0)
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
	for _, elem := range elems {
		n += len(elem)
	}

	b := make([]byte, n)
	bp := copy(b, elems[0])
	for _, s := range elems[1:] {
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], s)
	}
	return string(b)
}
