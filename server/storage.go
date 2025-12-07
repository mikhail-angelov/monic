package server

import (
	"sync"
	"time"

	"bconf.com/monic/types"
)

// StorageManager provides thread-safe storage for monitoring data
type StorageManager struct {
	alerts        []types.Alert
	statsHistory  []types.SystemStats
	httpHistory   []types.HTTPCheckResult
	dockerHistory []types.DockerContainerStats

	alertsMu        sync.RWMutex
	statsHistoryMu  sync.RWMutex
	httpHistoryMu   sync.RWMutex
	dockerHistoryMu sync.RWMutex

	maxHistorySize int
}

// NewStorageManager creates a new thread-safe storage manager
func NewStorageManager(maxHistorySize int) *StorageManager {
	if maxHistorySize <= 0 {
		maxHistorySize = 100 // Default to 100 entries
	}

	return &StorageManager{
		alerts:        make([]types.Alert, 0),
		statsHistory:  make([]types.SystemStats, 0),
		httpHistory:   make([]types.HTTPCheckResult, 0),
		dockerHistory: make([]types.DockerContainerStats, 0),
		maxHistorySize: maxHistorySize,
	}
}

// AddAlert adds an alert to storage
func (sm *StorageManager) AddAlert(alert types.Alert) {
	sm.alertsMu.Lock()
	defer sm.alertsMu.Unlock()

	sm.alerts = append(sm.alerts, alert)
	if len(sm.alerts) > sm.maxHistorySize {
		sm.alerts = sm.alerts[1:]
	}
}

// AddAlerts adds multiple alerts to storage
func (sm *StorageManager) AddAlerts(alerts []types.Alert) {
	if len(alerts) == 0 {
		return
	}

	sm.alertsMu.Lock()
	defer sm.alertsMu.Unlock()

	sm.alerts = append(sm.alerts, alerts...)
	if len(sm.alerts) > sm.maxHistorySize {
		sm.alerts = sm.alerts[len(sm.alerts)-sm.maxHistorySize:]
	}
}

// GetAlerts returns all alerts
func (sm *StorageManager) GetAlerts() []types.Alert {
	sm.alertsMu.RLock()
	defer sm.alertsMu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]types.Alert, len(sm.alerts))
	copy(result, sm.alerts)
	return result
}

// ClearAlerts removes all alerts from storage
func (sm *StorageManager) ClearAlerts() {
	sm.alertsMu.Lock()
	defer sm.alertsMu.Unlock()

	sm.alerts = make([]types.Alert, 0)
}

// AddSystemStats adds system stats to history
func (sm *StorageManager) AddSystemStats(stats types.SystemStats) {
	sm.statsHistoryMu.Lock()
	defer sm.statsHistoryMu.Unlock()

	sm.statsHistory = append(sm.statsHistory, stats)
	if len(sm.statsHistory) > sm.maxHistorySize {
		sm.statsHistory = sm.statsHistory[1:]
	}
}

// GetSystemStats returns all system stats history
func (sm *StorageManager) GetSystemStats() []types.SystemStats {
	sm.statsHistoryMu.RLock()
	defer sm.statsHistoryMu.RUnlock()

	result := make([]types.SystemStats, len(sm.statsHistory))
	copy(result, sm.statsHistory)
	return result
}

// GetLatestSystemStats returns the most recent system stats
func (sm *StorageManager) GetLatestSystemStats() *types.SystemStats {
	sm.statsHistoryMu.RLock()
	defer sm.statsHistoryMu.RUnlock()

	if len(sm.statsHistory) == 0 {
		return nil
	}

	// Return a copy
	latest := sm.statsHistory[len(sm.statsHistory)-1]
	return &latest
}

// AddHTTPCheckResult adds HTTP check result to history
func (sm *StorageManager) AddHTTPCheckResult(result types.HTTPCheckResult) {
	sm.httpHistoryMu.Lock()
	defer sm.httpHistoryMu.Unlock()

	sm.httpHistory = append(sm.httpHistory, result)
	if len(sm.httpHistory) > sm.maxHistorySize {
		sm.httpHistory = sm.httpHistory[1:]
	}
}

// GetHTTPCheckResults returns all HTTP check results
func (sm *StorageManager) GetHTTPCheckResults() []types.HTTPCheckResult {
	sm.httpHistoryMu.RLock()
	defer sm.httpHistoryMu.RUnlock()

	result := make([]types.HTTPCheckResult, len(sm.httpHistory))
	copy(result, sm.httpHistory)
	return result
}

// GetLatestHTTPCheckResult returns the most recent HTTP check result for a given name
func (sm *StorageManager) GetLatestHTTPCheckResult(name string) *types.HTTPCheckResult {
	sm.httpHistoryMu.RLock()
	defer sm.httpHistoryMu.RUnlock()

	for i := len(sm.httpHistory) - 1; i >= 0; i-- {
		if sm.httpHistory[i].Name == name {
			result := sm.httpHistory[i]
			return &result
		}
	}
	return nil
}

// AddDockerContainerStats adds Docker container stats to history
func (sm *StorageManager) AddDockerContainerStats(stats []types.DockerContainerStats) {
	if len(stats) == 0 {
		return
	}

	sm.dockerHistoryMu.Lock()
	defer sm.dockerHistoryMu.Unlock()

	sm.dockerHistory = append(sm.dockerHistory, stats...)
	if len(sm.dockerHistory) > sm.maxHistorySize {
		sm.dockerHistory = sm.dockerHistory[len(sm.dockerHistory)-sm.maxHistorySize:]
	}
}

// GetDockerContainerStats returns all Docker container stats
func (sm *StorageManager) GetDockerContainerStats() []types.DockerContainerStats {
	sm.dockerHistoryMu.RLock()
	defer sm.dockerHistoryMu.RUnlock()

	result := make([]types.DockerContainerStats, len(sm.dockerHistory))
	copy(result, sm.dockerHistory)
	return result
}

// GetStatus returns the current status of storage
func (sm *StorageManager) GetStatus() map[string]interface{} {
	sm.alertsMu.RLock()
	sm.statsHistoryMu.RLock()
	sm.httpHistoryMu.RLock()
	sm.dockerHistoryMu.RLock()
	defer func() {
		sm.alertsMu.RUnlock()
		sm.statsHistoryMu.RUnlock()
		sm.httpHistoryMu.RUnlock()
		sm.dockerHistoryMu.RUnlock()
	}()

	return map[string]interface{}{
		"alerts_count":        len(sm.alerts),
		"stats_history_count": len(sm.statsHistory),
		"http_history_count":  len(sm.httpHistory),
		"docker_history_count": len(sm.dockerHistory),
		"max_history_size":    sm.maxHistorySize,
		"timestamp":           time.Now().Format(time.RFC3339),
	}
}

// GetAlertsCount returns the number of alerts
func (sm *StorageManager) GetAlertsCount() int {
	sm.alertsMu.RLock()
	defer sm.alertsMu.RUnlock()
	return len(sm.alerts)
}

// GetSystemStatsCount returns the number of system stats entries
func (sm *StorageManager) GetSystemStatsCount() int {
	sm.statsHistoryMu.RLock()
	defer sm.statsHistoryMu.RUnlock()
	return len(sm.statsHistory)
}

// GetHTTPCheckResultsCount returns the number of HTTP check results
func (sm *StorageManager) GetHTTPCheckResultsCount() int {
	sm.httpHistoryMu.RLock()
	defer sm.httpHistoryMu.RUnlock()
	return len(sm.httpHistory)
}

// GetDockerContainerStatsCount returns the number of Docker container stats
func (sm *StorageManager) GetDockerContainerStatsCount() int {
	sm.dockerHistoryMu.RLock()
	defer sm.dockerHistoryMu.RUnlock()
	return len(sm.dockerHistory)
}
