package notify

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	EmailSecurityNone     = "none"
	EmailSecuritySTARTTLS = "starttls"
	EmailSecurityTLS      = "tls"
)

type EmailSettings struct {
	Enabled  bool
	Host     string
	Port     int
	Security string
	Username string
	Password string
	From     string
	To       string
}

func NormalizeEmailSettings(settings EmailSettings) EmailSettings {
	settings.Host = strings.TrimSpace(settings.Host)
	settings.Security = NormalizeEmailSecurity(settings.Security)
	settings.Username = strings.TrimSpace(settings.Username)
	settings.Password = strings.TrimSpace(settings.Password)
	settings.From = strings.TrimSpace(settings.From)
	settings.To = strings.TrimSpace(settings.To)
	if settings.Port == 0 {
		settings.Port = 587
	}
	return settings
}

func NormalizeEmailSecurity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EmailSecurityNone:
		return EmailSecurityNone
	case EmailSecurityTLS, "ssl", "ssl_tls", "smtps":
		return EmailSecurityTLS
	default:
		return EmailSecuritySTARTTLS
	}
}

func EmailChannelReady(settings EmailSettings) bool {
	settings = NormalizeEmailSettings(settings)
	return settings.Enabled && settings.Host != "" && settings.Port > 0 && settings.From != "" && settings.To != ""
}

func ValidateEmailSettings(settings EmailSettings, required bool) string {
	settings = NormalizeEmailSettings(settings)
	hasAny := settings.Host != "" ||
		settings.From != "" ||
		settings.To != "" ||
		settings.Username != "" ||
		settings.Password != ""
	if !required && !hasAny {
		return ""
	}
	if settings.Host == "" {
		return "email smtp host is required"
	}
	if len(settings.Host) > 255 || strings.ContainsAny(settings.Host, " \t\r\n") {
		return "invalid email smtp host"
	}
	if settings.Port < 1 || settings.Port > 65535 {
		return "email smtp port must be between 1 and 65535"
	}
	if settings.From == "" {
		return "email from address is required"
	}
	if _, err := mail.ParseAddress(settings.From); err != nil {
		return "invalid email from address"
	}
	if settings.To == "" {
		return "email recipient is required"
	}
	if _, err := ParseEmailRecipients(settings.To); err != nil {
		return "invalid email recipient: " + err.Error()
	}
	if settings.Username == "" && settings.Password != "" {
		return "email smtp username is required"
	}
	if settings.Username != "" && settings.Password == "" {
		return "email smtp password is required"
	}
	if len(settings.Username) > 256 {
		return "email smtp username is too long"
	}
	if len(settings.Password) > 512 {
		return "email smtp password is too long"
	}
	if hasControlChars(settings.Host, false) ||
		hasControlChars(settings.Username, false) ||
		hasControlChars(settings.Password, false) ||
		hasControlChars(settings.From, false) ||
		hasControlChars(settings.To, true) {
		return "email settings contain invalid characters"
	}
	return ""
}

func ParseEmailRecipients(value string) ([]*mail.Address, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	recipients := make([]*mail.Address, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		addr, err := mail.ParseAddress(part)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, addr)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("empty recipient list")
	}
	return recipients, nil
}

func SendEmail(settings EmailSettings, subject string, content string) error {
	settings = NormalizeEmailSettings(settings)
	if message := ValidateEmailSettings(settings, true); message != "" {
		return fmt.Errorf("%s", message)
	}

	from, err := mail.ParseAddress(settings.From)
	if err != nil {
		return err
	}
	recipients, err := ParseEmailRecipients(settings.To)
	if err != nil {
		return err
	}

	client, err := smtpClient(settings)
	if err != nil {
		return err
	}
	defer client.Close()

	if settings.Username != "" {
		auth := smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient.Address); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(emailMessage(from, recipients, subject, content)); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func smtpClient(settings EmailSettings) (*smtp.Client, error) {
	addr := net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port))
	dialer := net.Dialer{Timeout: 8 * time.Second}
	tlsConfig := &tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12}

	if settings.Security == EmailSecurityTLS {
		conn, err := tls.DialWithDialer(&dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, settings.Host)
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if settings.Security == EmailSecuritySTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, fmt.Errorf("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

func emailMessage(from *mail.Address, recipients []*mail.Address, subject string, content string) []byte {
	to := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		to = append(to, recipient.String())
	}
	lines := []string{
		"From: " + from.String(),
		"To: " + strings.Join(to, ", "),
		"Subject: " + mime.QEncoding.Encode("utf-8", strings.TrimSpace(subject)),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		content,
	}
	return []byte(strings.Join(lines, "\r\n"))
}

func hasControlChars(value string, allowListSeparators bool) bool {
	for _, r := range value {
		if r >= 32 {
			continue
		}
		if allowListSeparators && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		if r < 32 {
			return true
		}
	}
	return false
}
