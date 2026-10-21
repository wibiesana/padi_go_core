package config_test

import (
	"os"
	"testing"
	"time"

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

	// Test float and duration
	os.Setenv("FLOAT_VAL", "3.14")
	if config.GetEnvFloat("FLOAT_VAL", 1.0) != 3.14 {
		t.Fatalf("expected float 3.14")
	}

	os.Setenv("DUR_VAL", "5m")
	if config.GetEnvDuration("DUR_VAL", 0) != 5*time.Minute {
		t.Fatalf("expected duration 5m")
	}

	// Test aliases and env modes
	if config.Env("APP_NAME", "") != "Padi Test Suite" {
		t.Fatalf("Env alias failed")
	}
	if config.Get("APP_NAME", "") != "Padi Test Suite" {
		t.Fatalf("Get alias failed")
	}

	_ = config.SetEnv("APP_ENV", "development")
	config.Load()
	if !config.IsDevelopment() {
		t.Fatalf("expected IsDevelopment to be true")
	}
	if config.IsProduction() {
		t.Fatalf("expected IsProduction to be false")
	}
}
