# 🔍 Query Builder Guide

`padi_go_core/query` provides a 100% parameterized fluent SQL query builder with generic type-safe execution, pagination, and multi-dialect compatibility (PostgreSQL, MySQL, SQLite).

---

## 🚀 Fluent Queries

```go
q := query.New("orders").
    Where("status", "completed").
    WhereBetween("total_amount", 50, 1000).
    WhereIn("currency", "USD", "EUR", "IDR").
    OrderBy("created_at", "DESC")
```

---

## 📦 Type-Safe Generic Execution

### 1. `query.GetAll[T](q)`
Fetches matching rows and maps directly into a slice of type `T`:

```go
orders, err := query.GetAll[Order](q.Limit(20))
for _, order := range orders {
    fmt.Printf("Order #%d: $%.2f\n", order.ID, order.TotalAmount)
}
```

### 2. `query.GetFirst[T](q)`
Fetches a single row as a pointer `*T`:

```go
order, err := query.GetFirst[Order](query.New("orders").Where("id", 42))
if err == nil && order != nil {
    fmt.Println(order.CustomerName)
}
```

---

## 📊 Pagination

```go
opts := query.ParseOptions(r) // extracts page, per_page, sort, order, search

var products []Product
meta, err := query.New("products").
    Where("is_active", 1).
    Paginate(opts, []string{"name", "sku", "description"}, &products)

// Send standardized JSON response
response.Paginated(w, products, meta)
```

---

## 📈 Aggregations

```go
q := query.New("products").Where("category", "Books")

count, _ := q.Count()
exists, _ := q.Exists()
totalStock, _ := q.Sum("stock")
avgPrice, _ := q.Avg("price")
minPrice, _ := q.Min("price")
maxPrice, _ := q.Max("price")
```

---

## 🛠️ Insert, Update, Delete

```go
// Insert (returns new ID, handles PostgreSQL RETURNING id automatically)
newID, err := query.New("products").Insert(map[string]interface{}{
    "name":  "Smartphone",
    "price": 699.99,
})

// Update
affected, err := query.New("products").Where("id", newID).Update(map[string]interface{}{
    "price": 649.99,
})

// Delete
deleted, err := query.New("products").Where("id", newID).Delete()
```
