# 🌾 ActiveRecord & Eager Loading Guide

`padi_go_core/activerecord` provides a high-performance, reflection-cached ActiveRecord implementation with **out-of-the-box eager loading**, nested relations, column filtering, and full generic type-safe querying.

---

## 🚀 Model Definition & Relations

Declare your model struct embedding `activerecord.ActiveRecord`:

```go
package models

import (
	"time"
	"github.com/wibiesana/padi_go_core/activerecord"
)

type User struct {
	activerecord.ActiveRecord
	ID        uint      `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (User) TableName() string {
	return "users"
}

type Comment struct {
	activerecord.ActiveRecord
	ID        uint      `db:"id" json:"id"`
	PostID    uint      `db:"post_id" json:"post_id"`
	AuthorID  uint      `db:"author_id" json:"author_id"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (Comment) TableName() string {
	return "comments"
}

// BelongsTo relation to User
func (c Comment) Author() *activerecord.Relation {
	return c.BelongsTo(User{}, "author_id", "id")
}

type Post struct {
	activerecord.ActiveRecord
	ID        uint      `db:"id" json:"id"`
	UserID    uint      `db:"user_id" json:"user_id"`
	Title     string    `db:"title" json:"title"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (Post) TableName() string {
	return "posts"
}

// BelongsTo relation to User
func (p Post) Author() *activerecord.Relation {
	return p.BelongsTo(User{}, "user_id", "id")
}

// HasMany relation to Comments
func (p Post) Comments() *activerecord.Relation {
	return p.HasMany(Comment{}, "post_id", "id")
}
```

---

## ⚡ Eager Loading Queries

### 1. Simple Eager Loading with `.With()`
Load posts with their author in 2 optimized queries (no N+1 problem):

```go
posts, err := activerecord.NewModelQuery[Post]().
    With("author").
    Where("status", "published").
    OrderBy("created_at", "DESC").
    Get()

for _, post := range posts {
    if author, ok := post.GetRelation("author").(*User); ok && author != nil {
        fmt.Printf("Post '%s' written by %s\n", post.Title, author.Name)
    }
}
```

### 2. Nested Recursive Relations (`comments.author`)
Load posts, their comments, AND the authors of those comments:

```go
posts, err := activerecord.NewModelQuery[Post]().
    With("comments.author").
    Get()
```

### 3. Column Filtering (`author:id,name`)
Optimize query bandwidth by selecting only necessary columns:

```go
posts, err := activerecord.NewModelQuery[Post]().
    With("author:id,name").
    Get()
```

### 4. Paginated Eager Loading
```go
opts := query.ParseOptions(r)
posts, meta, err := activerecord.NewModelQuery[Post]().
    With("author", "comments").
    Paginate(opts, []string{"title", "content"})
```

### 5. Returning Maps for JSON APIs
```go
// Returns []map[string]interface{} with relation keys embedded directly
dataMaps, meta, err := activerecord.NewModelQuery[Post]().
    With("author", "comments.author").
    PaginateMaps(opts, []string{"title"})
```

---

## 💾 Basic CRUD Operations

### Create
```go
post := Post{
    UserID:  1,
    Title:   "New Post",
    Content: "Content body",
}
err := post.Save(r.Context()) // Automatically populates created_at / created_by
```

### Find by Primary Key
```go
post, err := (Post{}).Find(42)
if err != nil {
    // Record not found or database error
}
```

### Update
```go
post.Title = "Updated Title"
err := post.Save(r.Context()) // Automatically updates updated_at / updated_by
```

### Delete
```go
err := post.Delete()
```
