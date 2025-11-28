package config

import (
	"os"
	"strconv"

	"bconf.com/monic/types"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// LoadConfig loads configuration from environment variables only
func LoadConfig() (*types.Config, error) {
	config := &types.Config{}

	// Load .env file (Optional)
	// It's okay if .env doesn't exist
	_ = godotenv.Load()

	// Load from Environment Variables
	if err := envconfig.Process("MONIC", config); err != nil {
		return nil, err
	}

	// Calculate enabled status based on environment variables
	config = calculateEnabledStatus(config)

	// Handle single HTTP check from environment variables
	config = handleHTTPCheckFromEnv(config)

	return config, nil
}

// calculateEnabledStatus determines which features are enabled based on environment variables
func calculateEnabledStatus(config *types.Config) *types.Config {
	// Check if alerting is enabled (any alerting environment variables are set)
	config.Alerting.Enabled = isAlertingEnabled()

	// Check if email alerting is enabled
	config.Alerting.Email.Enabled = isEmailAlertingEnabled()

	// Check if mailgun alerting is enabled
	config.Alerting.Mailgun.Enabled = isMailgunAlertingEnabled()

	// Check if telegram alerting is enabled
	config.Alerting.Telegram.Enabled = isTelegramAlertingEnabled()

	// Check if docker checks are enabled
	config.DockerChecks.Enabled = isDockerChecksEnabled()

	// Check if HTTP server is enabled
	config.HTTPServer.Enabled = isHTTPServerEnabled()

	return config
}

// isAlertingEnabled checks if any alerting environment variables are set
func isAlertingEnabled() bool {
	return isEmailAlertingEnabled() || isMailgunAlertingEnabled() || isTelegramAlertingEnabled() ||
		os.Getenv("MONIC_ALERTING_LEVELS") != "" || os.Getenv("MONIC_ALERTING_COOLDOWN") != ""
}

// isEmailAlertingEnabled checks if email alerting environment variables are set
func isEmailAlertingEnabled() bool {
	return os.Getenv("MONIC_ALERTING_EMAIL_SMTP_HOST") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_SMTP_PORT") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_USERNAME") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_PASSWORD") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_FROM") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_TO") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_USE_TLS") != ""
}

// isMailgunAlertingEnabled checks if mailgun alerting environment variables are set
func isMailgunAlertingEnabled() bool {
	return os.Getenv("MONIC_MAILGUN_API_KEY") != "" ||
		os.Getenv("MONIC_MAILGUN_DOMAIN") != "" ||
		os.Getenv("MONIC_MAILGUN_FROM") != "" ||
		os.Getenv("MONIC_MAILGUN_TO") != "" ||
		os.Getenv("MONIC_MAILGUN_BASE_URL") != ""
}

// isTelegramAlertingEnabled checks if telegram alerting environment variables are set
func isTelegramAlertingEnabled() bool {
	return os.Getenv("MONIC_TELEGRAM_BOT_TOKEN") != "" ||
		os.Getenv("MONIC_TELEGRAM_CHAT_ID") != ""
}

// isDockerChecksEnabled checks if docker checks environment variables are set
func isDockerChecksEnabled() bool {
	return os.Getenv("MONIC_DOCKERCHECKS_CHECK_DOCKER_INTERVAL") != "" ||
		os.Getenv("MONIC_DOCKERCHECKS_CHECK_DOCKER_CONTAINERS") != ""
}

// isHTTPServerEnabled checks if HTTP server environment variables are set
func isHTTPServerEnabled() bool {
	return os.Getenv("MONIC_HTTPSERVER_HTTP_SERVER_PORT") != "" ||
		os.Getenv("MONIC_HTTPSERVER_HTTP_SERVER_USERNAME") != "" ||
		os.Getenv("MONIC_HTTPSERVER_HTTP_SERVER_PASSWORD") != ""
}

// handleHTTPCheckFromEnv creates an HTTP check from environment variables if configured
func handleHTTPCheckFromEnv(config *types.Config) *types.Config {
	// Check if HTTP check environment variables are set
	httpURL := os.Getenv("MONIC_CHECK_HTTP_URL")
	if httpURL == "" {
		return config
	}

	// Create a single HTTP check from environment variables
	httpCheck := types.HTTPCheck{
		Name:           "http-check",
		URL:            httpURL,
		Method:         os.Getenv("MONIC_CHECK_HTTP_METHOD"),
		Timeout:        10, // default
		ExpectedStatus: 200, // default
		CheckInterval:  300, // default
	}

	// Parse timeout if provided
	if timeoutStr := os.Getenv("MONIC_CHECK_HTTP_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			httpCheck.Timeout = timeout
		}
	}

	// Parse expected status if provided
	if statusStr := os.Getenv("MONIC_CHECK_HTTP_EXPECTED_STATUS"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			httpCheck.ExpectedStatus = status
		}
	}

	// Parse interval if provided
	if intervalStr := os.Getenv("MONIC_CHECK_HTTP_INTERVAL"); intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			httpCheck.CheckInterval = interval
		}
	}

	// Set method default if not provided
	if httpCheck.Method == "" {
		httpCheck.Method = "GET"
	}

	// Replace or add the HTTP check
	if len(config.HTTPChecks) == 0 {
		config.HTTPChecks = []types.HTTPCheck{httpCheck}
	} else {
		// Replace the first HTTP check with the environment-based one
		config.HTTPChecks[0] = httpCheck
	}

	return config
}
