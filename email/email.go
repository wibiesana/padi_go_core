package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/wibiesana/padi_go_core/config"
)

type Message struct {
	To          []string
	Subject     string
	Body        string
	ContentType string // text/plain or text/html (default: text/html)
}

// Send sends email via SMTP
func Send(msg Message) error {
	smtpHost := config.GetEnv("MAIL_HOST", "smtp.mailtrap.io")
	smtpPort := config.GetEnv("MAIL_PORT", "2525")
	smtpUser := config.GetEnv("MAIL_USERNAME", "")
	smtpPass := config.GetEnv("MAIL_PASSWORD", "")
	fromAddr := config.GetEnv("MAIL_FROM_ADDRESS", "no-reply@padi-api.com")
	fromName := config.GetEnv("MAIL_FROM_NAME", "Padi REST API")

	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	contentType := "text/html; charset=UTF-8"
	if msg.ContentType != "" {
		contentType = msg.ContentType
	}

	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", fromName, fromAddr)
	header["To"] = strings.Join(msg.To, ", ")
	header["Subject"] = msg.Subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = contentType

	var messageStr strings.Builder
	for k, v := range header {
		messageStr.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	messageStr.WriteString("\r\n")
	messageStr.WriteString(msg.Body)

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	// Direct Send if port 465 (SSL)
	if smtpPort == "465" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         smtpHost,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, smtpHost)
		if err != nil {
			return err
		}
		defer client.Close()

		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(fromAddr); err != nil {
			return err
		}
		for _, to := range msg.To {
			if err = client.Rcpt(to); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(messageStr.String()))
		if err != nil {
			return err
		}
		return w.Close()
	}

	// Standard STARTTLS or plain SMTP
	return smtp.SendMail(addr, auth, fromAddr, msg.To, []byte(messageStr.String()))
}
