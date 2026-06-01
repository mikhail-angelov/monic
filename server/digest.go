package server

import (
	"fmt"
	"strings"
	"time"

	"bconf.com/monic/monitor"
	"bconf.com/monic/types"
)

// DigestService builds daily status digest reports.
type DigestService struct {
	storage         Storage
	systemMonitor   systemMonitor
	containerTrack  *monitor.ContainerTracker
	appName         string
}

// systemMonitor is an interface for what DigestService needs from the system monitor.
type systemMonitor interface {
	GetThresholds() map[string]any
}

// NewDigestService creates a new digest service.
// dockerMonitor can be nil if Docker monitoring is disabled.
func NewDigestService(
	storage Storage,
	systemMonitor systemMonitor,
	containerTrack *monitor.ContainerTracker,
	appName string,
) *DigestService {
	return &DigestService{
		storage:         storage,
		systemMonitor:   systemMonitor,
		containerTrack:  containerTrack,
		appName:         appName,
	}
}

// BuildDigest assembles the full daily digest text.
func (ds *DigestService) BuildDigest() string {
	var b strings.Builder
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	appName := ds.appName
	if appName == "" {
		appName = "Monic"
	}

	// Header
	b.WriteString(fmt.Sprintf("%s Daily Digest — %s\n", appName, now.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("Generated: %s\n", now.Format(time.RFC1123)))
	b.WriteString(strings.Repeat("━", 50))
	b.WriteString("\n\n")

	// 1. Container summary
	ds.writeContainerSection(&b)

	// 2. Container incidents (last 24h)
	ds.writeIncidentSection(&b, cutoff)

	// 3. HTTP check summary
	ds.writeHTTPSection(&b, cutoff)

	// 4. System health (peak / current)
	ds.writeSystemSection(&b, cutoff)

	// Footer
	b.WriteString(strings.Repeat("━", 50))
	b.WriteString(fmt.Sprintf("\nEnd of %s Daily Digest\n", appName))

	return b.String()
}

func (ds *DigestService) writeContainerSection(b *strings.Builder) {
	b.WriteString("📊 MONITORED CONTAINERS\n")

	if ds.containerTrack == nil {
		b.WriteString("  Docker monitoring is disabled.\n\n")
		return
	}

	summary := ds.containerTrack.GetSummary()
	total := summary["total_containers"]
	running := summary["running_containers"]
	stopped := summary["stopped_containers"]

	b.WriteString(fmt.Sprintf("  Total: %v  |  Running: %v  |  Stopped: %v\n\n", total, running, stopped))
}

func (ds *DigestService) writeIncidentSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("📈 CONTAINER INCIDENTS (24h)\n")

	alerts := ds.storage.GetAlerts()
	failures := 0
	recoveries := 0

	for _, a := range alerts {
		if a.Timestamp.Before(cutoff) {
			continue
		}
		if a.Type == "docker" {
			if a.Level == "critical" {
				failures++
			} else if a.Level == "info" && strings.Contains(a.Message, "recovered") {
				recoveries++
			}
		}
	}

	b.WriteString(fmt.Sprintf("  Failures: %d\n", failures))
	b.WriteString(fmt.Sprintf("  Recoveries: %d\n", recoveries))
	b.WriteString("\n")
}

func (ds *DigestService) writeHTTPSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("🌐 HTTP CHECKS\n")

	results := ds.storage.GetHTTPCheckResults()
	total := 0
	passed := 0
	failed := 0

	// Track latest status per check name (24h only)
	latest := make(map[string]bool)
	for _, r := range results {
		if !r.Timestamp.After(cutoff) {
			continue
		}
		total++
		if r.Success {
			passed++
		} else {
			failed++
		}
		latest[r.Name] = r.Success
	}

	if total == 0 {
		b.WriteString("  No HTTP checks configured.\n\n")
		return
	}

	uptime := 0.0
	if total > 0 {
		uptime = float64(passed) / float64(total) * 100
	}
	b.WriteString(fmt.Sprintf("  Total checks: %d\n", total))
	b.WriteString(fmt.Sprintf("  Passed: %d  |  Failed: %d\n", passed, failed))
	b.WriteString(fmt.Sprintf("  Overall uptime: %.1f%%\n", uptime))

	// List failing checks
	for name, success := range latest {
		if !success {
			b.WriteString(fmt.Sprintf("  ⚠️  %s — last check failed\n", name))
		}
	}
	b.WriteString("\n")
}

func (ds *DigestService) writeSystemSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("💻 SYSTEM HEALTH\n")

	stats := ds.storage.GetSystemStats()

	// Peak values in last 24h
	peakCPU := 0.0
	peakMem := 0.0
	peakDisk := make(map[string]float64)
	var current *types.SystemStats

	for i, s := range stats {
		if s.Timestamp.After(cutoff) {
			if s.CPUUsage > peakCPU {
				peakCPU = s.CPUUsage
			}
			if s.MemoryUsage.UsedPercent > peakMem {
				peakMem = s.MemoryUsage.UsedPercent
			}
			for path, diskStat := range s.DiskUsage {
				if diskStat.UsedPercent > peakDisk[path] {
					peakDisk[path] = diskStat.UsedPercent
				}
			}
			current = &stats[i]
		}
	}

	if current == nil {
		b.WriteString("  No system stats recorded yet.\n\n")
		return
	}

	format := func(v float64) string {
		return fmt.Sprintf("%.1f%%", v)
	}

	b.WriteString(fmt.Sprintf("  CPU:    peak %s  /  current %s\n", format(peakCPU), format(current.CPUUsage)))
	b.WriteString(fmt.Sprintf("  Memory: peak %s  /  current %s\n", format(peakMem), format(current.MemoryUsage.UsedPercent)))

	if len(peakDisk) > 0 {
		b.WriteString("  Disk:\n")
		for path, peak := range peakDisk {
			if cur, ok := current.DiskUsage[path]; ok {
				b.WriteString(fmt.Sprintf("    %s: peak %s  /  current %s\n", path, format(peak), format(cur.UsedPercent)))
			}
		}
	}
	b.WriteString("\n")

	// Thresholds
	thresholds := ds.systemMonitor.GetThresholds()
	if thresholds != nil {
		b.WriteString("  Thresholds:\n")
		if cpu, ok := thresholds["cpu_threshold"]; ok {
			b.WriteString(fmt.Sprintf("    CPU: > %.0f%%\n", cpu))
		}
		if mem, ok := thresholds["memory_threshold"]; ok {
			b.WriteString(fmt.Sprintf("    Memory: > %.0f%%\n", mem))
		}
		if disk, ok := thresholds["disk_threshold"]; ok {
			b.WriteString(fmt.Sprintf("    Disk: > %.0f%%\n", disk))
		}
	}
	b.WriteString("\n")
}
