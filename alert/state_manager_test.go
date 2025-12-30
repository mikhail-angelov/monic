package alert

import (
	"testing"
	"time"

	"bconf.com/monic/types"
)

func TestNewStateManager(t *testing.T) {
	sm := NewStateManager()

	if sm == nil {
		t.Fatal("Expected StateManager instance, got nil")
	}

	if sm.states == nil {
		t.Error("Expected states map to be initialized")
	}

	if len(sm.states) != 0 {
		t.Errorf("Expected empty states map, got %d states", len(sm.states))
	}
}

func TestStateManager_GetOrCreateState(t *testing.T) {
	sm := NewStateManager()

	// Test creating new state
	state1 := sm.getOrCreateState("test_metric")
	if state1 == nil {
		t.Fatal("Expected state, got nil")
	}

	if state1.Type != "test_metric" {
		t.Errorf("Expected state type 'test_metric', got %s", state1.Type)
	}

	if state1.CurrentState != "ok" {
		t.Errorf("Expected initial state 'ok', got %s", state1.CurrentState)
	}

	if state1.ConsecutiveChecks != 0 {
		t.Errorf("Expected initial consecutive checks 0, got %d", state1.ConsecutiveChecks)
	}

	if !state1.LastAlertSent.IsZero() {
		t.Error("Expected LastAlertSent to be zero time initially")
	}

	if state1.SentCriticalAlert {
		t.Error("Expected SentCriticalAlert to be false initially")
	}

	// Test getting existing state
	state2 := sm.getOrCreateState("test_metric")
	if state2 != state1 {
		t.Error("Expected to get same state instance for same metric")
	}

	// Test creating another state
	state3 := sm.getOrCreateState("another_metric")
	if state3 == state1 {
		t.Error("Expected different state instance for different metric")
	}

	if len(sm.states) != 2 {
		t.Errorf("Expected 2 states in map, got %d", len(sm.states))
	}
}

func TestStateManager_ShouldSendAlert_CriticalState(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Create a critical state
	state := &types.AlertState{
		Type:              "test",
		CurrentState:      "critical",
		ConsecutiveChecks: 3,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	// Should send alert: 3 consecutive checks, no alert sent for this state change
	if !sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return true for critical state with 3 consecutive checks")
	}

	// Should NOT send alert: only 2 consecutive checks
	state.ConsecutiveChecks = 2
	if sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return false for critical state with < 3 consecutive checks")
	}

	// Should NOT send alert: already sent alert for this state change
	state.ConsecutiveChecks = 3
	state.LastAlertSent = now.Add(-5 * time.Minute) // After LastStateChange
	if sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return false when alert already sent for this state change")
	}
}

func TestStateManager_ShouldSendAlert_OkState(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Create an OK state that just recovered from critical
	state := &types.AlertState{
		Type:              "test",
		CurrentState:      "ok",
		ConsecutiveChecks: 1,                         // First OK check after recovery
		LastAlertSent:     now.Add(-5 * time.Minute), // Sent critical alert before state change
		LastStateChange:   now.Add(-1 * time.Minute), // State changed to OK recently
		SentCriticalAlert: true,
	}

	// Should send recovery alert: first OK check, alert sent before state change
	if !sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return true for recovery alert")
	}

	// Should NOT send alert: not first OK check after recovery
	state.ConsecutiveChecks = 2
	if sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return false for non-first OK check")
	}

	// Should NOT send alert: no critical alert was sent (SentCriticalAlert is false)
	state.ConsecutiveChecks = 1
	state.LastAlertSent = now.Add(-5 * time.Minute) // Alert was sent
	state.SentCriticalAlert = false                 // But not a critical alert (or already reset)
	if sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return false when SentCriticalAlert is false")
	}

	// Should NOT send alert: alert sent after state change (shouldn't happen in practice)
	state.LastAlertSent = now // After LastStateChange
	state.SentCriticalAlert = true
	if sm.shouldSendAlert(state, now) {
		t.Error("Expected shouldSendAlert to return false when alert sent after state change")
	}
}

func TestStateManager_UpdateState_CriticalAlert(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Create initial state in OK state
	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "ok",
		ConsecutiveChecks: 5,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	// First critical check - should not send alert (needs 3 consecutive)
	alert := sm.updateState(state, "cpu", "critical", "", now, 90.0, 80.0)
	if alert != nil {
		t.Error("Expected no alert on first critical check")
	}
	if state.CurrentState != "critical" {
		t.Error("Expected state to be updated to critical")
	}
	if state.ConsecutiveChecks != 1 {
		t.Errorf("Expected consecutive checks to be 1, got %d", state.ConsecutiveChecks)
	}
	if state.SentCriticalAlert {
		t.Error("Expected SentCriticalAlert to be false (no alert sent yet)")
	}

	// Second critical check - should not send alert
	alert = sm.updateState(state, "cpu", "critical", "", now.Add(time.Second), 91.0, 80.0)
	if alert != nil {
		t.Error("Expected no alert on second critical check")
	}
	if state.ConsecutiveChecks != 2 {
		t.Errorf("Expected consecutive checks to be 2, got %d", state.ConsecutiveChecks)
	}

	// Third critical check - should send alert
	alert = sm.updateState(state, "cpu", "critical", "", now.Add(2*time.Second), 92.0, 80.0)
	if alert == nil {
		t.Fatal("Expected alert on third consecutive critical check")
	}
	if alert.Type != "cpu" {
		t.Errorf("Expected alert type 'cpu', got %s", alert.Type)
	}
	if alert.Level != "critical" {
		t.Errorf("Expected alert level 'critical', got %s", alert.Level)
	}
	if alert.Message != "CPU usage is 92.0% (threshold: 80.0%)" {
		t.Errorf("Unexpected alert message: %s", alert.Message)
	}
	if !state.SentCriticalAlert {
		t.Error("Expected SentCriticalAlert to be true after sending critical alert")
	}
	if state.LastAlertSent.IsZero() {
		t.Error("Expected LastAlertSent to be updated")
	}
}

func TestStateManager_UpdateState_RecoveryAlert(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Create state in critical state with sent alert
	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "critical",
		ConsecutiveChecks: 3,
		LastAlertSent:     now.Add(-5 * time.Minute),
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: true,
	}

	// Recover to OK - should send recovery alert
	alert := sm.updateState(state, "cpu", "ok", "", now, 70.0, 80.0)
	if alert == nil {
		t.Fatal("Expected recovery alert when recovering from critical state")
	}
	if alert.Type != "cpu" {
		t.Errorf("Expected alert type 'cpu', got %s", alert.Type)
	}
	if alert.Level != "info" {
		t.Errorf("Expected alert level 'warning' for recovery, got %s", alert.Level)
	}
	if alert.Message != "CPU usage recovered to 70.0% (threshold: 80.0%)" {
		t.Errorf("Unexpected recovery message: %s", alert.Message)
	}
	if state.SentCriticalAlert {
		t.Error("Expected SentCriticalAlert to be reset after recovery alert")
	}
	if state.CurrentState != "ok" {
		t.Error("Expected state to be updated to ok")
	}
	if state.ConsecutiveChecks != 1 {
		t.Errorf("Expected consecutive checks to be 1 after state change, got %d", state.ConsecutiveChecks)
	}
}

func TestStateManager_UpdateState_NoRecoveryForTemporaryBlip(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Create state in critical state but NO alert sent (temporary blip)
	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "critical",
		ConsecutiveChecks: 2,           // Only 2 checks, no alert sent
		LastAlertSent:     time.Time{}, // No alert sent
		LastStateChange:   now.Add(-5 * time.Minute),
		SentCriticalAlert: false,
	}

	// Recover to OK - should NOT send recovery alert (was temporary blip)
	alert := sm.updateState(state, "cpu", "ok", "", now, 70.0, 80.0)
	if alert != nil {
		t.Error("Expected no recovery alert for temporary blip (no critical alert sent)")
	}
	if state.SentCriticalAlert {
		t.Error("Expected SentCriticalAlert to remain false")
	}
}

func TestStateManager_UpdateState_WithMessage(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Test with provided message (e.g., from HTTP checks)
	state := &types.AlertState{
		Type:              "http_test",
		CurrentState:      "ok",
		ConsecutiveChecks: 0,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	// Go critical with custom message
	alert := sm.updateState(state, "http_test", "critical", "Connection timeout", now, 0, 0)
	if alert != nil {
		t.Error("Expected no alert on first critical check")
	}

	// Need 3 consecutive checks
	state.ConsecutiveChecks = 3

	alert = sm.updateState(state, "http_test", "critical", "Connection timeout", now.Add(time.Second), 0, 0)
	if alert == nil {
		t.Fatal("Expected alert on third consecutive critical check")
	}
	if alert.Message != "Connection timeout" {
		t.Errorf("Expected custom message, got: %s", alert.Message)
	}
	if alert.Level != "critical" {
		t.Errorf("Expected level 'critical', got %s", alert.Level)
	}
}

func TestStateManager_CheckSystemMetric(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "ok",
		ConsecutiveChecks: 0,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	// Test critical threshold exceeded
	alert := sm.checkSystemMetric(state, "cpu", 90.0, 80.0, now)
	if alert != nil {
		t.Error("Expected no alert on first critical check")
	}
	if state.CurrentState != "critical" {
		t.Error("Expected state to be critical")
	}

	// Test below threshold (OK)
	state.CurrentState = "critical"
	state.ConsecutiveChecks = 3
	state.SentCriticalAlert = true

	alert = sm.checkSystemMetric(state, "cpu", 70.0, 80.0, now.Add(time.Second))
	if alert == nil {
		t.Fatal("Expected recovery alert when going below threshold")
	}
	if state.CurrentState != "ok" {
		t.Error("Expected state to be ok")
	}
}

func TestStateManager_UpdateSystemState(t *testing.T) {
	sm := NewStateManager()

	thresholds := &types.SystemChecksConfig{
		CPUThreshold:    80,
		MemoryThreshold: 85,
		DiskThreshold:   90,
	}

	// Test with all metrics critical
	stats := &types.SystemStats{
		CPUUsage: 90.0,
		MemoryUsage: types.MemoryStats{
			UsedPercent: 90.0,
		},
		DiskUsage: map[string]types.DiskStats{
			"/": {
				Path:        "/",
				UsedPercent: 95.0,
			},
		},
	}

	// First check - no alerts (need 3 consecutive)
	alerts := sm.UpdateSystemState(stats, thresholds)
	if len(alerts) != 0 {
		t.Errorf("Expected no alerts on first check, got %d", len(alerts))
	}

	// Run 2 more times to get 3 consecutive checks
	sm.UpdateSystemState(stats, thresholds)
	alerts = sm.UpdateSystemState(stats, thresholds)

	// Should have alerts for CPU, Memory, and Disk
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts after 3 consecutive checks, got %d", len(alerts))
	}

	// Verify alert types
	alertTypes := make(map[string]bool)
	for _, alert := range alerts {
		alertTypes[alert.Type] = true
	}

	if !alertTypes["cpu"] {
		t.Error("Expected CPU alert")
	}
	if !alertTypes["memory"] {
		t.Error("Expected Memory alert")
	}
	if !alertTypes["disk_/"] {
		t.Error("Expected Disk alert")
	}
}

func TestStateManager_UpdateHTTPState(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	// Test HTTP check failure
	results := []types.HTTPCheckResult{
		{
			Name:      "api",
			Success:   false,
			Error:     "Connection refused",
			Timestamp: now,
		},
	}

	// First failure - no alert
	alerts := sm.UpdateHTTPState(results)
	if len(alerts) != 0 {
		t.Errorf("Expected no alert on first failure, got %d", len(alerts))
	}

	// Second failure - no alert
	results[0].Timestamp = now.Add(time.Second)
	alerts = sm.UpdateHTTPState(results)
	if len(alerts) != 0 {
		t.Errorf("Expected no alert on second failure, got %d", len(alerts))
	}

	// Third failure - should alert
	results[0].Timestamp = now.Add(2 * time.Second)
	alerts = sm.UpdateHTTPState(results)
	if len(alerts) != 1 {
		t.Errorf("Expected alert on third failure, got %d", len(alerts))
	}

	if alerts[0].Type != "http_api" {
		t.Errorf("Expected alert type 'http_api', got %s", alerts[0].Type)
	}
	if alerts[0].Message != "Connection refused" {
		t.Errorf("Expected error message, got: %s", alerts[0].Message)
	}

	// Test HTTP check recovery
	results[0].Success = true
	results[0].Error = ""
	results[0].Timestamp = now.Add(3 * time.Second)

	alerts = sm.UpdateHTTPState(results)
	if len(alerts) != 1 {
		t.Errorf("Expected recovery alert, got %d", len(alerts))
	}
	if alerts[0].Message != "api has recovered" {
		t.Errorf("Expected recovery message, got: %s", alerts[0].Message)
	}
}

func TestStateManager_GetStates(t *testing.T) {
	sm := NewStateManager()

	// Initially empty
	states := sm.GetStates()
	if len(states) != 0 {
		t.Errorf("Expected empty states initially, got %d", len(states))
	}

	// Create some states
	sm.getOrCreateState("cpu")
	sm.getOrCreateState("memory")

	states = sm.GetStates()
	if len(states) != 2 {
		t.Errorf("Expected 2 states, got %d", len(states))
	}

	if _, exists := states["cpu"]; !exists {
		t.Error("Expected 'cpu' state to exist")
	}
	if _, exists := states["memory"]; !exists {
		t.Error("Expected 'memory' state to exist")
	}
}

func TestStateManager_ResetState(t *testing.T) {
	sm := NewStateManager()

	// Create a state
	sm.getOrCreateState("cpu")

	states := sm.GetStates()
	if len(states) != 1 {
		t.Errorf("Expected 1 state, got %d", len(states))
	}

	// Reset the state
	sm.ResetState("cpu")

	states = sm.GetStates()
	if len(states) != 0 {
		t.Errorf("Expected 0 states after reset, got %d", len(states))
	}

	// Reset non-existent state (should not panic)
	sm.ResetState("nonexistent")
}

func TestGetSystemAlertMessage(t *testing.T) {
	// Test CPU alert message
	msg := getSystemAlertMessage("cpu", 90.0, 80.0)
	expected := "CPU usage is 90.0% (threshold: 80.0%)"
	if msg != expected {
		t.Errorf("Expected CPU message '%s', got '%s'", expected, msg)
	}

	// Test Memory alert message
	msg = getSystemAlertMessage("memory", 95.0, 85.0)
	expected = "Memory usage is 95.0% (threshold: 85.0%)"
	if msg != expected {
		t.Errorf("Expected Memory message '%s', got '%s'", expected, msg)
	}

	// Test Disk alert message
	msg = getSystemAlertMessage("disk_/var", 95.0, 90.0)
	expected = "Disk usage on /var is 95.0% (threshold: 90.0%)"
	if msg != expected {
		t.Errorf("Expected Disk message '%s', got '%s'", expected, msg)
	}

	// Test unknown metric
	msg = getSystemAlertMessage("unknown", 50.0, 40.0)
	expected = "unknown is 50.0% (threshold: 40.0%)"
	if msg != expected {
		t.Errorf("Expected unknown metric message '%s', got '%s'", expected, msg)
	}
}

func TestGetSystemRecoveryMessage(t *testing.T) {
	// Test CPU recovery message
	msg := getSystemRecoveryMessage("cpu", 70.0, 80.0)
	expected := "CPU usage recovered to 70.0% (threshold: 80.0%)"
	if msg != expected {
		t.Errorf("Expected CPU recovery message '%s', got '%s'", expected, msg)
	}

	// Test Memory recovery message
	msg = getSystemRecoveryMessage("memory", 60.0, 85.0)
	expected = "Memory usage recovered to 60.0% (threshold: 85.0%)"
	if msg != expected {
		t.Errorf("Expected Memory recovery message '%s', got '%s'", expected, msg)
	}

	// Test Disk recovery message
	msg = getSystemRecoveryMessage("disk_/home", 80.0, 90.0)
	expected = "Disk usage on /home recovered to 80.0% (threshold: 90.0%)"
	if msg != expected {
		t.Errorf("Expected Disk recovery message '%s', got '%s'", expected, msg)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"integer", 50.0, "50.0%"},
		{"decimal", 50.5, "50.5%"},
		{"zero", 0.0, "0.0%"},
		{"high precision", 99.99, "100.0%"}, // rounds up
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.value)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %s, want %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"integer", 50.0, "50.0"},
		{"one decimal", 50.5, "50.5"},
		{"two decimals rounded", 50.55, "50.6"},
		{"zero", 0.0, "0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloat(tt.value)
			if result != tt.expected {
				t.Errorf("formatFloat(%v) = %s, want %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestFormatFloatPrecision(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		expected  string
	}{
		{"zero precision", 50.55, 0, "51"},
		{"one decimal", 50.55, 1, "50.6"},
		{"two decimals", 50.555, 2, "50.56"},
		{"three decimals", 50.5555, 3, "50.556"},
		{"zero value", 0.0, 1, "0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloatPrecision(tt.value, tt.precision)
			if result != tt.expected {
				t.Errorf("formatFloatPrecision(%v, %d) = %s, want %s",
					tt.value, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"multiple digits", 123, "123"},
		{"large number", 987654321, "987654321"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := itoa(tt.n)
			if result != tt.expected {
				t.Errorf("itoa(%d) = %s, want %s", tt.n, result, tt.expected)
			}
		})
	}
}
