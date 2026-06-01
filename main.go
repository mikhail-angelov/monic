package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bconf.com/monic/alert"
	"bconf.com/monic/config"
	"bconf.com/monic/discovery"
	"bconf.com/monic/monitor"
	"bconf.com/monic/server"
	"bconf.com/monic/types"
)

// version will be set during build
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Monic v%s\n", version)
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	systemMonitor := monitor.NewSystemMonitor(&cfg.SystemChecks)
	httpMonitor := monitor.NewHTTPMonitor()
	alertManager := alert.NewManager(&cfg.Alerting, cfg.AppName)
	stateManager := alert.NewStateManager()
	storage := server.NewStorageManager(100)
	containerTrack := monitor.NewContainerTracker()

	statsServer := server.NewStatsServer(
		&cfg.HTTPServer,
		systemMonitor,
		storage,
		containerTrack,
	)

	dockerWatcher, healthRegistry := initDockerWatcher(httpMonitor, cfg)

	var digestSvc *server.DigestService
	if cfg.Digest.Enabled {
		digestSvc = server.NewDigestService(storage, systemMonitor, containerTrack, cfg.AppName)
	}

	service := server.NewMonitorService(
		cfg,
		systemMonitor,
		httpMonitor,
		alertManager,
		stateManager,
		storage,
		statsServer,
		dockerWatcher,
		healthRegistry,
		containerTrack,
		digestSvc,
	)

	if err := service.Start(); err != nil {
		slog.Error("Failed to start monitoring service", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	service.Stop()
	slog.Info("Monic monitoring service shutdown complete")
}

// initDockerWatcher initializes the Docker discovery watcher.
// Returns nil values if the Docker daemon is unreachable.
func initDockerWatcher(httpMon *monitor.HTTPMonitor, cfg *types.Config) (*discovery.Watcher, *monitor.HealthCheckRegistry) {
	dockerClient, err := discovery.InitDockerClient()
	if err != nil {
		slog.Warn("Failed to initialize Docker client, Docker monitoring disabled", "error", err)
		return nil, nil
	}

	interval := time.Duration(cfg.DockerChecks.CheckInterval) * time.Second
	watcher := discovery.NewWatcher(dockerClient, interval)

	if monicID := os.Getenv("MONIC_CONTAINER_ID"); monicID != "" {
		watcher.ExcludeContainer(monicID)
	}

	return watcher, monitor.NewHealthCheckRegistry(httpMon)
}
