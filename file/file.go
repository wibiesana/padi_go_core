package file

import (
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wibiesana/padi_go_core/storage"
)

type UploadOptions = storage.UploadOptions

func DefaultUploadOptions() UploadOptions {
	return storage.DefaultUploadOptions()
}

// Upload uploads a single file from http.Request (matching PHP File::upload)
func Upload(r *http.Request, formKey string, opts ...UploadOptions) (string, error) {
	return storage.Upload(r, formKey, opts...)
}

// UploadMultiple uploads multiple files
func UploadMultiple(r *http.Request, formKey string, opts ...UploadOptions) ([]string, error) {
	return storage.UploadMultiple(r, formKey, opts...)
}

// SaveUploadedFile saves a multipart.FileHeader
func SaveUploadedFile(fh *multipart.FileHeader, opts UploadOptions) (string, error) {
	return storage.SaveUploadedFile(fh, opts)
}

// URL generates absolute URL for stored relative path
func URL(r *http.Request, relativePath string) string {
	return storage.URL(r, relativePath)
}

// URLOrNull generates absolute URL for stored relative path, or empty string if path is empty.
// Safe to use on nullable database columns.
func URLOrNull(r *http.Request, relativePath string) string {
	return storage.URLOrNull(r, relativePath)
}

// Exists checks if file exists
func Exists(relativePath string) bool {
	return storage.Exists(relativePath)
}

// Size returns file size
func Size(relativePath string) (int64, error) {
	return storage.Size(relativePath)
}

// MimeType detects file MIME type
func MimeType(relativePath string) (string, error) {
	return storage.MimeType(relativePath)
}

// Delete removes file from storage
func Delete(relativePath string) error {
	return storage.Delete(relativePath)
}

// Path returns the local file system path for a relative storage path
func Path(relativePath string) string {
	return filepath.Join("storage", filepath.Clean(relativePath))
}

// Put writes binary content to a storage path
func Put(relativePath string, content []byte) error {
	fullPath := Path(relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

// PutString writes a text string to a storage path
func PutString(relativePath string, content string) error {
	return Put(relativePath, []byte(content))
}

// Get reads binary content from a storage path
func Get(relativePath string) ([]byte, error) {
	fullPath := Path(relativePath)
	return os.ReadFile(fullPath)
}

// GetString reads text content from a storage path
func GetString(relativePath string) (string, error) {
	bytes, err := Get(relativePath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// SanitizeFileName strips path traversal characters and invalid symbols
func SanitizeFileName(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "")
	base = strings.ReplaceAll(base, "/", "")
	base = strings.ReplaceAll(base, "\\", "")
	return strings.TrimSpace(base)
}
