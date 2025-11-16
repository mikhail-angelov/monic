package monitor

import (
	"bconf.com/monic/v2/types"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// SimpleDockerMonitor handles Docker container monitoring using Docker CLI
type SimpleDockerMonitor struct {
	config *types.DockerConfig
}

// NewSimpleDockerMonitor creates a new simple Docker monitor instance
func NewSimpleDockerMonitor(config *types.DockerConfig) *SimpleDockerMonitor {
	return &SimpleDockerMonitor{
		config: config,
	}
}

// Initialize checks if Docker CLI is available
func (dm *SimpleDockerMonitor) Initialize() error {
	if !dm.config.Enabled {
		return nil
	}

	// Check if Docker CLI is available
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker CLI not available or Docker daemon not running: %w", err)
	}

	log.Println("Simple Docker monitor initialized successfully")
	return nil
}

// CheckContainers checks the status of Docker containers using Docker CLI
func (dm *SimpleDockerMonitor) CheckContainers() ([]types.DockerContainerStats, error) {
	if !dm.config.Enabled {
		return nil, nil
	}

	// Run docker ps with JSON output
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Parse JSON output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var stats []types.DockerContainerStats
	now := time.Now()

	for _, line := range lines {
		if line == "" {
			continue
		}

		var containerData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &containerData); err != nil {
			log.Printf("Warning: failed to parse container JSON: %v", err)
			continue
		}

		containerStats := types.DockerContainerStats{
			ContainerID:  getString(containerData["ID"]),
			Name:         getString(containerData["Names"]),
			Status:       getString(containerData["Status"]),
			State:        getString(containerData["State"]),
			Running:      strings.Contains(getString(containerData["State"]), "running"),
			RestartCount: 0, // Not available in basic docker ps
			Created:      now, // Not available in basic docker ps
			Timestamp:    now,
		}

		// Filter containers if specific ones are configured
		if len(dm.config.Containers) > 0 {
			found := false
			for _, targetContainer := range dm.config.Containers {
				if containerStats.Name == targetContainer || containerStats.Name == "/"+targetContainer {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Get detailed container info for restart count and exit code
		if containerInfo, err := dm.getContainerInfo(containerStats.ContainerID); err == nil {
			containerStats.RestartCount = containerInfo.RestartCount
			containerStats.ExitCode = containerInfo.ExitCode
			containerStats.Error = containerInfo.Error
		}

		stats = append(stats, containerStats)
	}

	return stats, nil
}

// CheckContainerStatus checks if specific containers are in the desired state
func (dm *SimpleDockerMonitor) CheckContainerStatus() ([]types.Alert, error) {
	if !dm.config.Enabled {
		return nil, nil
	}

	stats, err := dm.CheckContainers()
	if err != nil {
		return nil, err
	}

	var alerts []types.Alert
	now := time.Now()

	for _, container := range stats {
		// Check for stopped containers that should be running
		if !container.Running {
			alerts = append(alerts, types.Alert{
				Type:      "docker",
				Message:   fmt.Sprintf("Container %s (%s) is stopped", container.Name, container.ContainerID),
				Level:     "warning",
				Timestamp: now,
			})
		}

		// Check for containers with high restart counts
		if container.RestartCount > 10 {
			alerts = append(alerts, types.Alert{
				Type:      "docker",
				Message:   fmt.Sprintf("Container %s (%s) has high restart count: %d", container.Name, container.ContainerID, container.RestartCount),
				Level:     "warning",
				Timestamp: now,
			})
		}

		// Check for containers with non-zero exit codes
		if container.ExitCode != 0 && container.ExitCode != 137 { // 137 is SIGKILL, often intentional
			alerts = append(alerts, types.Alert{
				Type:      "docker",
				Message:   fmt.Sprintf("Container %s (%s) exited with error code: %d", container.Name, container.ContainerID, container.ExitCode),
				Level:     "critical",
				Timestamp: now,
			})
		}

		// Check for containers with errors
		if container.Error != "" {
			alerts = append(alerts, types.Alert{
				Type:      "docker",
				Message:   fmt.Sprintf("Container %s (%s) has error: %s", container.Name, container.ContainerID, container.Error),
				Level:     "critical",
				Timestamp: now,
			})
		}
	}

	return alerts, nil
}

// GetContainerSummary returns a summary of container status
func (dm *SimpleDockerMonitor) GetContainerSummary(stats []types.DockerContainerStats) map[string]interface{} {
	summary := make(map[string]interface{})
	
	total := len(stats)
	running := 0
	stopped := 0
	restarted := 0
	errored := 0

	for _, container := range stats {
		if container.Running {
			running++
		} else {
			stopped++
		}
		if container.RestartCount > 0 {
			restarted++
		}
		if container.ExitCode != 0 || container.Error != "" {
			errored++
		}
	}

	summary["total_containers"] = total
	summary["running_containers"] = running
	summary["stopped_containers"] = stopped
	summary["restarted_containers"] = restarted
	summary["errored_containers"] = errored

	if total > 0 {
		summary["running_percentage"] = float64(running) / float64(total) * 100
	} else {
		summary["running_percentage"] = 0.0
	}

	return summary
}

// getContainerInfo gets detailed container information using docker inspect
func (dm *SimpleDockerMonitor) getContainerInfo(containerID string) (*types.DockerContainerStats, error) {
	cmd := exec.Command("docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	var containerInfo []map[string]interface{}
	if err := json.Unmarshal(output, &containerInfo); err != nil {
		return nil, fmt.Errorf("failed to parse container inspect JSON: %w", err)
	}

	if len(containerInfo) == 0 {
		return nil, fmt.Errorf("no container info found")
	}

	info := containerInfo[0]
	state := info["State"].(map[string]interface{})

	stats := &types.DockerContainerStats{
		RestartCount: int(state["RestartCount"].(float64)),
		ExitCode:     int(state["ExitCode"].(float64)),
	}

	if errorMsg, ok := state["Error"].(string); ok && errorMsg != "" {
		stats.Error = errorMsg
	}

	return stats, nil
}

// getString safely extracts string from interface{}
func getString(value interface{}) string {
	if value == nil {
		return ""
	}
	return value.(string)
}
