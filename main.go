package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"bconf.com/monic/alert"
	"bconf.com/monic/monitor"
	"bconf.com/monic/server"
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
	statsServer   *server.StatsServer
	stopChan      chan struct{}
	wg            sync.WaitGroup
	alerts        []types.Alert
	statsHistory  []types.SystemStats
	httpHistory   []types.HTTPCheckResult
	dockerHistory []types.DockerContainerStats
	startTime     time.Time
}

// NewMonitorService creates a new monitoring service instance
func NewMonitorService(config *types.Config) *MonitorService {
	service := &MonitorService{
		config:        config,
		systemMonitor: monitor.NewSystemMonitor(&config.SystemChecks),
		httpMonitor:   monitor.NewHTTPMonitor(),
		dockerMonitor: monitor.NewDockerMonitor(&config.DockerChecks),
		alertManager:  alert.NewAlertManager(&config.Alerting),
		stateManager:  alert.NewStateManager(),
		stopChan:      make(chan struct{}),
		alerts:        make([]types.Alert, 0),
		statsHistory:  make([]types.SystemStats, 0),
		httpHistory:   make([]types.HTTPCheckResult, 0),
		dockerHistory: make([]types.DockerContainerStats, 0),
		startTime:     time.Now(),
	}

	// Initialize stats server
	service.statsServer = server.NewStatsServer(
		&config.HTTPServer,
		service.systemMonitor,
		&service.statsHistory,
		&service.httpHistory,
		&service.alerts,
		service.stateManager,
	)

	return service
}

// Start begins the monitoring service
func (ms *MonitorService) Start() error {
	log.Println("Starting Monic monitoring service...")

	// Validate HTTP checks configuration
	for _, check := range ms.config.HTTPChecks {
		if err := ms.httpMonitor.ValidateHTTPCheck(check); err != nil {
			return fmt.Errorf("invalid HTTP check configuration for %s: %w", check.Name, err)
		}
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
	log.Printf("System Info: %+v\n", systemInfo)

	// Initialize Docker monitor if enabled
	if ms.config.DockerChecks.Enabled {
		if err := ms.dockerMonitor.Initialize(); err != nil {
			log.Printf("Warning: Failed to initialize Docker monitor: %v", err)
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

	log.Println("Monic monitoring service started successfully")
	return nil
}

// Stop gracefully stops the monitoring service
func (ms *MonitorService) Stop() {
	log.Println("Stopping Monic monitoring service...")
	close(ms.stopChan)
	ms.wg.Wait()
	log.Println("Monic monitoring service stopped")
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
		log.Printf("Error collecting system stats: %v\n", err)
		return
	}

	// Add to history (keep last 100 entries)
	ms.statsHistory = append(ms.statsHistory, *stats)
	if len(ms.statsHistory) > 100 {
		ms.statsHistory = ms.statsHistory[1:]
	}

	// Use state manager to generate alerts with 3 consecutive failures logic
	alerts := ms.stateManager.UpdateSystemState(stats, &ms.config.SystemChecks)
	if len(alerts) > 0 {
		ms.alerts = append(ms.alerts, alerts...)
		log.Printf("System alerts generated: %d\n", len(alerts))
	}

	// Log current stats (in production, this would go to a proper logging system)
	log.Printf("System Stats - CPU: %.2f%%, Memory: %.2f%%, Disk: %v\n",
		stats.CPUUsage,
		stats.MemoryUsage.UsedPercent,
		ms.getDiskUsageSummary(stats.DiskUsage))
}

// collectHTTPStats collects and processes HTTP monitoring statistics
func (ms *MonitorService) collectHTTPStats() {
	results := ms.httpMonitor.CheckEndpointsConcurrent(ms.config.HTTPChecks)

	// Add to history (keep last 100 entries)
	ms.httpHistory = append(ms.httpHistory, results...)
	if len(ms.httpHistory) > 100 {
		ms.httpHistory = ms.httpHistory[len(ms.httpHistory)-100:]
	}

	// Use state manager to generate alerts with 3 consecutive failures logic
	alerts := ms.stateManager.UpdateHTTPState(results)
	if len(alerts) > 0 {
		ms.alerts = append(ms.alerts, alerts...)
		log.Printf("HTTP alerts generated: %d\n", len(alerts))
	}

	// Log HTTP stats
	httpStats := ms.httpMonitor.GetHTTPStats(results)
	log.Printf("HTTP Stats - Total: %d, Success: %d, Failed: %d, Success Rate: %.1f%%\n",
		httpStats["total_checks"],
		httpStats["successful_checks"],
		httpStats["failed_checks"],
		httpStats["success_rate"])
}

// collectDockerStats collects and processes Docker container statistics
func (ms *MonitorService) collectDockerStats() {
	stats, err := ms.dockerMonitor.CheckContainers()
	if err != nil {
		log.Printf("Error collecting Docker stats: %v\n", err)
		return
	}

	// Add to history (keep last 100 entries)
	ms.dockerHistory = append(ms.dockerHistory, stats...)
	if len(ms.dockerHistory) > 100 {
		ms.dockerHistory = ms.dockerHistory[len(ms.dockerHistory)-100:]
	}

	// Check for container status alerts
	alerts, err := ms.dockerMonitor.CheckContainerStatus()
	if err != nil {
		log.Printf("Error checking Docker container status: %v\n", err)
	} else if len(alerts) > 0 {
		ms.alerts = append(ms.alerts, alerts...)
		log.Printf("Docker alerts generated: %d\n", len(alerts))
	}

	// Log Docker stats
	summary := ms.dockerMonitor.GetContainerSummary(stats)
	log.Printf("Docker Stats - Total: %v, Running: %v, Stopped: %v, Running: %.1f%%\n",
		summary["total_containers"],
		summary["running_containers"],
		summary["stopped_containers"],
		summary["running_percentage"])
}

// processAlerts processes and reports alerts
func (ms *MonitorService) processAlerts() {
	if len(ms.alerts) == 0 {
		return
	}

	// Log alerts to console
	for _, alert := range ms.alerts {
		log.Printf("ALERT [%s] %s: %s\n", alert.Level, alert.Type, alert.Message)
	}

	// Send alerts via configured channels (email, Mailgun, etc.)
	if err := ms.alertManager.SendAlerts(ms.alerts); err != nil {
		log.Printf("Failed to send some alerts: %v\n", err)
	}

	// Clear processed alerts
	ms.alerts = make([]types.Alert, 0)
}

// getDiskUsageSummary creates a summary of disk usage
func (ms *MonitorService) getDiskUsageSummary(diskUsage map[string]types.DiskStats) string {
	var summary []string
	for path, stats := range diskUsage {
		summary = append(summary, fmt.Sprintf("%s:%.1f%%", path, stats.UsedPercent))
	}
	return fmt.Sprintf("[%s]", stringJoin(summary, ", "))
}

// GetStatus returns the current status of the monitoring service
func (ms *MonitorService) GetStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Basic service status
	status["service"] = "running"
	status["started_at"] = time.Now().Format(time.RFC3339)

	// System information
	systemInfo := ms.systemMonitor.GetSystemInfo()
	status["system_info"] = systemInfo

	// Recent statistics
	if len(ms.statsHistory) > 0 {
		status["latest_system_stats"] = ms.statsHistory[len(ms.statsHistory)-1]
	}

	// HTTP monitoring status
	if len(ms.httpHistory) > 0 {
		status["http_stats"] = ms.httpMonitor.GetHTTPStats(ms.httpHistory)
	}

	// Active alerts
	status["active_alerts"] = len(ms.alerts)

	return status
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

// loadConfig loads configuration from JSON file
func loadConfig(configPath string) (*types.Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config types.Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// version will be set during build
var version = "dev"

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Monic v%s\n", version)
		return
	}

	// Load configuration
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start monitoring service
	service := NewMonitorService(config)
	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start monitoring service: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	service.Stop()
	log.Println("Monic monitoring service shutdown complete")
}
