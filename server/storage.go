package server

import (
	"sync"
	"time"

	"bconf.com/monic/types"
)

// Storage defines the interface for storage operations
type Storage interface {
	GetLatestSystemStats() *types.SystemStats
	GetAlertsCount() int
	GetHTTPCheckResults() []types.HTTPCheckResult
	GetAlerts() []types.Alert
	GetSystemStats() []types.SystemStats

	AddSystemStats(stats types.SystemStats)
	AddAlert(alert types.Alert)
	AddAlerts(alerts []types.Alert)
	AddHTTPCheckResult(result types.HTTPCheckResult)
	ClearAlerts()
}

// StorageManager provides thread-safe in-memory storage for monitoring data
type StorageManager struct {
	alerts      []types.Alert
	statsHistory []types.SystemStats
	httpHistory  []types.HTTPCheckResult

	alertsMu       sync.RWMutex
	statsHistoryMu sync.RWMutex
	httpHistoryMu  sync.RWMutex

	maxHistorySize int
}

// NewStorageManager creates a new thread-safe storage manager
func NewStorageManager(maxHistorySize int) *StorageManager {
	if maxHistorySize <= 0 {
		maxHistorySize = 100
	}
	return &StorageManager{
		alerts:         make([]types.Alert, 0),
		statsHistory:   make([]types.SystemStats, 0),
		httpHistory:    make([]types.HTTPCheckResult, 0),
		maxHistorySize: maxHistorySize,
	}
}

// AddAlert adds a single alert to storage
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

// GetAlerts returns a copy of all stored alerts
func (sm *StorageManager) GetAlerts() []types.Alert {
	sm.alertsMu.RLock()
	defer sm.alertsMu.RUnlock()

	result := make([]types.Alert, len(sm.alerts))
	copy(result, sm.alerts)
	return result
}

// ClearAlerts removes all alerts from storage
func (sm *StorageManager) ClearAlerts() {
	sm.alertsMu.Lock()
	defer sm.alertsMu.Unlock()
	sm.alerts = sm.alerts[:0]
}

// GetAlertsCount returns the number of stored alerts
func (sm *StorageManager) GetAlertsCount() int {
	sm.alertsMu.RLock()
	defer sm.alertsMu.RUnlock()
	return len(sm.alerts)
}

// AddSystemStats appends a system stats snapshot to history
func (sm *StorageManager) AddSystemStats(stats types.SystemStats) {
	sm.statsHistoryMu.Lock()
	defer sm.statsHistoryMu.Unlock()

	sm.statsHistory = append(sm.statsHistory, stats)
	if len(sm.statsHistory) > sm.maxHistorySize {
		sm.statsHistory = sm.statsHistory[1:]
	}
}

// GetSystemStats returns a copy of the full system stats history
func (sm *StorageManager) GetSystemStats() []types.SystemStats {
	sm.statsHistoryMu.RLock()
	defer sm.statsHistoryMu.RUnlock()

	result := make([]types.SystemStats, len(sm.statsHistory))
	copy(result, sm.statsHistory)
	return result
}

// GetLatestSystemStats returns the most recent system stats snapshot
func (sm *StorageManager) GetLatestSystemStats() *types.SystemStats {
	sm.statsHistoryMu.RLock()
	defer sm.statsHistoryMu.RUnlock()

	if len(sm.statsHistory) == 0 {
		return nil
	}
	latest := sm.statsHistory[len(sm.statsHistory)-1]
	return &latest
}

// AddHTTPCheckResult appends an HTTP check result to history
func (sm *StorageManager) AddHTTPCheckResult(result types.HTTPCheckResult) {
	sm.httpHistoryMu.Lock()
	defer sm.httpHistoryMu.Unlock()

	sm.httpHistory = append(sm.httpHistory, result)
	if len(sm.httpHistory) > sm.maxHistorySize {
		sm.httpHistory = sm.httpHistory[1:]
	}
}

// GetHTTPCheckResults returns a copy of all HTTP check results
func (sm *StorageManager) GetHTTPCheckResults() []types.HTTPCheckResult {
	sm.httpHistoryMu.RLock()
	defer sm.httpHistoryMu.RUnlock()

	result := make([]types.HTTPCheckResult, len(sm.httpHistory))
	copy(result, sm.httpHistory)
	return result
}

// GetLatestHTTPCheckResult returns the most recent result for a given check name
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

// GetStatus returns counts for each storage bucket (for diagnostics)
func (sm *StorageManager) GetStatus() map[string]any {
	sm.alertsMu.RLock()
	sm.statsHistoryMu.RLock()
	sm.httpHistoryMu.RLock()
	defer func() {
		sm.alertsMu.RUnlock()
		sm.statsHistoryMu.RUnlock()
		sm.httpHistoryMu.RUnlock()
	}()

	return map[string]any{
		"alerts_count":        len(sm.alerts),
		"stats_history_count": len(sm.statsHistory),
		"http_history_count":  len(sm.httpHistory),
		"max_history_size":    sm.maxHistorySize,
		"timestamp":           time.Now().Format(time.RFC3339),
	}
}
