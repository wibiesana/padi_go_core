package activerecord_test

import (
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/activerecord"
	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/query"
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

	// Test passing columns as slice
	cols := []string{"id", "title"}
	allWithCols, err := activerecord.All[TestArticle](cols...)
	if err != nil || len(allWithCols) != 1 {
		t.Fatalf("failed All with slice columns: %v", err)
	}

	// Test passing columns as variadic strings
	foundWithCols, err := activerecord.FindByPk[TestArticle](art.ID, "id", "title")
	if err != nil || foundWithCols == nil {
		t.Fatalf("failed FindByPk with string columns: %v", err)
	}

	opts := query.Options{Page: 1, PerPage: 10}
	meta, list, err := activerecord.Paginate[TestArticle](opts, "title")
	if err != nil || len(list) != 1 || meta.Total != 1 {
		t.Fatalf("paginate failed")
	}

	// Test Paginate with variadic string search columns
	meta2, list2, err := activerecord.Paginate[TestArticle](opts, "title", "content")
	if err != nil || len(list2) != 1 || meta2.Total != 1 {
		t.Fatalf("paginate with variadic search columns failed")
	}

	// 3. Delete
	err = activerecord.Delete[TestArticle](art.ID)
	if err != nil {
		t.Fatalf("delete failed")
	}
}

type TestUser struct {
	activerecord.ActiveRecord
	ID       uint         `db:"id" json:"id"`
	Username string       `db:"username" json:"username"`
	Email    string       `db:"email" json:"email"`
	Profile  *TestProfile `db:"-" json:"profile"`
	Posts    []TestPost   `db:"-" json:"posts"`
}

func (TestUser) TableName() string {
	return "test_users"
}

func (TestUser) Relations() map[string]activerecord.Relation {
	return map[string]activerecord.Relation{
		"profile": activerecord.HasOne("test_profiles", "user_id", "id"),
		"posts":   activerecord.HasMany("test_posts", "user_id", "id"),
	}
}

type TestProfile struct {
	activerecord.ActiveRecord
	ID     uint   `db:"id" json:"id"`
	UserID uint   `db:"user_id" json:"user_id"`
	Bio    string `db:"bio" json:"bio"`
}

func (TestProfile) TableName() string {
	return "test_profiles"
}

func (TestProfile) Relations() map[string]activerecord.Relation {
	return map[string]activerecord.Relation{
		"user": activerecord.BelongsTo("test_users", "user_id", "id"),
	}
}

type TestPost struct {
	activerecord.ActiveRecord
	ID       uint          `db:"id" json:"id"`
	UserID   uint          `db:"user_id" json:"user_id"`
	Title    string        `db:"title" json:"title"`
	User     *TestUser     `db:"-" json:"user"`
	Comments []TestComment `db:"-" json:"comments"`
	Tags     []TestTag     `db:"-" json:"tags"`
}

func (TestPost) TableName() string {
	return "test_posts"
}

func (TestPost) Relations() map[string]activerecord.Relation {
	return map[string]activerecord.Relation{
		"user":     activerecord.BelongsTo("test_users", "user_id", "id"),
		"comments": activerecord.HasMany("test_comments", "post_id", "id"),
		"tags":     activerecord.BelongsToMany("test_tags", "test_post_tags", "post_id", "tag_id"),
	}
}

type TestComment struct {
	activerecord.ActiveRecord
	ID     uint      `db:"id" json:"id"`
	PostID uint      `db:"post_id" json:"post_id"`
	UserID uint      `db:"user_id" json:"user_id"`
	Body   string    `db:"body" json:"body"`
	Author *TestUser `rel:"author" json:"author"`
}

func (TestComment) TableName() string {
	return "test_comments"
}

func (TestComment) Relations() map[string]activerecord.Relation {
	return map[string]activerecord.Relation{
		"post":   activerecord.BelongsTo("test_posts", "post_id", "id"),
		"author": activerecord.BelongsTo("test_users", "user_id", "id"),
	}
}

type TestTag struct {
	activerecord.ActiveRecord
	ID   uint   `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

func (TestTag) TableName() string {
	return "test_tags"
}

func setupRelationDB(t *testing.T) {
	setupTestDB(t)
	activerecord.RegisterModel(TestUser{})
	activerecord.RegisterModel(TestProfile{})
	activerecord.RegisterModel(TestPost{})
	activerecord.RegisterModel(TestComment{})
	activerecord.RegisterModel(TestTag{})

	db := database.GetDB()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_users;
		DROP TABLE IF EXISTS test_profiles;
		DROP TABLE IF EXISTS test_posts;
		DROP TABLE IF EXISTS test_comments;
		DROP TABLE IF EXISTS test_tags;
		DROP TABLE IF EXISTS test_post_tags;

		CREATE TABLE test_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			email TEXT NOT NULL
		);

		CREATE TABLE test_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			bio TEXT
		);

		CREATE TABLE test_posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL
		);

		CREATE TABLE test_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			body TEXT NOT NULL
		);

		CREATE TABLE test_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		);

		CREATE TABLE test_post_tags (
			post_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL
		);

		INSERT INTO test_users (id, username, email) VALUES (1, 'alice', 'alice@test.com'), (2, 'bob', 'bob@test.com');
		INSERT INTO test_profiles (id, user_id, bio) VALUES (1, 1, 'Alice Bio'), (2, 2, 'Bob Bio');
		INSERT INTO test_posts (id, user_id, title) VALUES (1, 1, 'Alice First Post'), (2, 1, 'Alice Second Post'), (3, 2, 'Bob Post');
		INSERT INTO test_comments (id, post_id, user_id, body) VALUES (1, 1, 2, 'Bob on Alice Post 1'), (2, 1, 1, 'Alice reply'), (3, 3, 1, 'Alice on Bob Post');
		INSERT INTO test_tags (id, name) VALUES (1, 'Go'), (2, 'Framework'), (3, 'PHP');
		INSERT INTO test_post_tags (post_id, tag_id) VALUES (1, 1), (1, 2), (2, 1), (3, 3);
	`)
	if err != nil {
		t.Fatalf("failed to setup relation db tables: %v", err)
	}
}

func TestEagerLoading_BelongsTo_HasMany_HasOne(t *testing.T) {
	setupRelationDB(t)

	// 1. With BelongsTo
	posts, err := activerecord.With[TestPost]("user").OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("failed to query posts with user: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(posts))
	}
	if posts[0].User == nil || posts[0].User.Username != "alice" {
		t.Fatalf("expected post 1 author to be alice, got %+v", posts[0].User)
	}
	if posts[2].User == nil || posts[2].User.Username != "bob" {
		t.Fatalf("expected post 3 author to be bob, got %+v", posts[2].User)
	}

	// 2. With HasMany
	users, err := activerecord.With[TestUser]("posts").OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("failed to query users with posts: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if len(users[0].Posts) != 2 {
		t.Fatalf("expected alice to have 2 posts, got %d", len(users[0].Posts))
	}
	if len(users[1].Posts) != 1 {
		t.Fatalf("expected bob to have 1 post, got %d", len(users[1].Posts))
	}

	// 3. With HasOne
	usersWithProfile, err := activerecord.With[TestUser]("profile").OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("failed to query users with profile: %v", err)
	}
	if usersWithProfile[0].Profile == nil || usersWithProfile[0].Profile.Bio != "Alice Bio" {
		t.Fatalf("expected Alice profile bio, got %+v", usersWithProfile[0].Profile)
	}
}

func TestEagerLoading_BelongsToMany(t *testing.T) {
	setupRelationDB(t)

	posts, err := activerecord.With[TestPost]("tags").OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("failed to query posts with tags: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(posts))
	}
	if len(posts[0].Tags) != 2 {
		t.Fatalf("expected post 1 to have 2 tags, got %d", len(posts[0].Tags))
	}
	if len(posts[2].Tags) != 1 || posts[2].Tags[0].Name != "PHP" {
		t.Fatalf("expected post 3 tag to be PHP, got %+v", posts[2].Tags)
	}
}

func TestEagerLoading_NestedAndColumns(t *testing.T) {
	setupRelationDB(t)

	// Nested: post -> comments -> author
	posts, err := activerecord.With[TestPost]("comments.author", "user:id,username").OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("failed nested eager load: %v", err)
	}
	if len(posts) == 0 {
		t.Fatalf("expected posts")
	}

	// Check post 1 comments
	p1 := posts[0]
	if len(p1.Comments) != 2 {
		t.Fatalf("expected post 1 to have 2 comments, got %d", len(p1.Comments))
	}
	if p1.Comments[0].Author == nil || p1.Comments[0].Author.Username != "bob" {
		t.Fatalf("expected first comment author to be bob, got %+v", p1.Comments[0].Author)
	}

	// Check column filtering on user
	if p1.User == nil || p1.User.Username != "alice" {
		t.Fatalf("expected user to be alice, got %+v", p1.User)
	}
	if p1.User.Email != "" {
		t.Fatalf("expected email to be omitted due to column filter, got %s", p1.User.Email)
	}
}

func TestEagerLoading_MapsAndPagination(t *testing.T) {
	setupRelationDB(t)

	opts := query.Options{Page: 1, PerPage: 2}
	meta, maps, err := activerecord.With[TestPost]("user", "tags").PaginateMaps(opts)
	if err != nil {
		t.Fatalf("failed to paginate maps: %v", err)
	}
	if meta.Total != 3 || len(maps) != 2 {
		t.Fatalf("expected total 3, page 2 items, got total %d, items %d", meta.Total, len(maps))
	}

	// Check that Map has relation data attached
	userVal, hasUser := maps[0].Get("user")
	if !hasUser || userVal == nil {
		t.Fatalf("expected map 0 to have user relation")
	}
	userMap, ok := userVal.(*activerecord.Map)
	if !ok {
		t.Fatalf("expected user relation to be *activerecord.Map")
	}
	username, _ := userMap.Get("username")
	if username != "alice" {
		t.Fatalf("expected username alice, got %v", username)
	}

	// Single First query with eager loading
	singlePost, err := activerecord.With[TestPost]("user", "comments").Find(1)
	if err != nil || singlePost == nil {
		t.Fatalf("failed to find single post with relations: %v", err)
	}
	if singlePost.User == nil || len(singlePost.Comments) != 2 {
		t.Fatalf("expected single post to have user and 2 comments, got user=%+v, comments=%d", singlePost.User, len(singlePost.Comments))
	}
}

type TestBaseDepartment struct {
	activerecord.ActiveRecord
	ID          uint   `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
	SemesterID  uint   `db:"semester_id" json:"semester_id"`
	TeacherID   uint   `db:"teacher_id" json:"teacher_id"`
	Status      int    `db:"status" json:"status"`
}

type TestDepartment struct {
	TestBaseDepartment
}

func (TestDepartment) TableName() string {
	return "test_departments"
}

func TestActiveRecord_EmbeddedStructWithUnexportedFields(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_departments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			semester_id INTEGER,
			teacher_id INTEGER,
			status INTEGER DEFAULT 1
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test_departments table: %v", err)
	}

	dept := TestDepartment{
		TestBaseDepartment: TestBaseDepartment{
			Name:        "Sample name4",
			Description: "Sample description",
			SemesterID:  1,
			TeacherID:   10,
			Status:      1,
		},
	}

	// Test saving a model that embeds ActiveRecord (which has unexported mutex and fields)
	err = activerecord.Save(&dept)
	if err != nil {
		t.Fatalf("expected Save to succeed for embedded struct with unexported fields, got: %v", err)
	}

	if dept.ID == 0 {
		t.Fatalf("expected dept.ID to be populated after insert, got 0")
	}

	// Test find
	found, err := activerecord.Find[TestDepartment](dept.ID)
	if err != nil || found == nil {
		t.Fatalf("failed to find department: %v", err)
	}
	if found.Name != "Sample name4" {
		t.Fatalf("expected department name 'Sample name4', got '%s'", found.Name)
	}
}

