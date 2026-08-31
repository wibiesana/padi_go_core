package model_test

import (
	"testing"
	"time"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/model"
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
		DBDatabase:   "file:model_memdb?mode=memory&cache=shared",
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

func TestActiveRecordCRUD(t *testing.T) {
	setupTestDB(t)

	// 1. Create / Save
	art := TestArticle{
		Title:   "First Article",
		Content: "Hello World Content",
		Status:  "published",
	}
	err := model.Save(&art)
	if err != nil {
		t.Fatalf("failed to save article: %v", err)
	}
	if art.ID == 0 {
		t.Fatalf("expected article ID > 0, got %d", art.ID)
	}

	// 2. Find / FindByPk / FindOrFail
	found, err := model.Find[TestArticle](art.ID)
	if err != nil || found == nil {
		t.Fatalf("failed to find article: %v", err)
	}
	if found.Title != "First Article" {
		t.Fatalf("expected title 'First Article', got '%s'", found.Title)
	}

	foundPk, err := model.FindByPk[TestArticle](art.ID)
	if err != nil || foundPk == nil {
		t.Fatalf("failed to find by PK: %v", err)
	}

	foundFail, err := model.FindOrFail[TestArticle](art.ID)
	if err != nil || foundFail == nil {
		t.Fatalf("failed to FindOrFail: %v", err)
	}

	// 3. FindOne / FindAll / All
	foundOne, err := model.FindOne[TestArticle](map[string]interface{}{"status": "published"})
	if err != nil || foundOne == nil {
		t.Fatalf("failed to FindOne by map: %v", err)
	}

	all, err := model.All[TestArticle]()
	if err != nil || len(all) != 1 {
		t.Fatalf("expected 1 article, got %d, err: %v", len(all), err)
	}

	// 4. Update / UpdateRecord
	err = model.Update[TestArticle](art.ID, map[string]interface{}{"title": "Updated Title"})
	if err != nil {
		t.Fatalf("failed to update article: %v", err)
	}

	foundUpdated, _ := model.Find[TestArticle](art.ID)
	if foundUpdated.Title != "Updated Title" {
		t.Fatalf("expected updated title 'Updated Title', got '%s'", foundUpdated.Title)
	}

	// 5. Count & Paginate
	cnt, err := model.Count[TestArticle]()
	if err != nil || cnt != 1 {
		t.Fatalf("expected count 1, got %d, err: %v", cnt, err)
	}

	opts := query.Options{Page: 1, PerPage: 10}
	meta, list, err := model.Paginate[TestArticle](opts, "title", "content")
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 item in paginated list, got %d, err: %v", len(list), err)
	}
	if meta.Total != 1 {
		t.Fatalf("expected meta total 1, got %d", meta.Total)
	}
	// 6. Delete
	err = model.Delete[TestArticle](art.ID)
	if err != nil {
		t.Fatalf("failed to delete article: %v", err)
	}

	afterDelete, _ := model.Find[TestArticle](art.ID)
	if afterDelete != nil {
		t.Fatalf("expected article to be deleted, but still found")
	}
}

func TestAllActiveRecordMethods(t *testing.T) {
	setupTestDB(t)

	// 1. Metadata and connection tests
	tbl := model.GetTable[TestArticle]()
	if tbl != "test_articles" {
		t.Fatalf("expected test_articles, got %s", tbl)
	}

	db := model.GetDb()
	if db == nil {
		t.Fatalf("expected non-nil db")
	}

	likeOp := model.GetLikeOperator()
	if likeOp != "LIKE" {
		t.Fatalf("expected LIKE for sqlite, got %s", likeOp)
	}

	pk := model.GetPrimaryKeyName(TestArticle{})
	if pk != "id" {
		t.Fatalf("expected id pk, got %s", pk)
	}

	conn := model.GetConnectionName(TestArticle{})
	if conn != "" {
		t.Fatalf("expected empty default connection name, got %s", conn)
	}

	// 2. Column Cache & GetTableColumns
	cols, err := model.GetTableColumns("test_articles")
	if err != nil || len(cols) == 0 {
		t.Fatalf("failed to get table columns: %v", err)
	}
	model.ClearColumnsCache()

	// 3. BatchInsert
	items := []TestArticle{
		{Title: "Batch 1", Content: "Content 1", Status: "active"},
		{Title: "Batch 2", Content: "Content 2", Status: "active"},
		{Title: "Batch 3", Content: "Content 3", Status: "draft"},
	}
	err = model.BatchInsert(items)
	if err != nil {
		t.Fatalf("batch insert failed: %v", err)
	}

	countAll, err := model.Count[TestArticle]()
	if err != nil || countAll != 3 {
		t.Fatalf("expected count 3 after batch insert, got %d", countAll)
	}

	// 4. FindAll with slice of IDs
	allArticles, _ := model.All[TestArticle]()
	if len(allArticles) < 2 {
		t.Fatalf("expected at least 2 articles, got %d", len(allArticles))
	}
	records, err := model.FindAll[TestArticle]([]uint{allArticles[0].ID, allArticles[1].ID})
	if err != nil || len(records) != 2 {
		t.Fatalf("expected 2 records from FindAll with slice, got %d, err: %v", len(records), err)
	}

	// 5. Where & FilterWhere
	q := model.Where[TestArticle]("status", "active")
	var activeRecords []TestArticle
	err = q.All(&activeRecords)
	if err != nil || len(activeRecords) != 2 {
		t.Fatalf("expected 2 active records, got %d", len(activeRecords))
	}

	fq := model.FilterWhere[TestArticle](map[string]interface{}{
		"status": "active",
		"title":  "", // should be skipped
	})
	var fRecords []TestArticle
	err = fq.All(&fRecords)
	if err != nil || len(fRecords) != 2 {
		t.Fatalf("expected 2 records with FilterWhere, got %d", len(fRecords))
	}

	// 6. Search
	sq := model.Search[TestArticle]("Batch 1")
	var searchRecords []TestArticle
	err = sq.All(&searchRecords)
	if err != nil || len(searchRecords) != 1 {
		t.Fatalf("expected 1 search result, got %d, err: %v", len(searchRecords), err)
	}

	// 7. UpdateAll
	affected, err := model.UpdateAll[TestArticle](map[string]interface{}{"status": "archived"}, map[string]interface{}{"status": "active"})
	if err != nil || affected != 2 {
		t.Fatalf("expected 2 updated rows in UpdateAll, got %d, err: %v", affected, err)
	}

	// 8. DeleteAll with condition
	deleted, err := model.DeleteAll[TestArticle](map[string]interface{}{"status": "archived"})
	if err != nil || deleted != 2 {
		t.Fatalf("expected 2 deleted rows in DeleteAll, got %d, err: %v", deleted, err)
	}

	// 9. SanitizeOrderBy
	sanitized, err := model.SanitizeOrderBy("id DESC, title ASC")
	if err != nil || sanitized != "id DESC, title ASC" {
		t.Fatalf("unexpected SanitizeOrderBy result: %s, err: %v", sanitized, err)
	}
	_, err = model.SanitizeOrderBy("id; DROP TABLE users;")
	if err == nil {
		t.Fatalf("expected error on sql injection in SanitizeOrderBy")
	}

	// 10. Relationships helper check
	r1 := model.HasOne("comments", "article_id")
	if r1.Type != model.RelHasOne {
		t.Fatalf("unexpected rel type")
	}
	r2 := model.HasMany("comments", "article_id")
	if r2.Type != model.RelHasMany {
		t.Fatalf("unexpected rel type")
	}
	r3 := model.BelongsTo("users", "user_id")
	if r3.Type != model.RelBelongsTo {
		t.Fatalf("unexpected rel type")
	}
	r4 := model.BelongsToMany("tags", "article_tags", "article_id", "tag_id")
	if r4.Type != model.RelBelongsToMany {
		t.Fatalf("unexpected rel type")
	}
}
