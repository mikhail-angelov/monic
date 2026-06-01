package monitor

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"bconf.com/monic/types"
)

// ContainerStatus tracks the health state of a single container for alert deduplication.
type ContainerStatus struct {
	Name              string
	CustomName        string
	PrevRunning       bool
	CurrentRunning    bool
	CheckType         string
	ConsecutiveFails  int
	FailedSince       time.Time
	LastRecovery      time.Time
	SentCriticalAlert bool
}

// ContainerTracker manages the state of all monitored containers and generates alerts.
type ContainerTracker struct {
	mu         sync.RWMutex
	containers map[string]*ContainerStatus // containerID -> status
}

// NewContainerTracker creates a new container tracker.
func NewContainerTracker() *ContainerTracker {
	return &ContainerTracker{
		containers: make(map[string]*ContainerStatus),
	}
}

// UpdateFromEvent processes a container discovery event and returns alerts.
func (ct *ContainerTracker) UpdateFromEvent(containerID string, name, customName string, running bool, checkType string) []types.Alert {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	status, exists := ct.containers[containerID]
	if !exists {
		// New container
		ct.containers[containerID] = &ContainerStatus{
			Name:           name,
			CustomName:     customName,
			PrevRunning:    running,
			CurrentRunning: running,
			CheckType:      checkType,
		}
		if !running {
			return []types.Alert{{
				Type:      "docker",
				Message:   fmt.Sprintf("Container %s is not running (state: %s)", ct.displayName(name, customName), "stopped"),
				Level:     "critical",
				Timestamp: time.Now(),
			}}
		}
		return nil
	}

	// Update existing
	status.Name = name
	status.CustomName = customName
	status.PrevRunning = status.CurrentRunning
	status.CurrentRunning = running
	status.CheckType = checkType

	var alerts []types.Alert

	if status.PrevRunning && !status.CurrentRunning {
		// Running → stopped: critical alert
		alerts = append(alerts, types.Alert{
			Type:      "docker",
			Message:   fmt.Sprintf("Container %s stopped", ct.displayName(name, customName)),
			Level:     "critical",
			Timestamp: time.Now(),
		})
	} else if !status.PrevRunning && status.CurrentRunning {
		// Stopped → running: recovery
		alerts = append(alerts, types.Alert{
			Type:      "docker",
			Message:   fmt.Sprintf("Container %s recovered (now running)", ct.displayName(name, customName)),
			Level:     "info",
			Timestamp: time.Now(),
		})
	}

	return alerts
}

// Remove removes a container from tracking and returns a final alert if it was running.
func (ct *ContainerTracker) Remove(containerID string) []types.Alert {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	status, exists := ct.containers[containerID]
	if !exists {
		return nil
	}

	delete(ct.containers, containerID)

	if status.CurrentRunning {
		return []types.Alert{{
			Type:      "docker",
			Message:   fmt.Sprintf("Container %s disappeared from monitoring (was running)", ct.displayName(status.Name, status.CustomName)),
			Level:     "critical",
			Timestamp: time.Now(),
		}}
	}
	return nil
}

// GetSummary returns a summary of all tracked containers.
func (ct *ContainerTracker) GetSummary() map[string]any {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	total := len(ct.containers)
	running := 0
	stopped := 0

	for _, s := range ct.containers {
		if s.CurrentRunning {
			running++
		} else {
			stopped++
		}
	}

	return map[string]any{
		"total_containers":   total,
		"running_containers": running,
		"stopped_containers": stopped,
	}
}

// GetContainerStatuses returns all tracked container statuses for the web UI.
func (ct *ContainerTracker) GetContainerStatuses() []map[string]any {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make([]map[string]any, 0, len(ct.containers))
	for id, s := range ct.containers {
		result = append(result, map[string]any{
			"id":            id[:12],
			"name":          s.Name,
			"custom_name":   s.CustomName,
			"display_name":  ct.displayName(s.Name, s.CustomName),
			"running":       s.CurrentRunning,
			"check_type":    s.CheckType,
			"active_alert":  !s.CurrentRunning && s.SentCriticalAlert,
		})
	}
	return result
}

func (ct *ContainerTracker) displayName(name, customName string) string {
	if customName != "" && customName != name {
		return fmt.Sprintf("%s (%s)", customName, name)
	}
	return name
}

// Refactored: moved DockerContainerStats types to types.go.
// The old DockerMonitor and SimpleDockerMonitor are replaced by discovery.Watcher + ContainerTracker.

// Ensure backwards compatibility with importers
func init() {
	slog.Debug("Using new discovery-based Docker monitoring")
}
