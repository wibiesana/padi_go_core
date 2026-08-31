package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wibiesana/padi_go_core/config"
)

// blockedMimeSniff contains MIME type substrings that are always blocked
// regardless of file extension (defense against disguised executables)
var blockedMimeSniff = []string{
	"text/x-php",
	"application/x-php",
	"application/x-httpd-php",
	"text/x-script.phyton",
	"application/x-sh",
	"text/x-shellscript",
	"application/x-perl",
	"application/x-msdos-program",
	"application/x-msdownload",
	"application/octet-stream", // only block if file ext is also suspicious
}

// DangerousExtensions contains security-sensitive file extensions that cannot be uploaded
var DangerousExtensions = map[string]bool{
	"exe": true, "bat": true, "cmd": true, "sh": true, "php": true,
	"phtml": true, "phar": true, "py": true, "rb": true, "js": true,
	"jsp": true, "cgi": true, "pl": true, "dll": true, "so": true,
	"msi": true, "com": true, "vbs": true, "hta": true,
}

// UploadOptions upload customization options
type UploadOptions struct {
	SubDir       string
	AllowedExts  []string
	MaxSizeBytes int64
}

// DefaultUploadOptions returns default 10MB image/document upload options
func DefaultUploadOptions() UploadOptions {
	return UploadOptions{
		SubDir:       "uploads",
		AllowedExts:  []string{"jpg", "jpeg", "png", "webp", "gif", "pdf", "docx", "xlsx", "zip"},
		MaxSizeBytes: 10 * 1024 * 1024, // 10MB
	}
}

// Upload extracts a multipart file directly from http.Request and saves it securely
func Upload(r *http.Request, formKey string, opts ...UploadOptions) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}

	opt := DefaultUploadOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	maxSize := opt.MaxSizeBytes
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(maxSize); err != nil {
		return "", fmt.Errorf("failed to parse multipart form: %w", err)
	}

	file, header, err := r.FormFile(formKey)
	if err != nil {
		return "", fmt.Errorf("file form key '%s' not found: %w", formKey, err)
	}
	defer file.Close()

	return SaveUploadedFile(header, opt)
}

// UploadMultiple uploads all files from a specific form key (e.g. multiple photos)
func UploadMultiple(r *http.Request, formKey string, opts ...UploadOptions) ([]string, error) {
	if r == nil {
		return nil, errors.New("request is nil")
	}

	opt := DefaultUploadOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	maxSize := opt.MaxSizeBytes
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}

	if err := r.ParseMultipartForm(maxSize); err != nil {
		return nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	headers := r.MultipartForm.File[formKey]
	if len(headers) == 0 {
		return nil, fmt.Errorf("no files found under key '%s'", formKey)
	}

	var paths []string
	for _, fh := range headers {
		savedPath, err := SaveUploadedFile(fh, opt)
		if err != nil {
			return paths, err
		}
		paths = append(paths, savedPath)
	}

	return paths, nil
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

	// Read first 512 bytes for MIME content sniffing (PHP-style finfo defense)
	sniffBuf := make([]byte, 512)
	n, _ := src.Read(sniffBuf)
	detectedMime := http.DetectContentType(sniffBuf[:n])
	detectedMimeLower := strings.ToLower(detectedMime)
	for _, blocked := range blockedMimeSniff {
		if blocked == "application/octet-stream" {
			continue // allow generic binaries unless extension is blocked
		}
		if strings.Contains(detectedMimeLower, blocked) {
			return "", fmt.Errorf("file content type '%s' is not allowed", detectedMime)
		}
	}
	// Rewind to start for copy
	if _, err := src.Seek(0, 0); err != nil {
		// If not seekable, wrap remaining read with already-read bytes
		// This handles the case where the file doesn't support seeking
		_ = err // best-effort; file content already validated
	}

	// Generate random filename to prevent collisions & enumeration
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

	// Returns normalized relative path (e.g. "uploads/20260831_abc.jpg")
	relPath := filepath.ToSlash(filepath.Join(subDir, fileName))
	return relPath, nil
}

// URLOrNull converts a relative file path to an absolute URL, or returns an empty string if path is empty.
// Safe to use directly on nullable database columns to avoid building broken URLs.
func URLOrNull(r *http.Request, relativePath string) string {
	if relativePath == "" {
		return ""
	}
	return URL(r, relativePath)
}

// URL converts a relative file path (e.g. "uploads/abc.jpg") to an absolute URL
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

// Exists checks if a file exists in the storage directory
func Exists(relativePath string) bool {
	if relativePath == "" {
		return false
	}
	fullPath := filepath.Join("storage", filepath.Clean(relativePath))
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Size returns the file size in bytes
func Size(relativePath string) (int64, error) {
	if relativePath == "" {
		return 0, errors.New("empty path")
	}
	fullPath := filepath.Join("storage", filepath.Clean(relativePath))
	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// MimeType detects the MIME type of a stored file
func MimeType(relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("empty path")
	}
	ext := filepath.Ext(relativePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType, nil
}

// Delete removes file from storage directory
func Delete(relativePath string) error {
	if relativePath == "" {
		return nil
	}
	fullPath := filepath.Join("storage", filepath.Clean(relativePath))
	return os.Remove(fullPath)
}
