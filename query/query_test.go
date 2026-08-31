package query_test

import (
	"testing"
	"time"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/query"
)

type Product struct {
	ID        uint       `db:"id" json:"id"`
	Name      string     `db:"name" json:"name"`
	Price     float64    `db:"price" json:"price"`
	Category  string     `db:"category" json:"category"`
	Stock     int        `db:"stock" json:"stock"`
	CreatedAt *time.Time `db:"created_at" json:"created_at"`
}

func setupQueryDB(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:query_memdb?mode=memory&cache=shared",
	}
	config.AppConfig = cfg

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	_, err = db.Exec(`
		DROP TABLE IF EXISTS products;
		CREATE TABLE products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price REAL NOT NULL,
			category TEXT NOT NULL,
			stock INTEGER DEFAULT 0,
			created_at DATETIME
		);
	`)
	if err != nil {
		t.Fatalf("failed to create products table: %v", err)
	}
}

func TestQueryBuilder(t *testing.T) {
	setupQueryDB(t)

	// 1. Insert
	q := query.New("products")
	id1, err := q.Insert(map[string]interface{}{
		"name":     "Laptop",
		"price":    1500.0,
		"category": "Electronics",
		"stock":    10,
	})
	if err != nil || id1 == 0 {
		t.Fatalf("failed to insert product: %v", err)
	}

	id2, err := query.New("products").Insert(map[string]interface{}{
		"name":     "Phone",
		"price":    800.0,
		"category": "Electronics",
		"stock":    25,
	})
	if err != nil || id2 == 0 {
		t.Fatalf("failed to insert product 2: %v", err)
	}

	// 2. Count, Exists, Sum, Average
	count, err := query.New("products").Count()
	if err != nil || count != 2 {
		t.Fatalf("expected count 2, got %d, err: %v", count, err)
	}

	exists, err := query.New("products").Where("name", "Laptop").Exists()
	if err != nil || !exists {
		t.Fatalf("expected laptop to exist")
	}

	sum, err := query.New("products").Sum("stock")
	if err != nil || sum != 35 {
		t.Fatalf("expected sum 35, got %v", sum)
	}

	avg, err := query.New("products").Avg("price")
	if err != nil || avg != 1150.0 {
		t.Fatalf("expected avg 1150.0, got %v", avg)
	}

	// 3. First / All
	var p Product
	err = query.New("products").Where("id", id1).First(&p)
	if err != nil || p.Name != "Laptop" {
		t.Fatalf("failed to query first product: %v", err)
	}

	var allProducts []Product
	err = query.New("products").OrderBy("price", "DESC").All(&allProducts)
	if err != nil || len(allProducts) != 2 || allProducts[0].Name != "Laptop" {
		t.Fatalf("failed to query all products: %v", err)
	}

	// 4. WhereIn, WhereBetween, FilterWhere, Search
	var filtered []Product
	err = query.New("products").
		WhereIn("category", "Electronics", "Books").
		WhereBetween("price", 500.0, 2000.0).
		FilterWhere("stock", 10).
		All(&filtered)
	if err != nil || len(filtered) != 1 || filtered[0].Name != "Laptop" {
		t.Fatalf("failed filtered query")
	}

	// 5. Update & Delete
	rowsUpdated, err := query.New("products").Where("id", id1).Update(map[string]interface{}{
		"price": 1400.0,
	})
	if err != nil || rowsUpdated != 1 {
		t.Fatalf("failed update query")
	}

	rowsDeleted, err := query.New("products").Where("id", id2).Delete()
	if err != nil || rowsDeleted != 1 {
		t.Fatalf("failed delete query")
	}
}
