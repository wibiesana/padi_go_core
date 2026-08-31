package config_test

import (
	"os"
	"testing"

	"github.com/wibiesana/padi_go_core/config"
)

func TestConfigLoadAndHelpers(t *testing.T) {
	os.Setenv("APP_NAME", "Padi Test Suite")
	os.Setenv("APP_DEBUG", "true")
	os.Setenv("JWT_EXPIRATION", "48")
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, http://example.com")

	cfg := config.Load()
	if cfg == nil {
		t.Fatalf("expected non-nil config")
	}

	if cfg.AppName != "Padi Test Suite" {
		t.Fatalf("expected 'Padi Test Suite', got '%s'", cfg.AppName)
	}

	if !cfg.AppDebug {
		t.Fatalf("expected AppDebug to be true")
	}

	if cfg.JWTExpiration != 48 {
		t.Fatalf("expected JWTExpiration 48, got %d", cfg.JWTExpiration)
	}

	if len(cfg.CorsOrigins) != 2 || cfg.CorsOrigins[0] != "http://localhost:3000" {
		t.Fatalf("expected parsed cors origins")
	}

	// Test default fallbacks
	strVal := config.GetEnv("NON_EXISTING_KEY", "fallback")
	if strVal != "fallback" {
		t.Fatalf("expected fallback, got '%s'", strVal)
	}

	boolVal := config.GetEnvBool("NON_EXISTING_BOOL", true)
	if !boolVal {
		t.Fatalf("expected fallback true")
	}

	intVal := config.GetEnvInt("NON_EXISTING_INT", 100)
	if intVal != 100 {
		t.Fatalf("expected fallback 100")
	}
}
