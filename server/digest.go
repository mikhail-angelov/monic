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

	fmt.Fprintf(b, "  Total: %v  |  Running: %v  |  Stopped: %v\n\n", total, running, stopped)
}

func (ds *DigestService) writeIncidentSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("📈 CONTAINER INCIDENTS (24h)\n")

	failures := 0
	recoveries := 0
	for _, a := range ds.storage.GetAlerts() {
		if a.Timestamp.Before(cutoff) || a.Type != "docker" {
			continue
		}
		if a.Level == "critical" {
			failures++
		} else if a.Level == "info" && strings.Contains(a.Message, "recovered") {
			recoveries++
		}
	}

	fmt.Fprintf(b, "  Failures: %d\n", failures)
	fmt.Fprintf(b, "  Recoveries: %d\n\n", recoveries)
}

func (ds *DigestService) writeHTTPSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("🌐 HTTP CHECKS\n")

	total, passed, failed := 0, 0, 0
	latest := make(map[string]bool)
	for _, r := range ds.storage.GetHTTPCheckResults() {
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

	uptime := float64(passed) / float64(total) * 100
	fmt.Fprintf(b, "  Total checks: %d\n", total)
	fmt.Fprintf(b, "  Passed: %d  |  Failed: %d\n", passed, failed)
	fmt.Fprintf(b, "  Overall uptime: %.1f%%\n", uptime)
	for name, success := range latest {
		if !success {
			fmt.Fprintf(b, "  ⚠️  %s — last check failed\n", name)
		}
	}
	b.WriteString("\n")
}

func (ds *DigestService) writeSystemSection(b *strings.Builder, cutoff time.Time) {
	b.WriteString("💻 SYSTEM HEALTH\n")

	peakCPU, peakMem := 0.0, 0.0
	peakDisk := make(map[string]float64)
	var current *types.SystemStats

	stats := ds.storage.GetSystemStats()
	for i, s := range stats {
		if !s.Timestamp.After(cutoff) {
			continue
		}
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

	if current == nil {
		b.WriteString("  No system stats recorded yet.\n\n")
		return
	}

	pct := func(v float64) string { return fmt.Sprintf("%.1f%%", v) }

	fmt.Fprintf(b, "  CPU:    peak %s  /  current %s\n", pct(peakCPU), pct(current.CPUUsage))
	fmt.Fprintf(b, "  Memory: peak %s  /  current %s\n", pct(peakMem), pct(current.MemoryUsage.UsedPercent))
	if len(peakDisk) > 0 {
		b.WriteString("  Disk:\n")
		for path, peak := range peakDisk {
			if cur, ok := current.DiskUsage[path]; ok {
				fmt.Fprintf(b, "    %s: peak %s  /  current %s\n", path, pct(peak), pct(cur.UsedPercent))
			}
		}
	}
	b.WriteString("\n")

	thresholds := ds.systemMonitor.GetThresholds()
	if thresholds != nil {
		b.WriteString("  Thresholds:\n")
		if cpu, ok := thresholds["cpu_threshold"]; ok {
			fmt.Fprintf(b, "    CPU: > %.0f%%\n", cpu)
		}
		if mem, ok := thresholds["memory_threshold"]; ok {
			fmt.Fprintf(b, "    Memory: > %.0f%%\n", mem)
		}
		if disk, ok := thresholds["disk_threshold"]; ok {
			fmt.Fprintf(b, "    Disk: > %.0f%%\n", disk)
		}
	}
	b.WriteString("\n")
}
