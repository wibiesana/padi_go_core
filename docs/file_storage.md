# 📁 File & Media Storage Guide

`padi_go_core/file` and `padi_go_core/storage` provide secure file upload handling, MIME content sniffing, path traversal defense, and local filesystem manipulation.

---

## 🛡️ Uploading Files

```go
func (c *UserController) UploadAvatar(w http.ResponseWriter, r *http.Request) {
    // 1. Single File Upload
    path, err := file.Upload(r, "avatar", file.UploadOptions{
        SubDir:       "avatars",
        AllowedExts:  []string{"jpg", "jpeg", "png", "webp"},
        MaxSizeBytes: 5 * 1024 * 1024, // 5 MB
    })
    if err != nil {
        response.BadRequest(w, err.Error())
        return
    }

    // 2. Resolve Full Public URL (e.g. http://localhost:8080/storage/avatars/20260902_abc.jpg)
    fullURL := file.URL(r, path)

    // 3. Resolve Nullable URL (returns nil if path is empty)
    nullableURL := file.URLOrNull(r, path)

    response.Success(w, map[string]interface{}{
        "path": path,
        "url":  fullURL,
    })
}
```

---

## 🗄️ Filesystem Operations

```go
// Write binary content
err := storage.Put("exports/data.csv", []byte("id,name\n1,Alice"))

// Read binary content
bytes, err := storage.Get("exports/data.csv")

// Copy & Move files
err = storage.Copy("exports/data.csv", "backups/data_bak.csv")
err = storage.Move("backups/data_bak.csv", "archive/data_old.csv")

// Create and Delete Directories
err = storage.MakeDirectory("reports/2026")
err = storage.DeleteDirectory("reports/2026")

// List all files in a directory
files, err := storage.List("uploads")
```
