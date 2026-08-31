package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	AppName        string
	AppPort        string
	AppDebug       bool
	AppKey         string
	JWTSecret      string
	JWTExpiration  int // in hours
	DBConnection   string
	DBHost         string
	DBPort         string
	DBDatabase     string
	DBUsername     string
	DBPassword     string
	CorsOrigins    []string
	RateLimitReqs  int
	RateLimitWindow int // in seconds
}

var AppConfig *Config

// Load loads the .env file and populates AppConfig
func Load(envPath ...string) *Config {
	targetEnv := ".env"
	if len(envPath) > 0 && envPath[0] != "" {
		targetEnv = envPath[0]
	}

	// Try to find .env starting from current working directory or provided path
	_ = godotenv.Load(targetEnv)

	AppConfig = &Config{
		AppEnv:          GetEnv("APP_ENV", "development"),
		AppName:         GetEnv("APP_NAME", "Padi REST API"),
		AppPort:         GetEnv("APP_PORT", "8080"),
		AppDebug:        GetEnvBool("APP_DEBUG", true),
		AppKey:          GetEnv("APP_KEY", "padi-secret-key-change-in-prod"),
		JWTSecret:       GetEnv("JWT_SECRET", "padi-jwt-secret-key-change-me"),
		JWTExpiration:   GetEnvInt("JWT_EXPIRATION", 24),
		DBConnection:    GetEnv("DB_CONNECTION", "sqlite"),
		DBHost:          GetEnv("DB_HOST", "127.0.0.1"),
		DBPort:          GetEnv("DB_PORT", "3306"),
		DBDatabase:      GetEnv("DB_DATABASE", "database/database.sqlite"),
		DBUsername:      GetEnv("DB_USERNAME", "root"),
		DBPassword:      GetEnv("DB_PASSWORD", ""),
		CorsOrigins:     GetEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
		RateLimitReqs:   GetEnvInt("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow: GetEnvInt("RATE_LIMIT_WINDOW", 60),
	}

	return AppConfig
}

func GetEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func GetEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	valLower := strings.ToLower(val)
	return valLower == "true" || valLower == "1" || valLower == "yes"
}

func GetEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func GetEnvSlice(key string, defaultVal []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}

func EnsureDirectory(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
