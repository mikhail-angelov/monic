package main

import (
	"encoding/json"
	"os"
	"testing"

	"bconf.com/monic/types"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	configData := types.Config{
		SystemChecks: types.SystemChecksConfig{
			Interval:        30,
			CPUThreshold:    80,
			MemoryThreshold: 85,
			DiskThreshold:   90,
			DiskPaths:       []string{"/", "/tmp"},
		},
		HTTPChecks: []types.HTTPCheck{
			{
				Name:           "test",
				URL:            "http://localhost:8080/health",
				Method:         "GET",
				Timeout:        10,
				ExpectedStatus: 200,
				CheckInterval:  30,
			},
		},
	}

	// Write config to temporary file
	configFile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(configFile.Name())

	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(configData); err != nil {
		t.Fatalf("Failed to write config to file: %v", err)
	}
	configFile.Close()

	// Test loading the config
	loadedConfig, err := loadConfig(configFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches expected values
	if loadedConfig.SystemChecks.Interval != 30 {
		t.Errorf("Expected monitoring interval 30, got %d", loadedConfig.SystemChecks.Interval)
	}
	if loadedConfig.SystemChecks.CPUThreshold != 80 {
		t.Errorf("Expected CPU threshold 80, got %d", loadedConfig.SystemChecks.CPUThreshold)
	}
	if len(loadedConfig.SystemChecks.DiskPaths) != 2 {
		t.Errorf("Expected 2 disk paths, got %d", len(loadedConfig.SystemChecks.DiskPaths))
	}
	if len(loadedConfig.HTTPChecks) != 1 {
		t.Errorf("Expected 1 HTTP check, got %d", len(loadedConfig.HTTPChecks))
	}
	if loadedConfig.HTTPChecks[0].Name != "test" {
		t.Errorf("Expected HTTP check name 'test', got '%s'", loadedConfig.HTTPChecks[0].Name)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("/non/existent/path/config.json")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	// Create a temporary file with invalid JSON
	configFile, err := os.CreateTemp("", "test-invalid-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(configFile.Name())

	// Write invalid JSON
	configFile.WriteString("{ invalid json }")
	configFile.Close()

	_, err = loadConfig(configFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON config")
	}
}
