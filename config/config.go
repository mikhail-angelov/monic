package config

import (
	"os"

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

	return config, nil
}


// calculateEnabledStatus determines which features are enabled based on environment variables
func calculateEnabledStatus(config *types.Config) *types.Config {
	
	// Only set enabled status if not already set by envconfig
	if !config.Alerting.Email.Enabled {
		config.Alerting.Email.Enabled = isEmailAlertingEnabled()
	}

	if !config.Alerting.Mailgun.Enabled {
		config.Alerting.Mailgun.Enabled = isMailgunAlertingEnabled()
	}

	if !config.Alerting.Telegram.Enabled {
		config.Alerting.Telegram.Enabled = isTelegramAlertingEnabled()
	}

	if !config.DockerChecks.Enabled {
		config.DockerChecks.Enabled = isDockerChecksEnabled()
	}

	if !config.HTTPServer.Enabled {
		config.HTTPServer.Enabled = isHTTPServerEnabled()
	}

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
