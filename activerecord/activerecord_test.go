package activerecord_test

import (
	"testing"
	"time"

	"github.com/wibiesana/padi-core/activerecord"
	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/query"
)

type TestArticle struct {
	ID        uint       `db:"id" json:"id"`
	Title     string     `db:"title" json:"title"`
	Content   string     `db:"content" json:"content"`
	Status    string     `db:"status" json:"status"`
	CreatedAt *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
}

func (TestArticle) TableName() string {
	return "test_articles"
}

func setupTestDB(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:ar_memdb?mode=memory&cache=shared",
	}
	config.AppConfig = cfg

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	_, err = db.Exec(`
		DROP TABLE IF EXISTS test_articles;
		CREATE TABLE test_articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT,
			status TEXT DEFAULT 'draft',
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
}

func TestActiveRecord(t *testing.T) {
	setupTestDB(t)

	// 1. Save
	art := TestArticle{
		Title:   "PHP Matching ActiveRecord",
		Content: "Content Here",
		Status:  "published",
	}
	err := activerecord.Save(&art)
	if err != nil {
		t.Fatalf("failed to save article: %v", err)
	}
	if art.ID == 0 {
		t.Fatalf("expected ID > 0")
	}

	// 2. Find / FindOne / FindAll / Paginate
	found, err := activerecord.Find[TestArticle](art.ID)
	if err != nil || found == nil {
		t.Fatalf("failed to find: %v", err)
	}

	opts := query.Options{Page: 1, PerPage: 10}
	meta, list, err := activerecord.Paginate[TestArticle](opts, "title")
	if err != nil || len(list) != 1 || meta.Total != 1 {
		t.Fatalf("paginate failed")
	}

	// 3. Delete
	err = activerecord.Delete[TestArticle](art.ID)
	if err != nil {
		t.Fatalf("delete failed")
	}
}
