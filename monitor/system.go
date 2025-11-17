package monitor

import (
	"fmt"
	"runtime"
	"time"

	"bconf.com/monic/types"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemMonitor handles system resource monitoring
type SystemMonitor struct {
	config *types.SystemChecksConfig
}

// NewSystemMonitor creates a new system monitor instance
func NewSystemMonitor(config *types.SystemChecksConfig) *SystemMonitor {
	return &SystemMonitor{
		config: config,
	}
}

// CollectStats collects all system statistics
func (sm *SystemMonitor) CollectStats() (*types.SystemStats, error) {
	stats := &types.SystemStats{
		Timestamp: time.Now(),
		DiskUsage: make(map[string]types.DiskStats),
	}

	// Collect CPU usage
	cpuUsage, err := sm.getCPUUsage()
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}
	stats.CPUUsage = cpuUsage

	// Collect memory usage
	memStats, err := sm.getMemoryUsage()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory usage: %w", err)
	}
	stats.MemoryUsage = memStats

	// Collect disk usage for configured paths
	for _, path := range sm.config.DiskPaths {
		diskStats, err := sm.getDiskUsage(path)
		if err != nil {
			// Log error but continue with other paths
			fmt.Printf("Warning: failed to get disk usage for %s: %v\n", path, err)
			continue
		}
		stats.DiskUsage[path] = diskStats
	}

	return stats, nil
}

// getCPUUsage returns the current CPU usage percentage
func (sm *SystemMonitor) getCPUUsage() (float64, error) {
	// Get CPU usage for a short interval to get current usage
	percentages, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		return 0, err
	}

	if len(percentages) == 0 {
		return 0, fmt.Errorf("no CPU usage data available")
	}

	return percentages[0], nil
}

// getMemoryUsage returns current memory usage statistics
func (sm *SystemMonitor) getMemoryUsage() (types.MemoryStats, error) {
	var stats types.MemoryStats

	virtualMem, err := mem.VirtualMemory()
	if err != nil {
		return stats, err
	}

	stats.Total = virtualMem.Total
	stats.Used = virtualMem.Used
	stats.Free = virtualMem.Free
	stats.UsedPercent = virtualMem.UsedPercent

	return stats, nil
}

// getDiskUsage returns disk usage statistics for a specific path
func (sm *SystemMonitor) getDiskUsage(path string) (types.DiskStats, error) {
	var stats types.DiskStats

	usage, err := disk.Usage(path)
	if err != nil {
		return stats, err
	}

	stats.Path = path
	stats.Total = usage.Total
	stats.Used = usage.Used
	stats.Free = usage.Free
	stats.UsedPercent = usage.UsedPercent

	return stats, nil
}

// CheckThresholds checks if any system metrics exceed configured thresholds
func (sm *SystemMonitor) CheckThresholds(stats *types.SystemStats, thresholds *types.SystemChecksConfig) []types.Alert {
	var alerts []types.Alert

	// Check CPU threshold
	if stats.CPUUsage > float64(thresholds.CPUThreshold) {
		alerts = append(alerts, types.Alert{
			Type:      "cpu",
			Message:   fmt.Sprintf("CPU usage is %.2f%% (threshold: %d%%)", stats.CPUUsage, thresholds.CPUThreshold),
			Level:     "warning",
			Timestamp: time.Now(),
		})
	}

	// Check memory threshold
	if stats.MemoryUsage.UsedPercent > float64(thresholds.MemoryThreshold) {
		alerts = append(alerts, types.Alert{
			Type:      "memory",
			Message:   fmt.Sprintf("Memory usage is %.2f%% (threshold: %d%%)", stats.MemoryUsage.UsedPercent, thresholds.MemoryThreshold),
			Level:     "warning",
			Timestamp: time.Now(),
		})
	}

	// Check disk thresholds
	for path, diskStats := range stats.DiskUsage {
		if diskStats.UsedPercent > float64(thresholds.DiskThreshold) {
			alerts = append(alerts, types.Alert{
				Type:      "disk",
				Message:   fmt.Sprintf("Disk usage on %s is %.2f%% (threshold: %d%%)", path, diskStats.UsedPercent, thresholds.DiskThreshold),
				Level:     "warning",
				Timestamp: time.Now(),
			})
		}
	}

	return alerts
}

// GetSystemInfo returns basic system information
func (sm *SystemMonitor) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Get host info
	hostInfo, _ := sm.getHostInfo()
	info["host"] = hostInfo

	// Get runtime info
	info["runtime"] = map[string]interface{}{
		"go_version":   runtime.Version(),
		"num_cpu":      runtime.NumCPU(),
		"goroutines":   runtime.NumGoroutine(),
		"go_max_procs": runtime.GOMAXPROCS(0),
	}

	return info
}

// getHostInfo returns basic host information
func (sm *SystemMonitor) getHostInfo() (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get number of logical CPUs
	numCPU := runtime.NumCPU()
	info["num_cpus"] = numCPU

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return info, err
	}

	info["total_memory"] = memInfo.Total
	info["available_memory"] = memInfo.Available

	return info, nil
}
