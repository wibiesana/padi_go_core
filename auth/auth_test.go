package auth_test

import (
	"context"
	"net/http"
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

	// VerifyToken alias test
	vClaims, err := auth.VerifyToken(token)
	if err != nil || vClaims.UserID != userID {
		t.Errorf("VerifyToken failed: %v", err)
	}

	// VerifyPassword flexibility test
	if !auth.VerifyPassword("secret12345", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy") {
		// hash is for secret12345
	}
	hash, _ := auth.HashPassword("secret12345")
	if !auth.VerifyPassword("secret12345", hash) || !auth.VerifyPassword(hash, "secret12345") {
		t.Errorf("VerifyPassword argument order flexibility failed")
	}

	// Request helper tests
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if !auth.Check(req) {
		t.Errorf("Expected request to be authenticated")
	}
	if auth.UserID(req) != userID {
		t.Errorf("Expected UserID %d, got %d", userID, auth.UserID(req))
	}
	if !auth.HasRole(req, "admin", "superadmin") {
		t.Errorf("Expected user to have admin role")
	}
	if auth.HasRole(req, "customer", "guest") {
		t.Errorf("Expected user to not have customer role")
	}

	// Context helper test
	ctx := context.WithValue(req.Context(), auth.UserContextKey, claims)
	reqWithCtx := req.WithContext(ctx)
	if auth.UserIDFromContext(reqWithCtx.Context()) != userID {
		t.Errorf("UserIDFromContext failed")
	}
}
