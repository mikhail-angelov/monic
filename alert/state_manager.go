package alert

import (
	"fmt"
	"sync"
	"time"

	"bconf.com/monic/types"
)

// StateManager handles alert state tracking and deduplication.
// All public methods are safe for concurrent use.
type StateManager struct {
	mu     sync.Mutex
	states map[string]*types.AlertState
}

// NewStateManager creates a new state manager instance
func NewStateManager() *StateManager {
	return &StateManager{
		states: make(map[string]*types.AlertState),
	}
}

// UpdateSystemState updates the state for system metrics and returns alerts if needed
func (sm *StateManager) UpdateSystemState(stats *types.SystemStats, thresholds *types.SystemChecksConfig) []types.Alert {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var alerts []types.Alert
	now := time.Now()

	cpuState := sm.getOrCreateState("cpu")
	if a := sm.checkSystemMetric(cpuState, "cpu", stats.CPUUsage, float64(thresholds.CPUThreshold), now); a != nil {
		alerts = append(alerts, *a)
	}

	memState := sm.getOrCreateState("memory")
	if a := sm.checkSystemMetric(memState, "memory", stats.MemoryUsage.UsedPercent, float64(thresholds.MemoryThreshold), now); a != nil {
		alerts = append(alerts, *a)
	}

	for path, diskStats := range stats.DiskUsage {
		diskState := sm.getOrCreateState("disk_" + path)
		if a := sm.checkSystemMetric(diskState, "disk_"+path, diskStats.UsedPercent, float64(thresholds.DiskThreshold), now); a != nil {
			alerts = append(alerts, *a)
		}
	}

	return alerts
}

// UpdateHTTPState updates the state for HTTP checks and returns alerts if needed
func (sm *StateManager) UpdateHTTPState(results []types.HTTPCheckResult) []types.Alert {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var alerts []types.Alert
	now := time.Now()

	for _, result := range results {
		stateKey := "http_" + result.Name
		httpState := sm.getOrCreateState(stateKey)

		currentState := "ok"
		if !result.Success {
			currentState = "critical"
		}

		message := result.Error
		if currentState == "ok" && httpState.CurrentState == "critical" {
			message = result.Name + " has recovered"
		}
		if a := sm.updateState(httpState, stateKey, currentState, message, now, 0, 0); a != nil {
			alerts = append(alerts, *a)
		}
	}

	return alerts
}

func (sm *StateManager) checkSystemMetric(state *types.AlertState, alertType string, currentValue, threshold float64, now time.Time) *types.Alert {
	currentState := "ok"
	if currentValue >= threshold {
		currentState = "critical"
	}
	return sm.updateState(state, alertType, currentState, "", now, currentValue, threshold)
}

func (sm *StateManager) updateState(state *types.AlertState, alertType, currentState, message string, now time.Time, currentValue, threshold float64) *types.Alert {
	previousState := state.CurrentState

	if state.CurrentState != currentState {
		state.CurrentState = currentState
		state.ConsecutiveChecks = 1
		state.LastStateChange = now
		if previousState != "critical" || currentState != "ok" {
			state.SentCriticalAlert = false
		}
	} else {
		state.ConsecutiveChecks++
	}

	if !sm.shouldSendAlert(state) {
		return nil
	}

	state.LastAlertSent = now

	if message != "" {
		level := "warning"
		if currentState == "critical" {
			level = "critical"
			state.SentCriticalAlert = true
		}
		return &types.Alert{Type: alertType, Message: message, Level: level, Timestamp: now}
	}

	if currentState == "critical" {
		state.SentCriticalAlert = true
		return &types.Alert{
			Type:      alertType,
			Message:   getSystemAlertMessage(alertType, currentValue, threshold),
			Level:     "critical",
			Timestamp: now,
		}
	}

	if currentState == "ok" && previousState == "critical" {
		if !state.SentCriticalAlert {
			return nil
		}
		state.SentCriticalAlert = false
		return &types.Alert{
			Type:      alertType,
			Message:   getSystemRecoveryMessage(alertType, currentValue, threshold),
			Level:     "info",
			Timestamp: now,
		}
	}

	return nil
}

func (sm *StateManager) shouldSendAlert(state *types.AlertState) bool {
	if state.CurrentState == "ok" {
		if state.ConsecutiveChecks == 1 && state.LastAlertSent.Before(state.LastStateChange) && state.SentCriticalAlert {
			return true
		}
		return false
	}
	if state.ConsecutiveChecks < 3 {
		return false
	}
	return state.LastAlertSent.Before(state.LastStateChange)
}

// getOrCreateState returns an existing state or creates a new one. Caller must hold sm.mu.
func (sm *StateManager) getOrCreateState(alertType string) *types.AlertState {
	if state, exists := sm.states[alertType]; exists {
		return state
	}
	state := &types.AlertState{
		Type:            alertType,
		CurrentState:    "ok",
		LastStateChange: time.Now(),
	}
	sm.states[alertType] = state
	return state
}

// GetStates returns all current alert states (for testing and debugging)
func (sm *StateManager) GetStates() map[string]*types.AlertState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.states
}

// ResetState resets a specific alert state (for testing)
func (sm *StateManager) ResetState(alertType string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.states, alertType)
}

func getSystemAlertMessage(alertType string, currentValue, threshold float64) string {
	switch alertType {
	case "cpu":
		return formatSystemMessage("CPU usage", currentValue, threshold)
	case "memory":
		return formatSystemMessage("Memory usage", currentValue, threshold)
	default:
		if len(alertType) > 5 && alertType[:5] == "disk_" {
			return formatSystemMessage("Disk usage on "+alertType[5:], currentValue, threshold)
		}
		return formatSystemMessage(alertType, currentValue, threshold)
	}
}

func getSystemRecoveryMessage(alertType string, currentValue, threshold float64) string {
	switch alertType {
	case "cpu":
		return formatRecoveryMessage("CPU usage", currentValue, threshold)
	case "memory":
		return formatRecoveryMessage("Memory usage", currentValue, threshold)
	default:
		if len(alertType) > 5 && alertType[:5] == "disk_" {
			return formatRecoveryMessage("Disk usage on "+alertType[5:], currentValue, threshold)
		}
		return formatRecoveryMessage(alertType, currentValue, threshold)
	}
}

func formatSystemMessage(metric string, currentValue, threshold float64) string {
	return fmt.Sprintf("%s is %.1f%% (threshold: %.1f%%)", metric, currentValue, threshold)
}

func formatRecoveryMessage(metric string, currentValue, threshold float64) string {
	return fmt.Sprintf("%s recovered to %.1f%% (threshold: %.1f%%)", metric, currentValue, threshold)
}
