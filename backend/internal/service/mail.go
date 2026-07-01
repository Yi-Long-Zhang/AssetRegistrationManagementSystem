package service

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"

	"asset-registration-management-system/backend/internal/model"
)

type MailMessage struct {
	To      []mail.Address
	Subject string
	Body    string
}

type MailSender interface {
	Send(config model.MailConfig, password string, message MailMessage) error
}

type SMTPMailSender struct{}

func (SMTPMailSender) Send(config model.MailConfig, password string, message MailMessage) error {
	if len(message.To) == 0 {
		return nil
	}
	host := strings.TrimSpace(config.SMTPHost)
	if host == "" {
		return fmt.Errorf("SMTP host is empty")
	}
	port := config.SMTPPort
	if port == 0 {
		port = 25
	}
	address := net.JoinHostPort(host, fmt.Sprint(port))
	from := mail.Address{Name: config.FromName, Address: config.FromAddress}
	headers := map[string]string{
		"From":         from.String(),
		"To":           joinAddresses(message.To),
		"Subject":      message.Subject,
		"MIME-Version": "1.0",
		"Content-Type": `text/plain; charset="UTF-8"`,
	}
	raw := buildMail(headers, message.Body)

	var auth smtp.Auth
	if strings.TrimSpace(config.Username) != "" {
		auth = smtp.PlainAuth("", config.Username, password, host)
	}
	recipients := make([]string, 0, len(message.To))
	for _, item := range message.To {
		recipients = append(recipients, item.Address)
	}
	if config.UseTLS {
		return sendSMTPOverTLS(address, host, auth, config.FromAddress, recipients, raw)
	}
	return sendSMTP(address, host, auth, config.StartTLS, config.FromAddress, recipients, raw)
}

func sendSMTP(address, host string, auth smtp.Auth, startTLS bool, from string, to []string, raw []byte) error {
	client, err := smtp.Dial(address)
	if err != nil {
		return err
	}
	defer client.Close()
	if startTLS {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	return finishSMTP(client, auth, from, to, raw)
}

func sendSMTPOverTLS(address, host string, auth smtp.Auth, from string, to []string, raw []byte) error {
	conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()
	return finishSMTP(client, auth, from, to, raw)
}

func finishSMTP(client *smtp.Client, auth smtp.Auth, from string, to []string, raw []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMail(headers map[string]string, body string) []byte {
	var builder strings.Builder
	for key, value := range headers {
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(value)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")
	builder.WriteString(body)
	builder.WriteString("\r\n")
	return []byte(builder.String())
}

func joinAddresses(values []mail.Address) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, value.String())
	}
	return strings.Join(items, ", ")
}
