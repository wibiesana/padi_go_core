package email

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"github.com/wibiesana/padi_go_core/config"
)

type Message struct {
	To          []string
	Cc          []string
	Bcc         []string
	ReplyTo     string
	Subject     string
	Body        string
	ContentType string // text/plain or text/html (default: text/html)
	Attachments []string
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

	var allRecipients []string
	allRecipients = append(allRecipients, msg.To...)
	allRecipients = append(allRecipients, msg.Cc...)
	allRecipients = append(allRecipients, msg.Bcc...)

	var messageStr strings.Builder
	messageStr.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromAddr))
	messageStr.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	if len(msg.Cc) > 0 {
		messageStr.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.Cc, ", ")))
	}
	if msg.ReplyTo != "" {
		messageStr.WriteString(fmt.Sprintf("Reply-To: %s\r\n", msg.ReplyTo))
	}
	messageStr.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	messageStr.WriteString("MIME-Version: 1.0\r\n")

	if len(msg.Attachments) > 0 {
		boundary := "----=_Part_Padi_" + fmt.Sprintf("%d", os.Getpid())
		messageStr.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))

		// Body part
		messageStr.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		messageStr.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
		messageStr.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
		messageStr.WriteString(msg.Body)
		messageStr.WriteString("\r\n\r\n")

		// Attachment parts
		for _, attachPath := range msg.Attachments {
			fileBytes, err := os.ReadFile(attachPath)
			if err != nil {
				continue
			}
			fileName := filepath.Base(attachPath)
			mimeType := http.DetectContentType(fileBytes)

			messageStr.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			messageStr.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, fileName))
			messageStr.WriteString("Content-Transfer-Encoding: base64\r\n")
			messageStr.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", fileName))

			b64 := base64.StdEncoding.EncodeToString(fileBytes)
			for i := 0; i < len(b64); i += 76 {
				end := i + 76
				if end > len(b64) {
					end = len(b64)
				}
				messageStr.WriteString(b64[i:end])
				messageStr.WriteString("\r\n")
			}
			messageStr.WriteString("\r\n")
		}
		messageStr.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		messageStr.WriteString(fmt.Sprintf("Content-Type: %s\r\n\r\n", contentType))
		messageStr.WriteString(msg.Body)
	}

	var auth smtp.Auth
	if smtpUser != "" || smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}
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

		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return err
			}
		}
		if err = client.Mail(fromAddr); err != nil {
			return err
		}
		for _, to := range allRecipients {
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
	return smtp.SendMail(addr, auth, fromAddr, allRecipients, []byte(messageStr.String()))
}

// SendHTML sends an HTML email to a single or multiple recipients
func SendHTML(to string, subject, htmlBody string, attachments ...string) error {
	return Send(Message{
		To:          []string{to},
		Subject:     subject,
		Body:        htmlBody,
		ContentType: "text/html; charset=UTF-8",
		Attachments: attachments,
	})
}

// SendText sends a plain text email
func SendText(to string, subject, textBody string) error {
	return Send(Message{
		To:          []string{to},
		Subject:     subject,
		Body:        textBody,
		ContentType: "text/plain; charset=UTF-8",
	})
}

// SendTo sends a message to multiple recipients
func SendTo(to []string, subject, htmlBody string) error {
	return Send(Message{
		To:          to,
		Subject:     subject,
		Body:        htmlBody,
		ContentType: "text/html; charset=UTF-8",
	})
}

// SendTemplate renders an HTML template string with data and sends the email
func SendTemplate(to []string, subject string, templateStr string, data interface{}) error {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	return Send(Message{
		To:          to,
		Subject:     subject,
		Body:        buf.String(),
		ContentType: "text/html; charset=UTF-8",
	})
}

// SendAsync sends an email asynchronously in a goroutine
func SendAsync(msg Message, onComplete ...func(err error)) {
	go func() {
		err := Send(msg)
		if len(onComplete) > 0 && onComplete[0] != nil {
			onComplete[0](err)
		}
	}()
}
