package file_test

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/wibiesana/padi_go_core/file"
)

func TestFilePackageWrappers(t *testing.T) {
	// 1. Test URLOrNull
	req := httptest.NewRequest("GET", "http://localhost:8080/api", nil)
	if file.URLOrNull(req, "") != "" {
		t.Errorf("Expected URLOrNull with empty path to return empty string")
	}

	res := file.URLOrNull(req, "uploads/avatar.jpg")
	if res != "http://localhost:8080/storage/uploads/avatar.jpg" {
		t.Errorf("Unexpected URL result: %s", res)
	}

	// 2. Test File Upload via wrapper
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", "profile.png")
	if err != nil {
		t.Fatalf("failed to create part: %v", err)
	}
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01"))
	_ = writer.Close()

	uploadReq := httptest.NewRequest("POST", "/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())

	relPath, err := file.Upload(uploadReq, "photo")
	if err != nil {
		t.Fatalf("file.Upload failed: %v", err)
	}
	defer func() {
		_ = file.Delete(relPath)
		_ = os.RemoveAll("storage/uploads")
	}()

	if !file.Exists(relPath) {
		t.Errorf("Expected uploaded file to exist via file.Exists")
	}
}
