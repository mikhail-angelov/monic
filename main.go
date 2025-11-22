package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"bconf.com/monic/server"
	"bconf.com/monic/types"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// loadConfig loads configuration from JSON file and environment variables
func loadConfig(configPath string) (*types.Config, error) {
	config := &types.Config{}

	// 1. Load from JSON file (Optional/Fallback)
	file, err := os.Open(configPath)
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Return error if file exists but cannot be opened
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	// 2. Load .env file (Optional)
	// It's okay if .env doesn't exist
	_ = godotenv.Load()

	// 3. Override with Environment Variables
	if err := envconfig.Process("MONIC", config); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	return config, nil
}

// version will be set during build
var version = "dev"

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Monic v%s\n", version)
		return
	}

	// Load configuration
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	config, err := loadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create and start monitoring service
	service := server.NewMonitorService(config)
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
