package auth_test

import (
	"testing"

	"github.com/wibiesana/padi_go_core/auth"
	"github.com/wibiesana/padi_go_core/config"
)

func TestPasswordHashing(t *testing.T) {
	password := "secret12345"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !auth.CheckPasswordHash(password, hash) {
		t.Errorf("Password hash verification failed")
	}

	if auth.CheckPasswordHash("wrongpassword", hash) {
		t.Errorf("Password hash matched incorrect password")
	}
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	config.AppConfig = &config.Config{
		AppName:       "Padi Test",
		JWTSecret:     "super-secret-testing-key-1234567890",
		JWTExpiration: 1,
	}

	userID := uint(42)
	email := "user@example.com"
	role := "admin"

	token, err := auth.GenerateToken(userID, email, role)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("Expected Email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("Expected Role %s, got %s", role, claims.Role)
	}
}
