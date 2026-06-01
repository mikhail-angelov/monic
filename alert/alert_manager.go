package alert

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"bconf.com/monic/types"
)

// Manager handles sending alerts via configured channels
type Manager struct {
	config     *types.AlertingConfig
	appName    string
	lastSent   map[string]time.Time
	httpClient *http.Client
}

// NewManager creates a new alert manager instance
func NewManager(config *types.AlertingConfig, appName string) *Manager {
	return &Manager{
		config:     config,
		appName:    appName,
		lastSent:   make(map[string]time.Time),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendAlert sends an alert through all configured channels
func (am *Manager) SendAlert(alert types.Alert) error {
	if !am.shouldSendCooldown(alert) {
		return nil
	}

	appName := am.getAppName()
	subject := fmt.Sprintf("[%s Alert] %s - %s", appName, strings.ToUpper(alert.Level), alert.Type)
	body := am.buildEmailBody(alert)

	var errs []string

	if am.config.Email.Enabled {
		if err := am.sendEmailMessage(subject, body); err != nil {
			slog.Error("Failed to send email alert", "error", err)
			errs = append(errs, fmt.Sprintf("email: %v", err))
		}
	}

	if am.config.Mailgun.Enabled {
		if err := am.sendMailgunMessage(subject, body); err != nil {
			slog.Error("Failed to send Mailgun alert", "error", err)
			errs = append(errs, fmt.Sprintf("mailgun: %v", err))
		}
	}

	if am.config.Telegram.Enabled {
		if err := am.sendTelegramMessage(am.buildTelegramAlert(alert)); err != nil {
			slog.Error("Failed to send Telegram alert", "error", err)
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	am.lastSent[alert.Type] = time.Now()

	if len(errs) > 0 {
		return fmt.Errorf("failed to send alerts: %s", strings.Join(errs, "; "))
	}
	slog.Info("Alert sent", "level", alert.Level, "message", alert.Message)
	return nil
}

// SendAlerts sends multiple alerts
func (am *Manager) SendAlerts(alerts []types.Alert) error {
	var errs []string
	for _, alert := range alerts {
		if err := am.SendAlert(alert); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("some alerts failed to send: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SendDigest sends a daily digest report through all configured channels.
func (am *Manager) SendDigest(digestText string) error {
	appName := am.getAppName()
	subject := fmt.Sprintf("%s Daily Digest — %s", appName, time.Now().Format("2006-01-02"))

	var errs []string

	if am.config.Email.Enabled {
		if err := am.sendEmailMessage(subject, digestText); err != nil {
			slog.Error("Failed to send digest email", "error", err)
			errs = append(errs, fmt.Sprintf("email: %v", err))
		}
	}

	if am.config.Mailgun.Enabled {
		if err := am.sendMailgunMessage(subject, digestText); err != nil {
			slog.Error("Failed to send digest via Mailgun", "error", err)
			errs = append(errs, fmt.Sprintf("mailgun: %v", err))
		}
	}

	if am.config.Telegram.Enabled {
		if err := am.sendTelegramDigest(appName, digestText); err != nil {
			slog.Error("Failed to send digest via Telegram", "error", err)
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send digest: %s", strings.Join(errs, "; "))
	}
	slog.Info("Daily digest sent successfully")
	return nil
}

// sendEmailMessage sends an email with the given subject and body through the configured SMTP server.
func (am *Manager) sendEmailMessage(subject, body string) error {
	cfg := am.config.Email
	message := "From: " + cfg.From + "\r\n" +
		"To: " + cfg.To + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
		"\r\n" + body

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	if cfg.UseTLS {
		return am.sendEmailTLS(addr, auth, cfg, message)
	}
	if err := smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, []byte(message)); err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}
	slog.Info("Email sent", "recipient", cfg.To, "subject", subject)
	return nil
}

// sendEmailTLS sends an email using STARTTLS (port 587 style).
func (am *Manager) sendEmailTLS(addr string, auth smtp.Auth, cfg types.EmailConfig, message string) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer client.Close()

	if err = client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}
	if err = client.Mail(cfg.From); err != nil {
		return fmt.Errorf("SMTP MAIL failed: %w", err)
	}
	if err = client.Rcpt(cfg.To); err != nil {
		return fmt.Errorf("SMTP RCPT failed: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	defer w.Close()
	if _, err = w.Write([]byte(message)); err != nil {
		return fmt.Errorf("SMTP message write failed: %w", err)
	}
	slog.Info("Email sent (TLS)", "recipient", cfg.To, "subject", cfg.From)
	return nil
}

// sendMailgunMessage sends a message via Mailgun with the given subject and body.
func (am *Manager) sendMailgunMessage(subject, body string) error {
	cfg := am.config.Mailgun
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.mailgun.net/v3"
	}

	form := url.Values{}
	form.Set("from", cfg.From)
	form.Set("to", cfg.To)
	form.Set("subject", subject)
	form.Set("text", body)

	reqURL := fmt.Sprintf("%s/%s/messages", baseURL, cfg.Domain)
	req, err := http.NewRequest("POST", reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create Mailgun request: %w", err)
	}
	req.SetBasicAuth("api", cfg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailgun API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mailgun API returned status %s: %s", resp.Status, string(b))
	}
	slog.Info("Mailgun message sent", "recipient", cfg.To, "subject", subject)
	return nil
}

// sendTelegramMessage sends a single pre-formatted Telegram message (HTML parse mode).
func (am *Manager) sendTelegramMessage(text string) error {
	cfg := am.config.Telegram
	reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	reqBody := map[string]string{
		"chat_id":    cfg.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram request: %w", err)
	}
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}

// buildTelegramAlert formats a single alert as an HTML Telegram message.
func (am *Manager) buildTelegramAlert(alert types.Alert) string {
	appName := am.getAppName()
	icon := "❌"
	if alert.Level == "info" {
		icon = "✅"
	}
	return fmt.Sprintf("%s<b>[%s] %s - %s</b>: %s\n%s",
		icon, appName, strings.ToUpper(alert.Level), alert.Type,
		alert.Message, alert.Timestamp.Format(time.RFC1123))
}

// sendTelegramDigest sends a digest, splitting into multiple messages if it exceeds Telegram's 4096-char limit.
func (am *Manager) sendTelegramDigest(appName, digestText string) error {
	const maxLen = 4000
	full := fmt.Sprintf("<b>%s Daily Digest</b>\n%s\n\n<pre>%s</pre>",
		appName, time.Now().Format("2006-01-02"), escapeTelegramHTML(digestText))

	if len([]rune(full)) <= maxLen {
		return am.sendTelegramMessage(full)
	}

	// Split by lines, counting runes to stay within the limit
	lines := strings.Split(digestText, "\n")
	var part string
	for _, line := range lines {
		if len([]rune(part))+len([]rune(line))+1 > maxLen-100 {
			if err := am.sendTelegramMessage("<pre>" + escapeTelegramHTML(part) + "</pre>"); err != nil {
				return err
			}
			part = line
		} else {
			if part != "" {
				part += "\n"
			}
			part += line
		}
	}
	if part != "" {
		return am.sendTelegramMessage("<pre>" + escapeTelegramHTML(part) + "</pre>")
	}
	return nil
}

func (am *Manager) shouldSendCooldown(alert types.Alert) bool {
	lastSent, exists := am.lastSent[alert.Type]
	if !exists {
		return true
	}
	return time.Since(lastSent) >= time.Minute
}

func (am *Manager) getAppName() string {
	if am.appName != "" {
		return am.appName
	}
	return "Monic"
}

func (am *Manager) buildEmailBody(alert types.Alert) string {
	var b strings.Builder
	appName := am.getAppName()
	b.WriteString(fmt.Sprintf("%s MONITORING ALERT\n", strings.ToUpper(appName)))
	b.WriteString("=====================\n\n")
	b.WriteString(fmt.Sprintf("Alert Level: %s\n", strings.ToUpper(alert.Level)))
	b.WriteString(fmt.Sprintf("Alert Type: %s\n", alert.Type))
	b.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))
	b.WriteString(fmt.Sprintf("Timestamp: %s\n", alert.Timestamp.Format(time.RFC1123)))
	b.WriteString(fmt.Sprintf("Server Time: %s\n\n", time.Now().Format(time.RFC1123)))
	b.WriteString(fmt.Sprintf("This alert was generated by the %s monitoring service.\n", appName))
	return b.String()
}

// ValidateConfig validates the alerting configuration
func (am *Manager) ValidateConfig() error {
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

	if am.config.Telegram.Enabled {
		if am.config.Telegram.BotToken == "" {
			return fmt.Errorf("bot token is required for Telegram alerts")
		}
		if am.config.Telegram.ChatID == "" {
			return fmt.Errorf("chat ID is required for Telegram alerts")
		}
		if !isValidTelegramBotToken(am.config.Telegram.BotToken) {
			return fmt.Errorf("invalid Telegram bot token format")
		}
	}

	return nil
}

// isValidTelegramBotToken performs basic validation of Telegram bot token format.
// Expected format: digits:alphanumeric (e.g. 1234567890:ABCdef...)
func isValidTelegramBotToken(token string) bool {
	if len(token) < 20 {
		return false
	}
	parts := strings.Split(token, ":")
	if len(parts) != 2 {
		return false
	}
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	for _, ch := range parts[1] {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '_' && ch != '-' {
			return false
		}
	}
	return true
}

func escapeTelegramHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
