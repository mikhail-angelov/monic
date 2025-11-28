package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"bconf.com/monic/config"
	"bconf.com/monic/server"
)

// version will be set during build
var version = "dev"

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Monic v%s\n", version)
		return
	}

	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create and start monitoring service
	service := server.NewMonitorService(cfg)
	if err := service.Start(); err != nil {
		slog.Error("Failed to start monitoring service", "error", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	service.Stop()
	slog.Info("Monic monitoring service shutdown complete")
}
