package config

import (
	"fmt"
	"os"

	"bconf.com/monic/types"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// LoadConfig loads configuration from environment variables only
func LoadConfig() (*types.Config, error) {
	config := &types.Config{}

	// Load .env file (optional — ok if absent)
	_ = godotenv.Load()

	// Defaults applied before env vars so they can be overridden
	config.SystemChecks.Interval = 60  // 60 seconds
	config.DockerChecks.CheckInterval = 300 // 5 minutes

	// Load from environment variables
	if err := envconfig.Process("MONIC", config); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	config = calculateEnabledStatus(config)

	return config, nil
}

// calculateEnabledStatus determines which features are enabled based on environment variables.
// A feature's Enabled flag is only overridden when it is still false (not yet set by envconfig).
func calculateEnabledStatus(config *types.Config) *types.Config {
	if !config.Alerting.Email.Enabled {
		config.Alerting.Email.Enabled = isEmailAlertingEnabled()
	}
	if !config.Alerting.Mailgun.Enabled {
		config.Alerting.Mailgun.Enabled = isMailgunAlertingEnabled()
	}
	if !config.Alerting.Telegram.Enabled {
		config.Alerting.Telegram.Enabled = isTelegramAlertingEnabled()
	}

	// Docker monitoring is always enabled; if the socket is unavailable
	// InitDockerClient will log a warning and Docker monitoring is skipped.
	config.DockerChecks.Enabled = true

	if !config.HTTPServer.Enabled {
		config.HTTPServer.Enabled = isHTTPServerEnabled()
	}
	if !config.Digest.Enabled {
		config.Digest.Enabled = isDigestEnabled()
	}

	return config
}

func isEmailAlertingEnabled() bool {
	return os.Getenv("MONIC_ALERTING_EMAIL_SMTP_HOST") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_SMTP_PORT") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_USERNAME") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_PASSWORD") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_FROM") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_TO") != "" ||
		os.Getenv("MONIC_ALERTING_EMAIL_USE_TLS") != ""
}

func isMailgunAlertingEnabled() bool {
	return os.Getenv("MONIC_ALERTING_MAILGUN_API_KEY") != "" ||
		os.Getenv("MONIC_ALERTING_MAILGUN_DOMAIN") != "" ||
		os.Getenv("MONIC_ALERTING_MAILGUN_FROM") != "" ||
		os.Getenv("MONIC_ALERTING_MAILGUN_TO") != "" ||
		os.Getenv("MONIC_ALERTING_MAILGUN_BASE_URL") != ""
}

func isTelegramAlertingEnabled() bool {
	return os.Getenv("MONIC_ALERTING_TELEGRAM_BOT_TOKEN") != "" ||
		os.Getenv("MONIC_ALERTING_TELEGRAM_CHAT_ID") != ""
}

func isHTTPServerEnabled() bool {
	return os.Getenv("MONIC_HTTP_SERVER_PORT") != "" ||
		os.Getenv("MONIC_HTTP_SERVER_USERNAME") != "" ||
		os.Getenv("MONIC_HTTP_SERVER_PASSWORD") != ""
}

// isDigestEnabled returns true unless MONIC_DIGEST_ENABLED is explicitly set to "false".
func isDigestEnabled() bool {
	return os.Getenv("MONIC_DIGEST_ENABLED") != "false"
}
