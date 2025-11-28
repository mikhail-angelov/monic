package main

import (
	"os"
	"testing"
)

func TestLoadConfig_EnvOnly(t *testing.T) {
	// Set environment variables
	os.Setenv("MONIC_APP_NAME", "TestApp")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_INTERVAL", "30")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_CPU_THRESHOLD", "80")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_MEMORY_THRESHOLD", "85")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_DISK_THRESHOLD", "90")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_DISK_PATHS", "/,/tmp")
	defer func() {
		os.Unsetenv("MONIC_APP_NAME")
		os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_INTERVAL")
		os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_CPU_THRESHOLD")
		os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_MEMORY_THRESHOLD")
		os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_DISK_THRESHOLD")
		os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_DISK_PATHS")
	}()

	// Test loading the config from environment variables
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches expected values
	if config.AppName != "TestApp" {
		t.Errorf("Expected AppName 'TestApp', got '%s'", config.AppName)
	}
	if config.SystemChecks.Interval != 30 {
		t.Errorf("Expected monitoring interval 30, got %d", config.SystemChecks.Interval)
	}
	if config.SystemChecks.CPUThreshold != 80 {
		t.Errorf("Expected CPU threshold 80, got %d", config.SystemChecks.CPUThreshold)
	}
	if config.SystemChecks.MemoryThreshold != 85 {
		t.Errorf("Expected memory threshold 85, got %d", config.SystemChecks.MemoryThreshold)
	}
	if config.SystemChecks.DiskThreshold != 90 {
		t.Errorf("Expected disk threshold 90, got %d", config.SystemChecks.DiskThreshold)
	}
	if len(config.SystemChecks.DiskPaths) != 2 || config.SystemChecks.DiskPaths[0] != "/" || config.SystemChecks.DiskPaths[1] != "/tmp" {
		t.Errorf("Expected disk paths ['/', '/tmp'], got %v", config.SystemChecks.DiskPaths)
	}
}

func TestLoadConfig_HTTPCheckFromEnv(t *testing.T) {
	// Set HTTP check environment variables
	os.Setenv("MONIC_CHECK_HTTP_URL", "http://localhost:8080/health")
	os.Setenv("MONIC_CHECK_HTTP_METHOD", "GET")
	os.Setenv("MONIC_CHECK_HTTP_TIMEOUT", "10")
	os.Setenv("MONIC_CHECK_HTTP_EXPECTED_STATUS", "200")
	os.Setenv("MONIC_CHECK_HTTP_INTERVAL", "30")
	defer func() {
		os.Unsetenv("MONIC_CHECK_HTTP_URL")
		os.Unsetenv("MONIC_CHECK_HTTP_METHOD")
		os.Unsetenv("MONIC_CHECK_HTTP_TIMEOUT")
		os.Unsetenv("MONIC_CHECK_HTTP_EXPECTED_STATUS")
		os.Unsetenv("MONIC_CHECK_HTTP_INTERVAL")
	}()

	// Test loading the config
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP check was created from environment variables
	if len(config.HTTPChecks) != 1 {
		t.Errorf("Expected 1 HTTP check, got %d", len(config.HTTPChecks))
	} else {
		httpCheck := config.HTTPChecks[0]
		if httpCheck.URL != "http://localhost:8080/health" {
			t.Errorf("Expected HTTP check URL 'http://localhost:8080/health', got '%s'", httpCheck.URL)
		}
		if httpCheck.Method != "GET" {
			t.Errorf("Expected HTTP check method 'GET', got '%s'", httpCheck.Method)
		}
		if httpCheck.Timeout != 10 {
			t.Errorf("Expected HTTP check timeout 10, got %d", httpCheck.Timeout)
		}
		if httpCheck.ExpectedStatus != 200 {
			t.Errorf("Expected HTTP check expected status 200, got %d", httpCheck.ExpectedStatus)
		}
		if httpCheck.CheckInterval != 30 {
			t.Errorf("Expected HTTP check interval 30, got %d", httpCheck.CheckInterval)
		}
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("MONIC_APP_NAME", "EnvApp")
	os.Setenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_INTERVAL", "60")
	defer os.Unsetenv("MONIC_APP_NAME")
	defer os.Unsetenv("MONIC_SYSTEMCHECKS_CHECK_SYSTEM_INTERVAL")

	// Load config
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.AppName != "EnvApp" {
		t.Errorf("Expected AppName 'EnvApp', got '%s'", config.AppName)
	}
	if config.SystemChecks.Interval != 60 {
		t.Errorf("Expected Interval 60, got %d", config.SystemChecks.Interval)
	}
}
