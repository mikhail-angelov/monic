package alert

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"bconf.com/monic/types"
)

// AlertManager handles sending alerts via configured channels
type AlertManager struct {
	config   *types.AlertingConfig
	appName  string
	lastSent map[string]time.Time // Track last sent alerts to avoid spam
}

// NewAlertManager creates a new alert manager instance
func NewAlertManager(config *types.AlertingConfig, appName string) *AlertManager {
	return &AlertManager{
		config:   config,
		appName:  appName,
		lastSent: make(map[string]time.Time),
	}
}

// SendAlert sends an alert through all configured channels
func (am *AlertManager) SendAlert(alert types.Alert) error {
	if !am.config.Enabled {
		return nil // Alerting disabled
	}

	// Check if we should send this alert based on level
	if !am.shouldSendLevel(alert.Level) {
		return nil
	}

	// Check cooldown period
	if !am.shouldSendCooldown(alert) {
		return nil
	}

	var errors []string

	// Send via SMTP email if enabled
	if am.config.Email.Enabled {
		if err := am.sendEmail(alert); err != nil {
			errors = append(errors, fmt.Sprintf("email: %v", err))
		}
	}

	// Send via Mailgun if enabled
	if am.config.Mailgun.Enabled {
		if err := am.sendMailgun(alert); err != nil {
			errors = append(errors, fmt.Sprintf("mailgun: %v", err))
		}
	}

	// Send via Telegram if enabled
	if am.config.Telegram.Enabled {
		if err := am.sendTelegram(alert); err != nil {
			errors = append(errors, fmt.Sprintf("telegram: %v", err))
		}
	}

	// Update last sent time
	am.lastSent[alert.Type] = time.Now()

	if len(errors) > 0 {
		return fmt.Errorf("failed to send alerts: %s", strings.Join(errors, "; "))
	}

	log.Printf("Alert sent: [%s] %s", alert.Level, alert.Message)
	return nil
}

// shouldSendLevel checks if the alert level should be sent
func (am *AlertManager) shouldSendLevel(level string) bool {
	if len(am.config.AlertLevels) == 0 {
		// If no levels specified, send all
		return true
	}

	for _, configuredLevel := range am.config.AlertLevels {
		if configuredLevel == level {
			return true
		}
	}
	return false
}

// shouldSendCooldown checks if enough time has passed since the last alert of this type
func (am *AlertManager) shouldSendCooldown(alert types.Alert) bool {
	if am.config.Cooldown <= 0 {
		return true // No cooldown configured
	}

	lastSent, exists := am.lastSent[alert.Type]
	if !exists {
		return true // Never sent this type before
	}

	cooldownDuration := time.Duration(am.config.Cooldown) * time.Minute
	return time.Since(lastSent) >= cooldownDuration
}

// sendEmail sends an alert via SMTP email
func (am *AlertManager) sendEmail(alert types.Alert) error {
	emailConfig := am.config.Email

	// Validate email configuration
	if emailConfig.SMTPHost == "" || emailConfig.SMTPPort == 0 {
		return fmt.Errorf("SMTP host and port must be configured")
	}
	if emailConfig.From == "" || emailConfig.To == "" {
		return fmt.Errorf("from and to email addresses must be configured")
	}

	// Create email message
	appName := am.getAppName()
	subject := fmt.Sprintf("[%s Alert] %s - %s", appName, strings.ToUpper(alert.Level), alert.Type)
	body := am.buildEmailBody(alert)

	// Build message headers
	headers := make(map[string]string)
	headers["From"] = emailConfig.From
	headers["To"] = emailConfig.To
	headers["Subject"] = subject
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	// Build message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Connect to SMTP server
	auth := smtp.PlainAuth("", emailConfig.Username, emailConfig.Password, emailConfig.SMTPHost)
	addr := fmt.Sprintf("%s:%d", emailConfig.SMTPHost, emailConfig.SMTPPort)

	if emailConfig.UseTLS {
		// Use STARTTLS (required for Gmail)
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP dial failed: %w", err)
		}
		defer client.Close()

		// Start TLS
		tlsconfig := &tls.Config{
			ServerName: emailConfig.SMTPHost,
		}
		if err = client.StartTLS(tlsconfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}

		// Auth
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}

		// Send email
		if err = client.Mail(emailConfig.From); err != nil {
			return fmt.Errorf("SMTP MAIL failed: %w", err)
		}
		if err = client.Rcpt(emailConfig.To); err != nil {
			return fmt.Errorf("SMTP RCPT failed: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA failed: %w", err)
		}
		defer w.Close()

		_, err = w.Write([]byte(message))
		if err != nil {
			return fmt.Errorf("SMTP message write failed: %w", err)
		}
	} else {
		// Use plain SMTP
		err := smtp.SendMail(addr, auth, emailConfig.From, []string{emailConfig.To}, []byte(message))
		if err != nil {
			return fmt.Errorf("SMTP send failed: %w", err)
		}
	}

	return nil
}

// getAppName returns the application name, defaulting to "Monic" if not configured
func (am *AlertManager) getAppName() string {
	if am.appName != "" {
		return am.appName
	}
	return "Monic"
}

// sendMailgun sends an alert via Mailgun API
func (am *AlertManager) sendMailgun(alert types.Alert) error {
	mailgunConfig := am.config.Mailgun

	// Validate Mailgun configuration
	if mailgunConfig.APIKey == "" {
		return fmt.Errorf("Mailgun API key must be configured")
	}
	if mailgunConfig.Domain == "" {
		return fmt.Errorf("Mailgun domain must be configured")
	}
	if mailgunConfig.From == "" || mailgunConfig.To == "" {
		return fmt.Errorf("from and to email addresses must be configured")
	}

	// Set default base URL if not provided
	baseURL := mailgunConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://api.mailgun.net/v3"
	}

	// Build request data
	appName := am.getAppName()
	subject := fmt.Sprintf("[%s Alert] %s - %s", appName, strings.ToUpper(alert.Level), alert.Type)
	body := am.buildEmailBody(alert)

	formData := map[string]string{
		"from":    mailgunConfig.From,
		"to":      mailgunConfig.To,
		"subject": subject,
		"text":    body,
	}

	// Create request
	url := fmt.Sprintf("%s/%s/messages", baseURL, mailgunConfig.Domain)
	formBytes, err := json.Marshal(formData)
	if err != nil {
		return fmt.Errorf("failed to encode form data: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(formBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth("api", mailgunConfig.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Mailgun API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Mailgun API returned status %d", resp.StatusCode)
	}

	return nil
}

// sendTelegram sends an alert via Telegram Bot API
func (am *AlertManager) sendTelegram(alert types.Alert) error {
	telegramConfig := am.config.Telegram

	// Validate Telegram configuration
	if telegramConfig.BotToken == "" {
		return fmt.Errorf("Telegram bot token must be configured")
	}
	if telegramConfig.ChatID == "" {
		return fmt.Errorf("Telegram chat ID must be configured")
	}

	// Build message
	appName := am.getAppName()
	message := fmt.Sprintf("<b>[%s Alert] %s - %s</b>\n\n", appName, strings.ToUpper(alert.Level), alert.Type)
	message += fmt.Sprintf("Message: %s\n", alert.Message)
	message += fmt.Sprintf("Time: %s", alert.Timestamp.Format(time.RFC1123))

	// Create request URL
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramConfig.BotToken)

	// Create request body
	reqBody := map[string]string{
		"chat_id":    telegramConfig.ChatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram request: %w", err)
	}

	// Send request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("Telegram API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// buildEmailBody creates the email body for an alert
func (am *AlertManager) buildEmailBody(alert types.Alert) string {
	var body strings.Builder
	appName := am.getAppName()

	body.WriteString(fmt.Sprintf("%s MONITORING ALERT\n", strings.ToUpper(appName)))
	body.WriteString("=====================\n\n")
	body.WriteString(fmt.Sprintf("Alert Level: %s\n", strings.ToUpper(alert.Level)))
	body.WriteString(fmt.Sprintf("Alert Type: %s\n", alert.Type))
	body.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))
	body.WriteString(fmt.Sprintf("Timestamp: %s\n", alert.Timestamp.Format(time.RFC1123)))
	body.WriteString(fmt.Sprintf("Server Time: %s\n\n", time.Now().Format(time.RFC1123)))
	body.WriteString(fmt.Sprintf("This alert was generated by the %s monitoring service.\n", appName))

	return body.String()
}

// SendAlerts sends multiple alerts
func (am *AlertManager) SendAlerts(alerts []types.Alert) error {
	var errors []string

	for _, alert := range alerts {
		if err := am.SendAlert(alert); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some alerts failed to send: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ValidateConfig validates the alerting configuration
func (am *AlertManager) ValidateConfig() error {
	if !am.config.Enabled {
		return nil // No validation needed if disabled
	}

	// Validate email configuration if enabled
	if am.config.Email.Enabled {
		if am.config.Email.SMTPHost == "" {
			return fmt.Errorf("SMTP host is required for email alerts")
		}
		if am.config.Email.SMTPPort <= 0 {
			return fmt.Errorf("SMTP port must be positive")
		}
		if am.config.Email.From == "" {
			return fmt.Errorf("from email address is required")
		}
		if am.config.Email.To == "" {
			return fmt.Errorf("to email address is required")
		}
	}

	// Validate Mailgun configuration if enabled
	if am.config.Mailgun.Enabled {
		if am.config.Mailgun.APIKey == "" {
			return fmt.Errorf("API key is required for Mailgun alerts")
		}
		if am.config.Mailgun.Domain == "" {
			return fmt.Errorf("domain is required for Mailgun alerts")
		}
		if am.config.Mailgun.From == "" {
			return fmt.Errorf("from email address is required for Mailgun")
		}
		if am.config.Mailgun.To == "" {
			return fmt.Errorf("to email address is required for Mailgun")
		}
	}

	// Validate Telegram configuration if enabled
	if am.config.Telegram.Enabled {
		if am.config.Telegram.BotToken == "" {
			return fmt.Errorf("bot token is required for Telegram alerts")
		}
		if am.config.Telegram.ChatID == "" {
			return fmt.Errorf("chat ID is required for Telegram alerts")
		}
	}

	// Validate that at least one alerting method is configured if enabled
	if am.config.Enabled && !am.config.Email.Enabled && !am.config.Mailgun.Enabled && !am.config.Telegram.Enabled {
		return fmt.Errorf("alerting is enabled but no alerting methods are configured")
	}

	return nil
}
