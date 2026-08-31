package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/generator"
)

func TestGeneratorUtilsAndScaffolding(t *testing.T) {
	// 1. Test Naming Helpers
	if name := generator.TableNameToModelName("user_profiles"); name != "UserProfile" {
		t.Fatalf("expected 'UserProfile', got '%s'", name)
	}
	if name := generator.TableNameToModelName("articles"); name != "Article" {
		t.Fatalf("expected 'Article', got '%s'", name)
	}
	if col := generator.ColumnToFieldName("created_at"); col != "CreatedAt" {
		t.Fatalf("expected 'CreatedAt', got '%s'", col)
	}
	if col := generator.ColumnToFieldName("author_id"); col != "AuthorID" {
		t.Fatalf("expected 'AuthorID', got '%s'", col)
	}

	// 2. Test SQL type to Go type
	if goType := generator.MapSQLTypeToGoType("VARCHAR(255)", false); goType != "string" {
		t.Fatalf("expected 'string', got '%s'", goType)
	}
	if goType := generator.MapSQLTypeToGoType("DATETIME", true); goType != "*time.Time" {
		t.Fatalf("expected '*time.Time', got '%s'", goType)
	}
	if goType := generator.MapSQLTypeToGoType("BIGINT UNSIGNED", false); goType != "uint64" {
		t.Fatalf("expected 'uint64', got '%s'", goType)
	}

	// 3. Test In-memory CRUD Generation
	tmpDir := t.TempDir()
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:gen_test_mem?mode=memory&cache=shared",
	}
	config.AppConfig = cfg

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author_id INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	gen := generator.New(tmpDir)
	if gen.IsProtectedTable("users") != true {
		t.Fatalf("expected users to be protected table")
	}

	err = gen.GenerateCRUD("posts")
	if err != nil {
		t.Fatalf("GenerateCRUD failed: %v", err)
	}

	// Verify generated files in tmpDir
	baseModelPath := filepath.Join(tmpDir, "app", "Models", "Base", "Post.go")
	if _, err := os.Stat(baseModelPath); os.IsNotExist(err) {
		t.Fatalf("expected Base Model file to be generated at %s", baseModelPath)
	}

	customModelPath := filepath.Join(tmpDir, "app", "Models", "Post.go")
	if _, err := os.Stat(customModelPath); os.IsNotExist(err) {
		t.Fatalf("expected Custom Model file to be generated at %s", customModelPath)
	}

	baseCtrlPath := filepath.Join(tmpDir, "app", "Controllers", "Base", "PostController.go")
	if _, err := os.Stat(baseCtrlPath); os.IsNotExist(err) {
		t.Fatalf("expected Base Controller file to be generated at %s", baseCtrlPath)
	}

	customCtrlPath := filepath.Join(tmpDir, "app", "Controllers", "PostController.go")
	if _, err := os.Stat(customCtrlPath); os.IsNotExist(err) {
		t.Fatalf("expected Custom Controller file to be generated at %s", customCtrlPath)
	}

	collectionPath := filepath.Join(tmpDir, "api_collection", "post_api_collection.json")
	if _, err := os.Stat(collectionPath); os.IsNotExist(err) {
		t.Fatalf("expected API Collection file to be generated at %s", collectionPath)
	}
}
