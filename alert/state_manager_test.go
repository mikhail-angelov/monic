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

	state2 := sm.getOrCreateState("test_metric")
	if state2 != state1 {
		t.Error("Expected to get same state instance for same metric")
	}

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

	state := &types.AlertState{
		Type:              "test",
		CurrentState:      "critical",
		ConsecutiveChecks: 3,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	if !sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return true for critical state with 3 consecutive checks")
	}

	state.ConsecutiveChecks = 2
	if sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return false for critical state with < 3 consecutive checks")
	}

	state.ConsecutiveChecks = 3
	state.LastAlertSent = now.Add(-5 * time.Minute) // after LastStateChange
	if sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return false when alert already sent for this state change")
	}
}

func TestStateManager_ShouldSendAlert_OkState(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "test",
		CurrentState:      "ok",
		ConsecutiveChecks: 1,
		LastAlertSent:     now.Add(-5 * time.Minute),
		LastStateChange:   now.Add(-1 * time.Minute),
		SentCriticalAlert: true,
	}

	if !sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return true for recovery alert")
	}

	state.ConsecutiveChecks = 2
	if sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return false for non-first OK check")
	}

	state.ConsecutiveChecks = 1
	state.LastAlertSent = now.Add(-5 * time.Minute)
	state.SentCriticalAlert = false
	if sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return false when SentCriticalAlert is false")
	}

	state.LastAlertSent = now
	state.SentCriticalAlert = true
	if sm.shouldSendAlert(state) {
		t.Error("Expected shouldSendAlert to return false when alert sent after state change")
	}
}

func TestStateManager_UpdateState_CriticalAlert(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "ok",
		ConsecutiveChecks: 5,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

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

	alert = sm.updateState(state, "cpu", "critical", "", now.Add(time.Second), 91.0, 80.0)
	if alert != nil {
		t.Error("Expected no alert on second critical check")
	}
	if state.ConsecutiveChecks != 2 {
		t.Errorf("Expected consecutive checks to be 2, got %d", state.ConsecutiveChecks)
	}

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
}

func TestStateManager_UpdateState_RecoveryAlert(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "critical",
		ConsecutiveChecks: 3,
		LastAlertSent:     now.Add(-5 * time.Minute),
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: true,
	}

	alert := sm.updateState(state, "cpu", "ok", "", now, 70.0, 80.0)
	if alert == nil {
		t.Fatal("Expected recovery alert when recovering from critical state")
	}
	if alert.Level != "info" {
		t.Errorf("Expected alert level 'info' for recovery, got %s", alert.Level)
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
}

func TestStateManager_UpdateState_NoRecoveryForTemporaryBlip(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "cpu",
		CurrentState:      "critical",
		ConsecutiveChecks: 2,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-5 * time.Minute),
		SentCriticalAlert: false,
	}

	alert := sm.updateState(state, "cpu", "ok", "", now, 70.0, 80.0)
	if alert != nil {
		t.Error("Expected no recovery alert for temporary blip (no critical alert sent)")
	}
}

func TestStateManager_UpdateState_WithMessage(t *testing.T) {
	sm := NewStateManager()
	now := time.Now()

	state := &types.AlertState{
		Type:              "http_test",
		CurrentState:      "ok",
		ConsecutiveChecks: 0,
		LastAlertSent:     time.Time{},
		LastStateChange:   now.Add(-10 * time.Minute),
		SentCriticalAlert: false,
	}

	alert := sm.updateState(state, "http_test", "critical", "Connection timeout", now, 0, 0)
	if alert != nil {
		t.Error("Expected no alert on first critical check")
	}

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
	}

	alert := sm.checkSystemMetric(state, "cpu", 90.0, 80.0, now)
	if alert != nil {
		t.Error("Expected no alert on first critical check")
	}
	if state.CurrentState != "critical" {
		t.Error("Expected state to be critical")
	}

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

	stats := &types.SystemStats{
		CPUUsage:    90.0,
		MemoryUsage: types.MemoryStats{UsedPercent: 90.0},
		DiskUsage:   map[string]types.DiskStats{"/": {Path: "/", UsedPercent: 95.0}},
	}

	alerts := sm.UpdateSystemState(stats, thresholds)
	if len(alerts) != 0 {
		t.Errorf("Expected no alerts on first check, got %d", len(alerts))
	}

	sm.UpdateSystemState(stats, thresholds)
	alerts = sm.UpdateSystemState(stats, thresholds)

	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts after 3 consecutive checks, got %d", len(alerts))
	}

	alertTypes := make(map[string]bool)
	for _, a := range alerts {
		alertTypes[a.Type] = true
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

	results := []types.HTTPCheckResult{
		{Name: "api", Success: false, Error: "Connection refused", Timestamp: now},
	}

	alerts := sm.UpdateHTTPState(results)
	if len(alerts) != 0 {
		t.Errorf("Expected no alert on first failure, got %d", len(alerts))
	}

	results[0].Timestamp = now.Add(time.Second)
	alerts = sm.UpdateHTTPState(results)
	if len(alerts) != 0 {
		t.Errorf("Expected no alert on second failure, got %d", len(alerts))
	}

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

	states := sm.GetStates()
	if len(states) != 0 {
		t.Errorf("Expected empty states initially, got %d", len(states))
	}

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

	sm.getOrCreateState("cpu")
	if len(sm.GetStates()) != 1 {
		t.Errorf("Expected 1 state, got %d", len(sm.GetStates()))
	}

	sm.ResetState("cpu")
	if len(sm.GetStates()) != 0 {
		t.Errorf("Expected 0 states after reset, got %d", len(sm.GetStates()))
	}

	sm.ResetState("nonexistent") // must not panic
}

func TestGetSystemAlertMessage(t *testing.T) {
	tests := []struct {
		alertType string
		value     float64
		threshold float64
		expected  string
	}{
		{"cpu", 90.0, 80.0, "CPU usage is 90.0% (threshold: 80.0%)"},
		{"memory", 95.0, 85.0, "Memory usage is 95.0% (threshold: 85.0%)"},
		{"disk_/var", 95.0, 90.0, "Disk usage on /var is 95.0% (threshold: 90.0%)"},
		{"unknown", 50.0, 40.0, "unknown is 50.0% (threshold: 40.0%)"},
	}
	for _, tt := range tests {
		got := getSystemAlertMessage(tt.alertType, tt.value, tt.threshold)
		if got != tt.expected {
			t.Errorf("getSystemAlertMessage(%q): got %q, want %q", tt.alertType, got, tt.expected)
		}
	}
}

func TestGetSystemRecoveryMessage(t *testing.T) {
	tests := []struct {
		alertType string
		value     float64
		threshold float64
		expected  string
	}{
		{"cpu", 70.0, 80.0, "CPU usage recovered to 70.0% (threshold: 80.0%)"},
		{"memory", 60.0, 85.0, "Memory usage recovered to 60.0% (threshold: 85.0%)"},
		{"disk_/home", 80.0, 90.0, "Disk usage on /home recovered to 80.0% (threshold: 90.0%)"},
	}
	for _, tt := range tests {
		got := getSystemRecoveryMessage(tt.alertType, tt.value, tt.threshold)
		if got != tt.expected {
			t.Errorf("getSystemRecoveryMessage(%q): got %q, want %q", tt.alertType, got, tt.expected)
		}
	}
}
