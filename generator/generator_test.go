package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/generator"
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
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author_id INTEGER REFERENCES users(id),
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

	// 4. Test CRUD Generation with Realtime enabled
	genRealtime := generator.New(tmpDir).WithRealtime(true)
	err = genRealtime.GenerateCRUD("posts")
	if err != nil {
		t.Fatalf("GenerateCRUD with realtime failed: %v", err)
	}

	baseCtrlBytes, err := os.ReadFile(baseCtrlPath)
	if err != nil {
		t.Fatalf("failed reading generated Base Controller: %v", err)
	}
	baseCtrlStr := string(baseCtrlBytes)
	if !strings.Contains(baseCtrlStr, "realtime.Publish") {
		t.Fatalf("expected Base Controller to include realtime.Publish calls")
	}

	baseModelBytes, err := os.ReadFile(baseModelPath)
	if err != nil {
		t.Fatalf("failed reading generated Base Model: %v", err)
	}
	baseModelStr := string(baseModelBytes)
	if !strings.Contains(baseModelStr, `WithColumns("id", "username")`) {
		t.Fatalf("expected Base Model to specify WithColumns for relation, got:\n%s", baseModelStr)
	}

	if !strings.Contains(baseCtrlStr, `withRelations := []string{"author:id,username"}`) {
		t.Fatalf("expected Base Controller to specify withRelations eager loading, got:\n%s", baseCtrlStr)
	}

	baseResPath := filepath.Join(tmpDir, "app", "Resources", "Base", "PostResource.go")
	baseResBytes, err := os.ReadFile(baseResPath)
	if err != nil {
		t.Fatalf("failed reading generated Base Resource: %v", err)
	}
	baseResStr := string(baseResBytes)
	if strings.Contains(baseResStr, "author_name") {
		t.Fatalf("expected Base Resource to NOT contain flattened author_name, got:\n%s", baseResStr)
	}

	// 5. Test Multiple Foreign Keys pointing to the same table (e.g. id and approved_by referencing users)
	_, err = db.Exec(`
		CREATE TABLE creator_profiles (
			id INTEGER PRIMARY KEY REFERENCES users(id),
			approved_by INTEGER REFERENCES users(id),
			pen_name TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create creator_profiles table: %v", err)
	}

	if err := gen.GenerateCRUD("creator_profiles"); err != nil {
		t.Fatalf("GenerateCRUD for creator_profiles failed: %v", err)
	}

	creatorModelPath := filepath.Join(tmpDir, "app", "Models", "Base", "CreatorProfile.go")
	creatorBytes, err := os.ReadFile(creatorModelPath)
	if err != nil {
		t.Fatalf("failed to read generated CreatorProfile base model: %v", err)
	}
	creatorStr := string(creatorBytes)
	if !strings.Contains(creatorStr, `"approved_by": activerecord.BelongsTo("users", "approved_by"`) {
		t.Fatalf("expected CreatorProfile to have approved_by relation, got:\n%s", creatorStr)
	}
	if !strings.Contains(creatorStr, `"user": activerecord.BelongsTo("users", "id"`) {
		t.Fatalf("expected CreatorProfile to have user relation for id column, got:\n%s", creatorStr)
	}
	if strings.Count(creatorStr, `"users":`) > 0 {
		t.Fatalf("expected CreatorProfile to not contain duplicate 'users' keys, got:\n%s", creatorStr)
	}
}

