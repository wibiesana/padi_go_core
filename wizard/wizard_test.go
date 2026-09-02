package wizard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wibiesana/padi_go_core/wizard"
)

func TestWizardInstantiation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "padi_wizard_test")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	w := wizard.New(tempDir)
	if w == nil {
		t.Fatalf("expected wizard instance")
	}
}
