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

// Container lifecycle events emitted by Watcher.
const (
	EventAdded   EventType = iota
	EventRemoved
	EventUpdated
)

// ContainerEvent describes a change in the set of monitored containers.
type ContainerEvent struct {
	Type      EventType
	Container types.MonitoredContainer
	Previous  *types.MonitoredContainer // set for EventUpdated
}

// Watcher manages Docker container discovery via polling.
type Watcher struct {
	client   *http.Client
	interval time.Duration

	excludeMu  sync.RWMutex
	excludeIDs map[string]bool // containers to exclude (e.g. Monic itself)

	mu        sync.RWMutex
	monitored map[string]types.MonitoredContainer // key: containerID

	eventsCh chan ContainerEvent
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewWatcher creates a new Docker container watcher.
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
	w.excludeMu.Lock()
	defer w.excludeMu.Unlock()
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

// Start begins the polling loop. Blocks until the context is canceled or Stop is called.
func (w *Watcher) Start(ctx context.Context) error {
	slog.Info("Starting Docker container watcher", "interval", w.interval)

	w.poll(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("watcher context done: %w", ctx.Err())
		case <-ticker.C:
			w.poll(ctx)
		case <-w.stopCh:
			return nil
		}
	}
}

// Stop signals the watcher to stop. Safe to call multiple times.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
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

	w.excludeMu.RLock()
	excluded := make(map[string]bool, len(w.excludeIDs))
	for id := range w.excludeIDs {
		excluded[id] = true
	}
	w.excludeMu.RUnlock()

	discovered := make(map[string]types.MonitoredContainer)
	for _, c := range containers {
		if excluded[c.ID] {
			continue
		}
		mc := w.parseContainer(c)
		if mc == nil {
			continue
		}
		discovered[c.ID] = *mc
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for id, prev := range w.monitored {
		if _, exists := discovered[id]; !exists {
			slog.Info("Container removed from monitoring", "name", prev.Name, "id", shortID(id))
			w.emit(ContainerEvent{Type: EventRemoved, Container: prev})
		}
	}

	for id, curr := range discovered {
		prev, exists := w.monitored[id]
		if !exists {
			slog.Info("Container added to monitoring", "name", curr.Name, "id", shortID(id), "check_type", curr.CheckType)
			w.emit(ContainerEvent{Type: EventAdded, Container: curr})
		} else if containersChanged(prev, curr) {
			slog.Debug("Container updated in monitoring", "name", curr.Name, "id", shortID(id))
			w.emit(ContainerEvent{Type: EventUpdated, Container: curr, Previous: &prev})
		}
	}

	w.monitored = discovered
}

// listContainers fetches all containers via the Docker API over Unix socket.
func (w *Watcher) listContainers(ctx context.Context) ([]dockerContainer, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/containers/json?all=true", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker API returned status %d", resp.StatusCode)
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
		ID:         c.ID,
		Name:       name,
		CustomName: customName,
		Labels:     c.Labels,
		Running:    c.State == "running",
		Status:     c.State,
		CheckType:  types.CheckTypeContainer,
	}

	if checkVal, ok := c.Labels[types.LabelCheck]; ok {
		if strings.ToLower(strings.TrimSpace(checkVal)) == types.CheckTypeHTTP {
			mc.CheckType = types.CheckTypeHTTP
		}
	}

	if url, ok := c.Labels[types.LabelCheckHTTPURL]; ok && url != "" {
		mc.CheckType = types.CheckTypeHTTP
		mc.CheckHTTPURL = url
	}

	if mc.CheckType == types.CheckTypeHTTP {
		mc.CheckHTTPInterval = parseLabelInt(c.Labels, types.LabelCheckHTTPInterval, 30)
		mc.CheckHTTPTimeout = parseLabelInt(c.Labels, types.LabelCheckHTTPTimeout, 5)
		mc.CheckHTTPExpectedCode = parseLabelInt(c.Labels, types.LabelCheckHTTPExpected, 200)
	}

	return mc
}

func (w *Watcher) emit(evt ContainerEvent) {
	select {
	case w.eventsCh <- evt:
	default:
		slog.Warn("Container event channel full, dropping event",
			"type", evt.Type, "container", evt.Container.Name)
	}
}

func containersChanged(a, b types.MonitoredContainer) bool {
	return a.Running != b.Running ||
		a.CheckType != b.CheckType ||
		a.CheckHTTPURL != b.CheckHTTPURL ||
		a.CheckHTTPInterval != b.CheckHTTPInterval ||
		a.CheckHTTPTimeout != b.CheckHTTPTimeout ||
		a.CheckHTTPExpectedCode != b.CheckHTTPExpectedCode ||
		a.CustomName != b.CustomName
}

func extractName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	name := names[0]
	if name != "" && name[0] == '/' {
		name = name[1:]
	}
	return name
}

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

// shortID returns a safe 12-char prefix of a container ID for logging.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// InitDockerClient creates an HTTP client connected to Docker's Unix socket.
func InitDockerClient() (*http.Client, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/_ping", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	_ = resp.Body.Close()

	slog.Info("Docker client initialized successfully (plain HTTP over Unix socket)")
	return client, nil
}
