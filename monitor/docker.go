package monitor

import (
	"context"
	"fmt"
	"log"
	"time"

	"bconf.com/monic/types"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerMonitor handles Docker container monitoring
type DockerMonitor struct {
	config *types.DockerConfig
	client *client.Client
}

// NewDockerMonitor creates a new Docker monitor instance
func NewDockerMonitor(config *types.DockerConfig) *DockerMonitor {
	return &DockerMonitor{
		config: config,
	}
}

// Initialize initializes the Docker client
func (dm *DockerMonitor) Initialize() error {
	if !dm.config.Enabled {
		return nil
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	dm.client = cli

	// Test connection
	_, err = dm.client.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	log.Println("Docker monitor initialized successfully")
	return nil
}

// CheckContainers checks the status of Docker containers
func (dm *DockerMonitor) CheckContainers() ([]types.DockerContainerStats, error) {
	if !dm.config.Enabled || dm.client == nil {
		return nil, nil
	}

	ctx := context.Background()
	containers, err := dm.client.ContainerList(ctx, container.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var stats []types.DockerContainerStats
	now := time.Now()

	for _, c := range containers {
		// Filter containers if specific ones are configured
		if len(dm.config.Containers) > 0 {
			found := false
			for _, targetContainer := range dm.config.Containers {
				for _, name := range c.Names {
					if name == targetContainer || name == "/"+targetContainer {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		containerStats := types.DockerContainerStats{
			ContainerID:  c.ID[:12], // Short ID
			Name:         getContainerName(c.Names),
			Status:       c.Status,
			State:        c.State,
			Running:      c.State == "running",
			Created:      time.Unix(c.Created, 0),
			Timestamp:    now,
		}

		// Get detailed container info
		containerInfo, err := dm.client.ContainerInspect(ctx, c.ID)
		if err == nil {
			if containerInfo.State != nil {
				if containerInfo.State.Running {
					containerStats.StartedAt = containerInfo.State.StartedAt 
				} else {
					containerStats.FinishedAt = containerInfo.State.FinishedAt
					containerStats.ExitCode = containerInfo.State.ExitCode
					if containerInfo.State.Error != "" {
						containerStats.Error = containerInfo.State.Error
					}
				}
			}
		} else {
			log.Printf("Warning: failed to inspect container %s: %v", c.ID[:12], err)
		}

		stats = append(stats, containerStats)
	}

	return stats, nil
}

// CheckContainerStatus checks if specific containers are in the desired state
func (dm *DockerMonitor) CheckContainerStatus() ([]types.Alert, error) {
	if !dm.config.Enabled || dm.client == nil {
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
func (dm *DockerMonitor) GetContainerSummary(stats []types.DockerContainerStats) map[string]interface{} {
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

// Close closes the Docker client connection
func (dm *DockerMonitor) Close() error {
	if dm.client != nil {
		return dm.client.Close()
	}
	return nil
}

// getContainerName extracts the container name from the names array
func getContainerName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}

	// Docker container names start with "/", remove it
	name := names[0]
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	return name
}
