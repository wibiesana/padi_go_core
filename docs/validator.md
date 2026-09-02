# ✅ Validation & Request Binding Guide

`padi_go_core/validator` provides tag-based struct validation, one-line generic request binding (`validator.Bind[T]`), and a procedural `FormValidator` builder.

---

## 🎯 Generic One-Line Binding (`validator.Bind[T]`)

Decodes JSON request body and validates all tags in a single call:

```go
type RegisterRequest struct {
    Username string `json:"username" validate:"required,min=3,max=30"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Age      int    `json:"age" validate:"gte=18"`
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
    req, errs, err := validator.Bind[RegisterRequest](r)
    if err != nil {
        // Automatically returns 422 Unprocessable Entity with error map
        response.UnprocessableEntity(w, errs)
        return
    }

    // req is *RegisterRequest, fully validated
    fmt.Println(req.Username, req.Email)
}
```

---

## 📝 Procedural Form Validator (`validator.New`)

For dynamic map or form-data validation:

```go
func (c *PostController) Store(w http.ResponseWriter, r *http.Request) {
    v := validator.New(r)
    v.Required("title", "content", "category_id")
    v.Min("title", 5)
    v.Max("title", 200)

    if v.Fails() {
        response.UnprocessableEntity(w, v.Errors())
        return
    }

    // Process valid input...
}
```

---

## 🏷️ Supported Struct Validation Tags

| Tag | Example | Description |
| :--- | :--- | :--- |
| `required` | `validate:"required"` | Field must not be empty / nil |
| `email` | `validate:"email"` | Must be a valid email format |
| `min` | `validate:"min=5"` | Minimum string length / numeric value |
| `max` | `validate:"max=100"` | Maximum string length / numeric value |
| `gte` / `lte` | `validate:"gte=18"` | Greater/less than or equal to |
| `alphanum` | `validate:"alphanum"` | Letters and numbers only |
| `numeric` | `validate:"numeric"` | Numeric digits only |
| `eqfield` | `validate:"eqfield=Password"` | Must match another field (e.g., confirmation) |
