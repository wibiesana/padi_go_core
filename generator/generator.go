package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wibiesana/padi-core/database"
)

type ColumnInfo struct {
	Name         string
	DataType     string
	GoType       string
	IsNullable   bool
	IsPrimaryKey bool
	JSONName     string
	ValidateTag  string
}

type Generator struct {
	db              *sql.DB
	baseDir         string
	protectedTables []string
}

func New(baseDir ...string) *Generator {
	dir := "."
	if len(baseDir) > 0 && baseDir[0] != "" {
		dir = baseDir[0]
	}
	return &Generator{
		db:              database.GetDB(),
		baseDir:         dir,
		protectedTables: []string{"password_resets", "migrations", "jobs"},
	}
}

// IsProtectedTable checks if table is a protected core table
func (g *Generator) IsProtectedTable(tableName string) bool {
	t := strings.ToLower(tableName)
	for _, pt := range g.protectedTables {
		if t == pt {
			return true
		}
	}
	return false
}

// TableNameToModelName converts "user_profiles" or "users" to "UserProfile" / "User"
func TableNameToModelName(tableName string) string {
	parts := strings.Split(tableName, "_")
	var res strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		res.WriteString(strings.ToUpper(part[:1]))
		res.WriteString(strings.ToLower(part[1:]))
	}
	name := res.String()
	if strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") && len(name) > 3 {
		name = strings.TrimSuffix(name, "s")
	}
	return name
}

// ColumnToFieldName converts "created_at" -> "CreatedAt", "id" -> "ID"
func ColumnToFieldName(col string) string {
	if strings.ToLower(col) == "id" {
		return "ID"
	}
	if strings.HasSuffix(strings.ToLower(col), "_id") {
		prefix := col[:len(col)-3]
		return ColumnToFieldName(prefix) + "ID"
	}

	parts := strings.Split(col, "_")
	var res strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		if upper == "ID" || upper == "URL" || upper == "API" || upper == "IP" || upper == "JSON" || upper == "UUID" {
			res.WriteString(upper)
		} else {
			res.WriteString(strings.ToUpper(part[:1]))
			res.WriteString(part[1:])
		}
	}
	return res.String()
}

// MapSQLTypeToGoType maps SQL data type to idiomatic Go type
func MapSQLTypeToGoType(sqlType string, isNullable bool) string {
	t := strings.ToLower(sqlType)

	switch {
	case strings.Contains(t, "int"):
		if strings.Contains(t, "bigint") {
			if strings.Contains(t, "unsigned") {
				return "uint64"
			}
			return "int64"
		}
		if strings.Contains(t, "unsigned") {
			return "uint"
		}
		return "uint"
	case strings.Contains(t, "bool"):
		return "bool"
	case strings.Contains(t, "float"), strings.Contains(t, "double"), strings.Contains(t, "decimal"), strings.Contains(t, "numeric"):
		return "float64"
	case strings.Contains(t, "time"), strings.Contains(t, "date"):
		if isNullable {
			return "*time.Time"
		}
		return "time.Time"
	case strings.Contains(t, "json"), strings.Contains(t, "jsonb"):
		return "string"
	default:
		if isNullable && !strings.Contains(t, "text") && !strings.Contains(t, "char") {
			return "*string"
		}
		return "string"
	}
}

// GetTableColumns inspects DB table schema
func (g *Generator) GetTableColumns(tableName string) ([]ColumnInfo, error) {
	var columns []ColumnInfo
	driver := database.GetDriver()

	if driver == "sqlite" {
		rows, err := g.db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var cid, notnull, pk int
			var name, typeStr string
			var dflt *string
			if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dflt, &pk); err == nil {
				isPK := pk > 0
				isNullable := notnull == 0 && !isPK
				goType := MapSQLTypeToGoType(typeStr, isNullable)

				validateTag := ""
				if !isNullable && !isPK && !strings.Contains(name, "_at") {
					validateTag = "validate:\"required\""
				}

				columns = append(columns, ColumnInfo{
					Name:         name,
					DataType:     typeStr,
					GoType:       goType,
					IsNullable:   isNullable,
					IsPrimaryKey: isPK,
					JSONName:     name,
					ValidateTag:  validateTag,
				})
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else if driver == "mysql" {
		rows, err := g.db.Query(fmt.Sprintf("DESCRIBE `%s`", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var field, typeStr, null, key, extra string
			var def *string
			if err := rows.Scan(&field, &typeStr, &null, &key, &def, &extra); err == nil {
				isPK := key == "PRI"
				isNullable := null == "YES"
				goType := MapSQLTypeToGoType(typeStr, isNullable)

				validateTag := ""
				if !isNullable && !isPK && !strings.Contains(field, "_at") {
					validateTag = "validate:\"required\""
				}

				columns = append(columns, ColumnInfo{
					Name:         field,
					DataType:     typeStr,
					GoType:       goType,
					IsNullable:   isNullable,
					IsPrimaryKey: isPK,
					JSONName:     field,
					ValidateTag:  validateTag,
				})
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		// Postgres
		rows, err := g.db.Query(`
			SELECT column_name, data_type, is_nullable 
			FROM information_schema.columns 
			WHERE table_name = $1 
			ORDER BY ordinal_position`, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var colName, dataType, isNullableStr string
				if err := rows.Scan(&colName, &dataType, &isNullableStr); err == nil {
					isPK := colName == "id"
					isNullable := isNullableStr == "YES"
					goType := MapSQLTypeToGoType(dataType, isNullable)

					columns = append(columns, ColumnInfo{
						Name:         colName,
						DataType:     dataType,
						GoType:       goType,
						IsNullable:   isNullable,
						IsPrimaryKey: isPK,
						JSONName:     colName,
					})
				}
			}
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
	}

	if len(columns) == 0 {
		columns = []ColumnInfo{
			{Name: "id", DataType: "uint", GoType: "uint", IsPrimaryKey: true, JSONName: "id"},
			{Name: "name", DataType: "varchar", GoType: "string", JSONName: "name", ValidateTag: `validate:"required"`},
			{Name: "created_at", DataType: "datetime", GoType: "time.Time", JSONName: "created_at"},
			{Name: "updated_at", DataType: "datetime", GoType: "time.Time", JSONName: "updated_at"},
		}
	}

	return columns, nil
}

// GetAllTables retrieves all user tables in the database
func (g *Generator) GetAllTables() ([]string, error) {
	var tables []string
	driver := database.GetDriver()

	if driver == "sqlite" {
		rows, err := g.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil && !g.IsProtectedTable(name) {
				tables = append(tables, name)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return tables, nil
	} else if driver == "mysql" {
		rows, err := g.db.Query("SHOW TABLES")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil && !g.IsProtectedTable(name) {
				tables = append(tables, name)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return tables, nil
	} else {
		rows, err := g.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil && !g.IsProtectedTable(name) {
				tables = append(tables, name)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return tables, nil
	}
}

// GenerateAll generates CRUD for all tables in the database
func (g *Generator) GenerateAll() error {
	tables, err := g.GetAllTables()
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		fmt.Println("⚠️ No tables found to generate.")
		return nil
	}

	fmt.Printf("🚀 Found %d tables. Starting batch CRUD generation...\n\n", len(tables))
	for _, t := range tables {
		if err := g.GenerateCRUD(t); err != nil {
			fmt.Printf("❌ Failed generating for table %s: %v\n", t, err)
		}
	}
	fmt.Println("🎉 All CRUDs generated successfully!")
	return nil
}

// GenerateCRUD generates Base Model, Model, Controller/Handler, and prints route snippet
func (g *Generator) GenerateCRUD(tableName string) error {
	if g.IsProtectedTable(tableName) {
		fmt.Printf("⚠️  Table '%s' is a protected core table. Skipping generator to preserve core logic.\n", tableName)
		return nil
	}

	modelName := TableNameToModelName(tableName)
	columns, err := g.GetTableColumns(tableName)
	if err != nil {
		return fmt.Errorf("failed to introspect table %s: %w", tableName, err)
	}

	fmt.Printf("📦 Generating CRUD for table: %s (Model: %s)...\n", tableName, modelName)

	// 1. Generate Base Model
	if err := g.generateBaseModel(tableName, modelName, columns); err != nil {
		return err
	}

	// 2. Generate Custom Model (If doesn't exist)
	if err := g.generateCustomModel(modelName); err != nil {
		return err
	}

	// 3. Generate Base Controller (Always overwritten with latest DB CRUD logic)
	if err := g.generateBaseController(tableName, modelName, columns); err != nil {
		return err
	}

	// 4. Generate Concrete Controller (Never overwritten - holds custom user actions)
	if err := g.generateConcreteController(modelName); err != nil {
		return err
	}

	// 5. Generate API Collection (Postman / Thunder Client / Insomnia)
	if err := g.generateAPICollection(tableName, modelName, columns); err != nil {
		return err
	}

	// 6. Print Routes registration instruction
	fmt.Printf("\n✨ Successfully generated CRUD for %s!\n", modelName)
	fmt.Println("👉 Register your routes in app/Routes/api.go:")
	fmt.Println("------------------------------------------------------------------")
	fmt.Printf("r.Route(\"/%s\", func(r chi.Router) {\n", strings.ToLower(tableName))
	fmt.Printf("    ctrl := controllers.New%sController()\n", modelName)
	fmt.Println("    r.Get(\"/\", ctrl.Index)")
	fmt.Println("    r.Post(\"/\", ctrl.Store)")
	fmt.Println("    r.Get(\"/{id}\", ctrl.Show)")
	fmt.Println("    r.Put(\"/{id}\", ctrl.Update)")
	fmt.Println("    r.Delete(\"/{id}\", ctrl.Destroy)")
	fmt.Println("})")
	fmt.Println("------------------------------------------------------------------")

	return nil
}

func (g *Generator) generateBaseModel(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "app", "Models", "Base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var fieldsCode strings.Builder
	hasTime := false

	for _, col := range columns {
		fieldName := ColumnToFieldName(col.Name)
		if strings.Contains(col.GoType, "time.Time") {
			hasTime = true
		}

		tag := fmt.Sprintf(`db:"%s" json:"%s"`, col.Name, col.JSONName)
		if col.ValidateTag != "" {
			tag += " " + col.ValidateTag
		}

		fieldsCode.WriteString(fmt.Sprintf("\t%s %s `%s`\n", fieldName, col.GoType, tag))
	}

	timeImport := ""
	if hasTime {
		timeImport = "\t\"time\"\n"
	}

	content := fmt.Sprintf(`// Code generated by Padi Generator. DO NOT EDIT.
package base

import (
%s)

type %s struct {
%s}

func (%s) TableName() string {
	return "%s"
}
`, timeImport, modelName, fieldsCode.String(), modelName, tableName)

	targetFile := filepath.Join(dir, modelName+".go")
	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateCustomModel(modelName string) error {
	dir := filepath.Join(g.baseDir, "app", "Models")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+".go")
	if _, err := os.Stat(targetFile); err == nil {
		return nil
	}

	content := fmt.Sprintf(`package models

import (
	base "padi_template/app/Models/Base"
	"github.com/wibiesana/padi-core/model"
	"github.com/wibiesana/padi-core/query"
)

type %s struct {
	base.%s
}

// Find finds record by ID
func (%s) Find(id interface{}) (*%s, error) {
	var item %s
	err := query.New("%s").Where("id", id).First(&item)
	return &item, err
}

// Save inserts or updates record automatically
func (m *%s) Save() error {
	return model.Save(m)
}

// Delete removes record
func (m *%s) Delete() error {
	return model.Delete(m)
}
`, modelName, modelName, modelName, modelName, modelName, strings.ToLower(modelName)+"s", modelName, modelName)

	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateBaseController(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "app", "Controllers", "Base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+"Controller.go")

	var searchCols []string
	for _, col := range columns {
		if strings.Contains(col.GoType, "string") {
			searchCols = append(searchCols, fmt.Sprintf(`"%s"`, col.Name))
		}
	}
	searchColsStr := strings.Join(searchCols, ", ")

	tmpl := `// Code generated by Padi Generator. DO NOT EDIT.
package base

import (
	"database/sql"
	"net/http"

	models "padi_template/app/Models"
	"github.com/wibiesana/padi-core/query"
	"github.com/wibiesana/padi-core/response"
	"github.com/wibiesana/padi-core/router"
	"github.com/wibiesana/padi-core/validator"
)

type {{ModelName}}Controller struct{}

// Index lists records with pagination and search
func (c *{{ModelName}}Controller) Index(w http.ResponseWriter, r *http.Request) {
	opts := query.ParseOptions(r)
	var records []models.{{ModelName}}

	searchColumns := []string{{{SearchColumns}}}
	meta, err := query.New("{{TableName}}").Paginate(opts, searchColumns, &records)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}} list")
		return
	}

	response.Paginated(w, records, meta, "{{ModelName}} list retrieved successfully")
}

// Show retrieves a single record by ID
func (c *{{ModelName}}Controller) Show(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := (models.{{ModelName}}{}).Find(id)
	if err != nil || item == nil || item.ID == 0 {
		if err == sql.ErrNoRows {
			response.NotFound(w, "{{ModelName}} not found")
			return
		}
		response.InternalServerError(w, "Failed to retrieve {{ModelName}}")
		return
	}

	response.Success(w, item, "{{ModelName}} retrieved successfully")
}

// Store creates a new record
func (c *{{ModelName}}Controller) Store(w http.ResponseWriter, r *http.Request) {
	var item models.{{ModelName}}
	if errs, err := validator.BindJSON(r, &item); err != nil {
		response.UnprocessableEntity(w, errs, "Validation failed")
		return
	}

	if err := item.Save(); err != nil {
		response.InternalServerError(w, "Failed to create {{ModelName}}: "+err.Error())
		return
	}

	response.Created(w, item, "{{ModelName}} created successfully")
}

// Update updates an existing record
func (c *{{ModelName}}Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := (models.{{ModelName}}{}).Find(id)
	if err != nil || item == nil || item.ID == 0 {
		if err == sql.ErrNoRows {
			response.NotFound(w, "{{ModelName}} not found")
			return
		}
		response.InternalServerError(w, "Failed to find {{ModelName}}")
		return
	}

	if errs, err := validator.BindJSON(r, item); err != nil {
		response.UnprocessableEntity(w, errs, "Validation failed")
		return
	}
	item.ID = id

	if err := item.Save(); err != nil {
		response.InternalServerError(w, "Failed to update {{ModelName}}")
		return
	}

	response.Success(w, item, "{{ModelName}} updated successfully")
}

// Destroy deletes a record
func (c *{{ModelName}}Controller) Destroy(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := (models.{{ModelName}}{}).Find(id)
	if err != nil || item == nil || item.ID == 0 {
		if err == sql.ErrNoRows {
			response.NotFound(w, "{{ModelName}} not found")
			return
		}
		response.InternalServerError(w, "Failed to find {{ModelName}}")
		return
	}

	if err := item.Delete(); err != nil {
		response.InternalServerError(w, "Failed to delete {{ModelName}}")
		return
	}

	response.Success(w, nil, "{{ModelName}} deleted successfully")
}
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{TableName}}", tableName)
	content = strings.ReplaceAll(content, "{{SearchColumns}}", searchColsStr)

	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateConcreteController(modelName string) error {
	dir := filepath.Join(g.baseDir, "app", "Controllers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+"Controller.go")
	if _, err := os.Stat(targetFile); err == nil {
		// Do not overwrite concrete controller if already exists
		return nil
	}

	tmpl := `package controllers

import (
	base "padi_template/app/Controllers/Base"
)

type {{ModelName}}Controller struct {
	base.{{ModelName}}Controller
}

func New{{ModelName}}Controller() *{{ModelName}}Controller {
	return &{{ModelName}}Controller{}
}

// Custom actions, overrides, or hooks can be added below
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateAPICollection(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "api_collection")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	routePrefix := strings.ToLower(tableName)

	sampleBodyMap := make(map[string]interface{})
	for _, col := range columns {
		if col.IsPrimaryKey || strings.Contains(col.Name, "_at") {
			continue
		}
		if strings.Contains(col.GoType, "int") {
			sampleBodyMap[col.JSONName] = 1
		} else if strings.Contains(col.GoType, "bool") {
			sampleBodyMap[col.JSONName] = true
		} else if strings.Contains(col.GoType, "float") {
			sampleBodyMap[col.JSONName] = 10.5
		} else {
			sampleBodyMap[col.JSONName] = fmt.Sprintf("Sample %s", col.JSONName)
		}
	}
	bodyBytes, _ := json.MarshalIndent(sampleBodyMap, "", "    ")
	bodyJSON := string(bodyBytes)

	collection := map[string]interface{}{
		"info": map[string]interface{}{
			"name":        fmt.Sprintf("%s API", modelName),
			"description": fmt.Sprintf("REST API endpoints for %s resource", modelName),
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"item": []map[string]interface{}{
			{
				"name": fmt.Sprintf("Get All %ss (Paginated)", modelName),
				"request": map[string]interface{}{
					"method": "GET",
					"header": []interface{}{},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/v1/%s?page=1&per_page=15&search=&sort=id&order=desc", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{"v1", routePrefix},
						"query": []map[string]string{
							{"key": "page", "value": "1"},
							{"key": "per_page", "value": "15"},
							{"key": "search", "value": ""},
							{"key": "sort", "value": "id"},
							{"key": "order", "value": "desc"},
						},
					},
				},
				"response": []interface{}{},
			},
			{
				"name": fmt.Sprintf("Get Single %s by ID", modelName),
				"request": map[string]interface{}{
					"method": "GET",
					"header": []interface{}{},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/v1/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{"v1", routePrefix, "1"},
					},
				},
				"response": []interface{}{},
			},
			{
				"name": fmt.Sprintf("Create %s", modelName),
				"request": map[string]interface{}{
					"method": "POST",
					"header": []map[string]string{
						{"key": "Content-Type", "value": "application/json"},
					},
					"body": map[string]interface{}{
						"mode": "raw",
						"raw":  bodyJSON,
					},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/v1/%s", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{"v1", routePrefix},
					},
				},
				"response": []interface{}{},
			},
			{
				"name": fmt.Sprintf("Update %s", modelName),
				"request": map[string]interface{}{
					"method": "PUT",
					"header": []map[string]string{
						{"key": "Content-Type", "value": "application/json"},
					},
					"body": map[string]interface{}{
						"mode": "raw",
						"raw":  bodyJSON,
					},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/v1/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{"v1", routePrefix, "1"},
					},
				},
				"response": []interface{}{},
			},
			{
				"name": fmt.Sprintf("Delete %s", modelName),
				"request": map[string]interface{}{
					"method": "DELETE",
					"header": []interface{}{},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/v1/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{"v1", routePrefix, "1"},
					},
				},
				"response": []interface{}{},
			},
		},
	}

	collectionBytes, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}

	targetFile := filepath.Join(dir, fmt.Sprintf("%s_api_collection.json", strings.ToLower(modelName)))
	if err := os.WriteFile(targetFile, collectionBytes, 0644); err != nil {
		return err
	}

	fmt.Printf("✓ API Collection created at %s\n", targetFile)
	return nil
}
