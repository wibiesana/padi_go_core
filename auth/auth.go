package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/wibiesana/padi-core/config"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// JWTClaims custom JWT payload claims
type JWTClaims struct {
	UserID   uint                   `json:"user_id"`
	Email    string                 `json:"email"`
	Role     string                 `json:"role,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

// HashPassword hashes plain password with bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares password with hashed string
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates signed JWT string for given user
func GenerateToken(userID uint, email string, role string, meta ...map[string]interface{}) (string, error) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	var metadata map[string]interface{}
	if len(meta) > 0 {
		metadata = meta[0]
	}

	expirationHours := cfg.JWTExpiration
	if expirationHours <= 0 {
		expirationHours = 24
	}

	claims := JWTClaims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		Metadata: metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(expirationHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.AppName,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateToken parses and validates a JWT token string
func ValidateToken(tokenString string) (*JWTClaims, error) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid or expired token")
}
