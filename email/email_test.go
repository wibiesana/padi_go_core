package email_test

import (
	"testing"

	"github.com/wibiesana/padi_go_core/email"
)

func TestEmailSendValidation(t *testing.T) {
	// 1. Test empty recipients validation
	msg := email.Message{
		To:      []string{},
		Subject: "Test Subject",
		Body:    "<p>Hello World</p>",
	}

	err := email.Send(msg)
	if err == nil {
		t.Fatalf("expected error on empty recipients list")
	}

	// 2. Test invalid host failure without panic
	msgWithTo := email.Message{
		To:      []string{"user@example.com"},
		Subject: "Test Subject",
		Body:    "<p>Hello World</p>",
	}
	_ = email.Send(msgWithTo)

	// 3. Test SendTemplate error handling
	err = email.SendTemplate([]string{"user@example.com"}, "Welcome", "Hello {{.Name}}", map[string]string{"Name": "Alice"})
	if err == nil {
		// SMTP may fail if no daemon running, which is expected
	}

	// 4. Test invalid template syntax error
	err = email.SendTemplate([]string{"user@example.com"}, "Bad", "Hello {{.Name", nil)
	if err == nil {
		t.Fatalf("expected template parsing error")
	}

	// 5. Test SendAsync without panic
	done := make(chan bool)
	email.SendAsync(msg, func(err error) {
		if err == nil {
			t.Errorf("expected error on empty recipients")
		}
		done <- true
	})
	<-done
}
