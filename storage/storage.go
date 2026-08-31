package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wibiesana/padi_go_core/config"
)

var DangerousExtensions = map[string]bool{
	"exe": true, "bat": true, "cmd": true, "sh": true, "php": true,
	"phtml": true, "phar": true, "py": true, "rb": true, "js": true,
	"jsp": true, "cgi": true, "pl": true, "dll": true, "so": true,
}

// UploadOptions upload customization options
type UploadOptions struct {
	SubDir       string
	AllowedExts  []string
	MaxSizeBytes int64
}

// DefaultUploadOptions returns default 5MB image/document options
func DefaultUploadOptions() UploadOptions {
	return UploadOptions{
		SubDir:       "uploads",
		AllowedExts:  []string{"jpg", "jpeg", "png", "webp", "gif", "pdf", "docx", "xlsx", "zip"},
		MaxSizeBytes: 10 * 1024 * 1024, // 10MB
	}
}

// SaveUploadedFile validates and saves a multipart file securely
func SaveUploadedFile(fh *multipart.FileHeader, opts UploadOptions) (string, error) {
	if fh == nil {
		return "", errors.New("no file provided")
	}

	if opts.MaxSizeBytes <= 0 {
		opts.MaxSizeBytes = 10 * 1024 * 1024
	}
	if fh.Size > opts.MaxSizeBytes {
		return "", fmt.Errorf("file size (%d bytes) exceeds allowed limit (%d bytes)", fh.Size, opts.MaxSizeBytes)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	if DangerousExtensions[ext] {
		return "", fmt.Errorf("file extension .%s is not allowed for security reasons", ext)
	}

	if len(opts.AllowedExts) > 0 {
		allowed := false
		for _, a := range opts.AllowedExts {
			if strings.ToLower(a) == ext {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("file type .%s is not permitted. Allowed: %s", ext, strings.Join(opts.AllowedExts, ", "))
		}
	}

	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Generate random filename to prevent collisions & directory traversal
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	timestamp := time.Now().Format("20060102")
	fileName := fmt.Sprintf("%s_%s.%s", timestamp, randomHex, ext)

	subDir := opts.SubDir
	if subDir == "" {
		subDir = "uploads"
	}

	targetDir := filepath.Join("storage", subDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	destPath := filepath.Join(targetDir, fileName)
	dest, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return "", err
	}

	// Returns normalized relative path
	relPath := filepath.ToSlash(filepath.Join(subDir, fileName))
	return relPath, nil
}

// URL converts a relative file path (e.g. "uploads/2026_abc.jpg") to an absolute URL
func URL(r *http.Request, relativePath string) string {
	if relativePath == "" {
		return ""
	}
	if strings.HasPrefix(relativePath, "http://") || strings.HasPrefix(relativePath, "https://") {
		return relativePath
	}

	cfg := config.AppConfig
	scheme := "http"
	if r != nil && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}

	host := "localhost:8080"
	if r != nil && r.Host != "" {
		host = r.Host
	} else if cfg != nil && cfg.AppPort != "" {
		host = "localhost:" + cfg.AppPort
	}

	return fmt.Sprintf("%s://%s/storage/%s", scheme, host, strings.TrimPrefix(relativePath, "/"))
}

// Delete removes file from storage directory
func Delete(relativePath string) error {
	if relativePath == "" {
		return nil
	}
	fullPath := filepath.Join("storage", filepath.Clean(relativePath))
	return os.Remove(fullPath)
}
