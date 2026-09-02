# 📧 Email & Templating Guide

`padi_go_core/email` provides SMTP mail delivery with HTML/plain-text formatting, MIME multipart base64 attachments, template rendering, and background asynchronous dispatching.

---

## 📨 Sending Emails

### 1. HTML Email
```go
err := email.SendHTML("customer@example.com", "Welcome to Padi!", "<h1>Welcome to Padi!</h1>")
```

### 2. Email with Attachments
```go
err := email.SendHTML(
    "accounting@example.com",
    "Monthly Invoice",
    "<p>Please find attached your invoice.</p>",
    "storage/invoices/inv_2026.pdf",
    "storage/receipts/rec_2026.png",
)
```

### 3. Plain Text Email
```go
err := email.SendText("user@example.com", "Password Reset OTP", "Your OTP is: 994821")
```

### 4. Template-Driven Email
Renders standard Go `html/template`:

```go
type WelcomeData struct {
    Name       string
    ConfirmURL string
}

tmpl := `
    <h2>Hi {{ .Name }},</h2>
    <p>Please click below to verify your account:</p>
    <a href="{{ .ConfirmURL }}">Verify Account</a>
`

data := WelcomeData{
    Name:       "Alice",
    ConfirmURL: "https://example.com/verify?token=xyz",
}

err := email.SendTemplate([]string{"alice@example.com"}, "Confirm Your Email", tmpl, data)
```

---

## ⚡ Asynchronous Non-Blocking Dispatch (`SendAsync`)

Sends email in a background goroutine:

```go
msg := email.Message{
    To:      []string{"user@example.com"},
    Subject: "Async Notification",
    Body:    "<p>Processed in background</p>",
    IsHTML:  true,
}

email.SendAsync(msg, func(err error) {
    if err != nil {
        logger.Errorf("Async email delivery failed: %v", err)
    } else {
        logger.Info("Async email delivered successfully")
    }
})
```
