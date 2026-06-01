// Package discovery implements label-based Docker container discovery for Monic.
// It polls the Docker API at a configurable interval via plain HTTP over Unix socket,
// filters containers by monic.* labels, and emits events when containers are added,
// removed, or updated.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"bconf.com/monic/types"
)

// EventType represents the type of change detected in monitored containers.
type EventType int

const (
	EventAdded   EventType = iota
	EventRemoved
	EventUpdated
)

// ContainerEvent describes a change in the set of monitored containers.
type ContainerEvent struct {
	Type         EventType
	Container    types.MonitoredContainer
	Previous     *types.MonitoredContainer // set for EventUpdated
}

// Watcher manages Docker container discovery via polling.
type Watcher struct {
	client     *http.Client
	interval   time.Duration
	excludeIDs map[string]bool // containers to exclude (e.g. Monic itself)

	mu       sync.RWMutex
	monitored map[string]types.MonitoredContainer // key: containerID

	eventsCh chan ContainerEvent
	stopCh   chan struct{}
}

// NewWatcher creates a new Docker container watcher.
// The interval parameter controls how often containers are polled.
func NewWatcher(dockerClient *http.Client, interval time.Duration) *Watcher {
	return &Watcher{
		client:     dockerClient,
		interval:   interval,
		excludeIDs: make(map[string]bool),
		monitored:  make(map[string]types.MonitoredContainer),
		eventsCh:   make(chan ContainerEvent, 64),
		stopCh:     make(chan struct{}),
	}
}

// ExcludeContainer marks a container ID to be excluded from monitoring.
func (w *Watcher) ExcludeContainer(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.excludeIDs[id] = true
}

// Events returns a read-only channel of container events.
func (w *Watcher) Events() <-chan ContainerEvent {
	return w.eventsCh
}

// GetMonitored returns a snapshot of currently monitored containers.
func (w *Watcher) GetMonitored() []types.MonitoredContainer {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]types.MonitoredContainer, 0, len(w.monitored))
	for _, c := range w.monitored {
		result = append(result, c)
	}
	return result
}

// Start begins the polling loop. Blocks until the context is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	slog.Info("Starting Docker container watcher",
		"interval", w.interval)

	// Do an immediate first poll
	w.poll(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.poll(ctx)
		case <-w.stopCh:
			return nil
		}
	}
}

// Stop signals the watcher to stop.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// dockerContainer is a minimal representation of a Docker container from the API.
type dockerContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// poll fetches all containers, filters by labels, and emits change events.
func (w *Watcher) poll(ctx context.Context) {
	containers, err := w.listContainers(ctx)
	if err != nil {
		slog.Error("Failed to list Docker containers", "error", err)
		return
	}

	// Build a map of newly discovered monitored containers
	discovered := make(map[string]types.MonitoredContainer)

	for _, c := range containers {
		// Skip excluded containers
		if w.excludeIDs[c.ID] {
			continue
		}

		mc := w.parseContainer(c)
		if mc == nil {
			continue // no monic.enabled=true label or explicitly disabled
		}

		discovered[c.ID] = *mc
	}

	// Compare with previous state and emit events
	w.mu.Lock()
	defer w.mu.Unlock()

	// Find removed containers
	for id, prev := range w.monitored {
		if _, exists := discovered[id]; !exists {
			slog.Info("Container removed from monitoring",
				"name", prev.Name, "id", id[:12])
			w.emit(ContainerEvent{Type: EventRemoved, Container: prev})
		}
	}

	// Find added and updated containers
	for id, curr := range discovered {
		prev, exists := w.monitored[id]
		if !exists {
			slog.Info("Container added to monitoring",
				"name", curr.Name, "id", id[:12], "check_type", curr.CheckType)
			w.emit(ContainerEvent{Type: EventAdded, Container: curr})
		} else if containersChanged(prev, curr) {
			slog.Debug("Container updated in monitoring",
				"name", curr.Name, "id", id[:12])
			w.emit(ContainerEvent{Type: EventUpdated, Container: curr, Previous: &prev})
		}
	}

	w.monitored = discovered
}

// listContainers fetches all containers via the Docker API over Unix socket.
func (w *Watcher) listContainers(ctx context.Context) ([]dockerContainer, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/containers/json?all=true", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker API returned status %d", resp.StatusCode)
	}

	var containers []dockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode container list: %w", err)
	}

	return containers, nil
}

// parseContainer extracts monitoring info from a Docker container.
// Returns nil if the container doesn't have monic.enabled=true or is disabled.
func (w *Watcher) parseContainer(c dockerContainer) *types.MonitoredContainer {
	enabledVal, hasEnabled := c.Labels[types.LabelEnabled]
	if !hasEnabled {
		return nil
	}

	// Check if explicitly disabled
	enabledVal = strings.ToLower(strings.TrimSpace(enabledVal))
	if enabledVal == "false" || enabledVal == "no" || enabledVal == "0" {
		return nil
	}

	name := extractName(c.Names)
	customName := c.Labels[types.LabelName]
	if customName == "" {
		customName = name
	}

	mc := &types.MonitoredContainer{
		ID:        c.ID,
		Name:      name,
		CustomName: customName,
		Labels:    c.Labels,
		Running:   c.State == "running",
		Status:    c.State,
		CheckType: types.CheckTypeContainer, // default: status only
	}

	// Determine check type
	checkVal, hasCheck := c.Labels[types.LabelCheck]
	if hasCheck && strings.ToLower(strings.TrimSpace(checkVal)) == types.CheckTypeHTTP {
		mc.CheckType = types.CheckTypeHTTP
	}

	// Parse HTTP check parameters
	if url, ok := c.Labels[types.LabelCheckHTTPURL]; ok && url != "" {
		mc.CheckType = types.CheckTypeHTTP // implicitly enable HTTP check
		mc.CheckHTTPURL = url
	}

	if mc.CheckType == types.CheckTypeHTTP {
		mc.CheckHTTPInterval = parseLabelInt(c.Labels, types.LabelCheckHTTPInterval, 30)
		mc.CheckHTTPTimeout = parseLabelInt(c.Labels, types.LabelCheckHTTPTimeout, 5)
		mc.CheckHTTPExpectedCode = parseLabelInt(c.Labels, types.LabelCheckHTTPExpected, 200)
	}

	return mc
}

// emit sends an event to the channel (non-blocking).
func (w *Watcher) emit(evt ContainerEvent) {
	select {
	case w.eventsCh <- evt:
	default:
		slog.Warn("Container event channel full, dropping event",
			"type", evt.Type, "container", evt.Container.Name)
	}
}

// containersChanged returns true if the monitored properties differ.
func containersChanged(a, b types.MonitoredContainer) bool {
	return a.Running != b.Running ||
		a.CheckType != b.CheckType ||
		a.CheckHTTPURL != b.CheckHTTPURL ||
		a.CheckHTTPInterval != b.CheckHTTPInterval ||
		a.CheckHTTPTimeout != b.CheckHTTPTimeout ||
		a.CheckHTTPExpectedCode != b.CheckHTTPExpectedCode ||
		a.CustomName != b.CustomName
}

// extractName extracts the container name from Docker's names array.
func extractName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	name := names[0]
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	return name
}

// parseLabelInt parses an integer label value with a default.
func parseLabelInt(labels map[string]string, key string, defaultVal int) int {
	val, ok := labels[key]
	if !ok || val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		slog.Warn("Invalid integer label", "label", key, "value", val, "default", defaultVal)
		return defaultVal
	}
	if n <= 0 {
		return defaultVal
	}
	return n
}

// unixDialer returns a net.Dialer suitable for connecting to Docker's Unix socket.
func unixDialer() *net.Dialer {
	return &net.Dialer{Timeout: 5 * time.Second}
}

// InitDockerClient creates an HTTP client connected to Docker's Unix socket,
// using plain HTTP without the Docker SDK library.
func InitDockerClient() (*http.Client, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return unixDialer().DialContext(ctx, "unix", "/var/run/docker.sock")
			},
		},
	}

	// Verify connectivity by pinging the Docker API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/_ping", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	resp.Body.Close()

	slog.Info("Docker client initialized successfully (plain HTTP over Unix socket)")
	return client, nil
}
