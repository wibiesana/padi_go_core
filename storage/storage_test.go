package storage_test

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wibiesana/padi_go_core/storage"
)

func TestStorageUploadAndDelete(t *testing.T) {
	// 1. Create dummy multipart file header
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test_avatar.png")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte("fake image binary content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	_ = req.ParseMultipartForm(10 << 20)

	fileHeader := req.MultipartForm.File["file"][0]

	// 2. Save Uploaded File
	opts := storage.DefaultUploadOptions()
	opts.SubDir = "test_uploads"
	relPath, err := storage.SaveUploadedFile(fileHeader, opts)
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}
	defer func() {
		_ = storage.Delete(relPath)
		_ = os.RemoveAll("storage/test_uploads")
	}()

	if relPath == "" {
		t.Fatalf("expected non-empty relative path")
	}

	// 3. Test URL generator
	url := storage.URL(req, relPath)
	if url == "" {
		t.Fatalf("expected generated URL")
	}

	// 4. Test dangerous extension rejection
	badBody := new(bytes.Buffer)
	badWriter := multipart.NewWriter(badBody)
	badPart, _ := badWriter.CreateFormFile("file", "script.exe")
	_, _ = badPart.Write([]byte("binary"))
	_ = badWriter.Close()

	badReq := httptest.NewRequest("POST", "/upload", badBody)
	badReq.Header.Set("Content-Type", badWriter.FormDataContentType())
	_ = badReq.ParseMultipartForm(10 << 20)

	badFileHeader := badReq.MultipartForm.File["file"][0]
	_, err = storage.SaveUploadedFile(badFileHeader, opts)
	if err == nil {
		t.Fatalf("expected error on dangerous file extension .exe")
	}

	// 5. Test Delete
	fullPath := filepath.Join("storage", relPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatalf("expected uploaded file to exist on disk before delete")
	}

	if err := storage.Delete(relPath); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("expected uploaded file to be deleted")
	}
}
