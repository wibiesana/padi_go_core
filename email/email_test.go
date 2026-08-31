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
	err = email.Send(msgWithTo)
	if err == nil {
		// If by chance a local smtp is open, that's fine, otherwise err is expected
	}
}
