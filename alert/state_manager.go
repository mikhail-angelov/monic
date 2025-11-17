package alert

import (
	"time"

	"bconf.com/monic/types"
)

// StateManager handles alert state tracking and deduplication
type StateManager struct {
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
	var alerts []types.Alert
	now := time.Now()

	// Check CPU
	cpuState := sm.getOrCreateState("cpu")
	cpuAlert := sm.checkSystemMetric(cpuState, "cpu", stats.CPUUsage, float64(thresholds.CPUThreshold), now)
	if cpuAlert != nil {
		alerts = append(alerts, *cpuAlert)
	}

	// Check Memory
	memoryState := sm.getOrCreateState("memory")
	memoryAlert := sm.checkSystemMetric(memoryState, "memory", stats.MemoryUsage.UsedPercent, float64(thresholds.MemoryThreshold), now)
	if memoryAlert != nil {
		alerts = append(alerts, *memoryAlert)
	}

	// Check Disk for each path
	for path, diskStats := range stats.DiskUsage {
		diskState := sm.getOrCreateState("disk_" + path)
		diskAlert := sm.checkSystemMetric(diskState, "disk_"+path, diskStats.UsedPercent, float64(thresholds.DiskThreshold), now)
		if diskAlert != nil {
			alerts = append(alerts, *diskAlert)
		}
	}

	return alerts
}

// UpdateHTTPState updates the state for HTTP checks and returns alerts if needed
func (sm *StateManager) UpdateHTTPState(results []types.HTTPCheckResult) []types.Alert {
	var alerts []types.Alert
	now := time.Now()

	for _, result := range results {
		stateKey := "http_" + result.Name
		httpState := sm.getOrCreateState(stateKey)

		// Determine current state
		currentState := "ok"
		if !result.Success {
			currentState = "critical"
		}

		alert := sm.updateState(httpState, stateKey, currentState, result.Error, now)
		if alert != nil {
			alerts = append(alerts, *alert)
		}
	}

	return alerts
}

// checkSystemMetric checks a system metric against threshold and updates state
func (sm *StateManager) checkSystemMetric(state *types.AlertState, alertType string, currentValue, threshold float64, now time.Time) *types.Alert {
	// Determine current state
	currentState := "ok"
	if currentValue >= threshold {
		currentState = "critical"
	}

	message := ""
	if currentState == "critical" {
		message = getSystemAlertMessage(alertType, currentValue, threshold)
	} else {
		message = getSystemRecoveryMessage(alertType, currentValue, threshold)
	}

	return sm.updateState(state, alertType, currentState, message, now)
}

// updateState updates the alert state and returns an alert if needed
func (sm *StateManager) updateState(state *types.AlertState, alertType, currentState, message string, now time.Time) *types.Alert {
	// If state changed, reset consecutive checks
	if state.CurrentState != currentState {
		state.CurrentState = currentState
		state.ConsecutiveChecks = 1
		state.LastStateChange = now
	} else {
		state.ConsecutiveChecks++
	}

	// Check if we should send an alert
	if sm.shouldSendAlert(state, now) {
		state.LastAlertSent = now
		level := "warning"
		if currentState == "critical" {
			level = "critical"
		}

		return &types.Alert{
			Type:      alertType,
			Message:   message,
			Level:     level,
			Timestamp: now,
		}
	}

	return nil
}

// shouldSendAlert determines if an alert should be sent based on state
func (sm *StateManager) shouldSendAlert(state *types.AlertState, now time.Time) bool {
	// Don't send alerts for OK state
	if state.CurrentState == "ok" {
		// Only send recovery alert if we were previously in a bad state
		// and this is the first OK check after recovery
		if state.ConsecutiveChecks == 1 && state.LastAlertSent.After(state.LastStateChange) {
			return true
		}
		return false
	}

	// For bad states, require 3 consecutive failures
	if state.ConsecutiveChecks < 3 {
		return false
	}

	// Check if we've already sent an alert for this state
	// Only send one alert per state change
	if state.LastAlertSent.After(state.LastStateChange) {
		return false
	}

	return true
}

// getOrCreateState gets an existing state or creates a new one
func (sm *StateManager) getOrCreateState(alertType string) *types.AlertState {
	if state, exists := sm.states[alertType]; exists {
		return state
	}

	state := &types.AlertState{
		Type:              alertType,
		CurrentState:      "ok",
		ConsecutiveChecks: 0,
		LastAlertSent:     time.Time{},
		LastStateChange:   time.Now(),
	}
	sm.states[alertType] = state
	return state
}

// getSystemAlertMessage generates alert messages for system metrics
func getSystemAlertMessage(alertType string, currentValue, threshold float64) string {
	switch alertType {
	case "cpu":
		return formatSystemMessage("CPU usage", currentValue, threshold, "%")
	case "memory":
		return formatSystemMessage("Memory usage", currentValue, threshold, "%")
	default:
		if len(alertType) > 5 && alertType[:5] == "disk_" {
			path := alertType[5:]
			return formatSystemMessage("Disk usage on "+path, currentValue, threshold, "%")
		}
		return formatSystemMessage(alertType, currentValue, threshold, "%")
	}
}

// getSystemRecoveryMessage generates recovery messages for system metrics
func getSystemRecoveryMessage(alertType string, currentValue, threshold float64) string {
	switch alertType {
	case "cpu":
		return formatRecoveryMessage("CPU usage", currentValue, threshold, "%")
	case "memory":
		return formatRecoveryMessage("Memory usage", currentValue, threshold, "%")
	default:
		if len(alertType) > 5 && alertType[:5] == "disk_" {
			path := alertType[5:]
			return formatRecoveryMessage("Disk usage on "+path, currentValue, threshold, "%")
		}
		return formatRecoveryMessage(alertType, currentValue, threshold, "%")
	}
}

// formatSystemMessage formats system alert messages
func formatSystemMessage(metric string, currentValue, threshold float64, unit string) string {
	return metric + " is " + formatValue(currentValue, unit) + " (threshold: " + formatValue(threshold, unit) + ")"
}

// formatRecoveryMessage formats system recovery messages
func formatRecoveryMessage(metric string, currentValue, threshold float64, unit string) string {
	return metric + " recovered to " + formatValue(currentValue, unit) + " (threshold: " + formatValue(threshold, unit) + ")"
}

// formatValue formats a value with unit
func formatValue(value float64, unit string) string {
	return formatFloat(value) + unit
}

// formatFloat formats a float to one decimal place
func formatFloat(value float64) string {
	return formatFloatPrecision(value, 1)
}

// formatFloatPrecision formats a float with specified precision
func formatFloatPrecision(value float64, precision int) string {
	// Simple formatting without using fmt.Sprintf
	// This implementation handles basic cases
	if value == 0 {
		return "0.0"
	}

	// Handle integer part
	intPart := int(value)
	fracPart := value - float64(intPart)

	// Handle fractional part
	fracStr := ""
	if precision > 0 {
		multiplier := 1
		for i := 0; i < precision; i++ {
			multiplier *= 10
		}
		fracInt := int(fracPart*float64(multiplier) + 0.5)
		fracStr = "." + itoa(fracInt)
	}

	return itoa(intPart) + fracStr
}

// itoa converts integer to string (basic implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}

// GetStates returns all current alert states (for testing and debugging)
func (sm *StateManager) GetStates() map[string]*types.AlertState {
	return sm.states
}

// ResetState resets a specific alert state (for testing)
func (sm *StateManager) ResetState(alertType string) {
	delete(sm.states, alertType)
}
