package passwordreset

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host, Username, Password, From, FromName string
	Port                                     int
	TLS                                      bool
}

type SMTPMailer struct{ config SMTPConfig }

func NewSMTPMailer(config SMTPConfig) (*SMTPMailer, error) {
	if config.Host == "" || config.Port <= 0 || config.Username == "" || config.Password == "" || config.From == "" {
		return nil, fmt.Errorf("incomplete Portal SMTP configuration")
	}
	return &SMTPMailer{config: config}, nil
}

func (m *SMTPMailer) SendPasswordResetCode(ctx context.Context, recipient, code string, ttl time.Duration) error {
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;color:#172033"><h2>TogoAPI 密码重置</h2><p>您的验证码是：</p><p style="font-size:32px;font-weight:700;letter-spacing:8px">%s</p><p>验证码将在 %d 分钟后失效。</p><p>如果不是您本人操作，请忽略此邮件。</p></body></html>`, html.EscapeString(code), int(ttl/time.Minute))
	return m.SendHTML(ctx, recipient, "[TogoAPI] 密码重置验证码", body)
}

func (m *SMTPMailer) SendHTML(ctx context.Context, recipient, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || to.Address != strings.TrimSpace(recipient) {
		return fmt.Errorf("invalid recipient email")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("invalid email subject")
	}
	address := net.JoinHostPort(m.config.Host, strconv.Itoa(m.config.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var client *smtp.Client
	if m.config.TLS {
		conn, dialErr := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12})
		if dialErr != nil {
			return dialErr
		}
		client, err = smtp.NewClient(conn, m.config.Host)
	} else {
		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			return dialErr
		}
		client, err = smtp.NewClient(conn, m.config.Host)
	}
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)); err != nil {
		return err
	}
	if err := client.Mail(m.config.From); err != nil {
		return err
	}
	if err := client.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	fromName := strings.TrimSpace(m.config.FromName)
	if fromName == "" {
		fromName = "TogoAPI"
	}
	from := (&mail.Address{Name: fromName, Address: m.config.From}).String()
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", from, to.String(), mime.QEncoding.Encode("UTF-8", subject), body)
	if _, err := w.Write([]byte(message)); err != nil {
		return err
	}
	return w.Close()
}
