package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wibiesana/padi_go_core/database"
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
		protectedTables: []string{"users", "password_resets", "migrations", "jobs"},
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

	// 3. Generate Base Resource (Always overwritten with latest DB schema)
	if err := g.generateBaseResource(tableName, modelName, columns); err != nil {
		return err
	}

	// 4. Generate Concrete Resource (Never overwritten - holds custom user transformations)
	if err := g.generateConcreteResource(modelName); err != nil {
		return err
	}

	// 5. Generate Base Controller (Always overwritten with latest DB CRUD logic)
	if err := g.generateBaseController(tableName, modelName, columns); err != nil {
		return err
	}

	// 6. Generate Concrete Controller (Never overwritten - holds custom user actions)
	if err := g.generateConcreteController(modelName); err != nil {
		return err
	}

	// 7. Generate API Collection (Postman / Thunder Client / Insomnia)
	if err := g.generateAPICollection(tableName, modelName, columns); err != nil {
		return err
	}

	// 8. Auto-inject route into app/Routes/api.go
	if err := g.autoRegisterRoute(tableName, modelName); err != nil {
		fmt.Printf("⚠️ Notice: Could not automatically register route in api.go: %v\n", err)
	}

	fmt.Printf("\n✨ Successfully generated CRUD for %s!\n", modelName)
	return nil
}

// autoRegisterRoute automatically injects generated CRUD routes into app/Routes/api.go (inside protected group if exists)
func (g *Generator) autoRegisterRoute(tableName, modelName string) error {
	routesFile := filepath.Join(g.baseDir, "app", "Routes", "api.go")
	contentBytes, err := os.ReadFile(routesFile)
	if err != nil {
		return err
	}
	content := string(contentBytes)

	routePath := strings.ToLower(tableName)
	routeMarker := fmt.Sprintf(`r.Route("/%s"`, routePath)
	protectedRouteMarker := fmt.Sprintf(`protected.Route("/%s"`, routePath)

	// Check if already registered
	if strings.Contains(content, routeMarker) || strings.Contains(content, protectedRouteMarker) {
		fmt.Printf("✓ Route /%s is already registered in %s\n", routePath, routesFile)
		return nil
	}

	routeSnippet := fmt.Sprintf(`
		// %s CRUD Resource
		protected.Route("/%s", func(r chi.Router) {
			ctrl := controllers.New%sController()
			r.Get("/", ctrl.Index)
			r.Get("/all", ctrl.All)
			r.Post("/", ctrl.Store)
			r.Get("/{id}", ctrl.Show)
			r.Put("/{id}", ctrl.Update)
			r.Delete("/{id}", ctrl.Destroy)
		})`, modelName, routePath, modelName)

	// Target insertion inside protected group if available
	protectedTarget := "protected.Use(middleware.AuthRequired)"
	if idx := strings.Index(content, protectedTarget); idx != -1 {
		insertPos := idx + len(protectedTarget)
		newContent := content[:insertPos] + routeSnippet + content[insertPos:]
		if err := os.WriteFile(routesFile, []byte(newContent), 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Automatically registered protected route [/%s] in %s\n", routePath, routesFile)
		return nil
	}

	// Fallback: Insert before last closing brace in RegisterRoutes
	if lastBrace := strings.LastIndex(content, "}"); lastBrace != -1 {
		fallbackSnippet := fmt.Sprintf(`
	r.Mux.Route("/%s", func(r chi.Router) {
		ctrl := controllers.New%sController()
		r.Get("/", ctrl.Index)
		r.Get("/all", ctrl.All)
		r.Post("/", ctrl.Store)
		r.Get("/{id}", ctrl.Show)
		r.Put("/{id}", ctrl.Update)
		r.Delete("/{id}", ctrl.Destroy)
	})
`, routePath, modelName)
		newContent := content[:lastBrace] + fallbackSnippet + content[lastBrace:]
		if err := os.WriteFile(routesFile, []byte(newContent), 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Automatically registered route [/%s] in %s\n", routePath, routesFile)
		return nil
	}

	return nil
}

func (g *Generator) generateBaseModel(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "app", "Models", "Base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var fieldsCode strings.Builder
	hasTime := false
	var fillableCols []string
	pkCol := "id"
	var relationsCode strings.Builder
	hasRelations := false

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

		if col.IsPrimaryKey {
			pkCol = col.Name
		} else if !strings.Contains(col.Name, "_at") {
			fillableCols = append(fillableCols, fmt.Sprintf(`"%s"`, col.Name))
		}

		// Detect BelongsTo foreign keys (e.g. user_id -> users, author_id -> authors, category_id -> categories)
		if strings.HasSuffix(col.Name, "_id") {
			relName := strings.TrimSuffix(col.Name, "_id")
			relTable := relName + "s"
			if strings.HasSuffix(relName, "y") {
				relTable = strings.TrimSuffix(relName, "y") + "ies"
			}
			relationsCode.WriteString(fmt.Sprintf("\t\t\"%s\": activerecord.BelongsTo(\"%s\", \"%s\", \"id\"),\n", relName, relTable, col.Name))
			hasRelations = true
		}
	}

	var imports strings.Builder
	if hasTime {
		imports.WriteString("\t\"time\"\n")
	}
	if hasRelations {
		imports.WriteString("\t\"github.com/wibiesana/padi_go_core/activerecord\"\n")
	}

	relationsMethod := ""
	if hasRelations {
		relationsMethod = fmt.Sprintf(`
func (%s) Relations() map[string]activerecord.Relation {
	return map[string]activerecord.Relation{
%s	}
}
`, modelName, relationsCode.String())
	}

	importsBlock := ""
	if imports.Len() > 0 {
		importsBlock = fmt.Sprintf("\nimport (\n%s)\n", imports.String())
	}

	content := fmt.Sprintf(`// Code generated by Padi Generator. DO NOT EDIT.
package base
%s
type %s struct {
%s}

func (%s) TableName() string {
	return "%s"
}

func (%s) PrimaryKey() string {
	return "%s"
}

func (%s) Fillable() []string {
	return []string{%s}
}
%s`, importsBlock, modelName, fieldsCode.String(), modelName, tableName, modelName, pkCol, modelName, strings.Join(fillableCols, ", "), relationsMethod)

	targetFile := filepath.Join(dir, modelName+".go")
	return os.WriteFile(targetFile, []byte(content), 0644)
}

// GetModuleName resolves Go module name from go.mod or fallback
func (g *Generator) GetModuleName() string {
	goModPath := filepath.Join(g.baseDir, "go.mod")
	if content, err := os.ReadFile(goModPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}
	return "padi-template"
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

	moduleName := g.GetModuleName()
	tmpl := `package models

import (
	base "{{ModuleName}}/app/Models/Base"
)

type {{ModelName}} struct {
	base.{{ModelName}}
}

// Add custom methods, hooks (BeforeSave, etc.), or relationships here
`
	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)

	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateBaseResource(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "app", "Resources", "Base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+"Resource.go")
	moduleName := g.GetModuleName()

	var fieldsMapping strings.Builder
	for _, col := range columns {
		if strings.ToLower(col.Name) == "password" || strings.ToLower(col.Name) == "remember_token" {
			continue
		}
		fieldName := ColumnToFieldName(col.Name)
		fieldsMapping.WriteString(fmt.Sprintf("\t\t\"%s\": item.%s,\n", col.JSONName, fieldName))
	}

	tmpl := `// Code generated by Padi Generator. DO NOT EDIT.
package base

import (
	models "{{ModuleName}}/app/Models"
)

type {{ModelName}}Resource struct{}

// ToMap converts a single model instance to a map
func (r *{{ModelName}}Resource) ToMap(item *models.{{ModelName}}) map[string]interface{} {
	if item == nil {
		return nil
	}
	return map[string]interface{}{
{{FieldsMapping}}	}
}

// ToMapCollection converts a slice of models to a slice of maps
func (r *{{ModelName}}Resource) ToMapCollection(items []models.{{ModelName}}) []map[string]interface{} {
	if items == nil {
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, len(items))
	for i := range items {
		result[i] = r.ToMap(&items[i])
	}
	return result
}
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)
	content = strings.ReplaceAll(content, "{{FieldsMapping}}", fieldsMapping.String())

	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateConcreteResource(modelName string) error {
	dir := filepath.Join(g.baseDir, "app", "Resources")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+"Resource.go")
	if _, err := os.Stat(targetFile); err == nil {
		// Do not overwrite concrete resource if already exists
		return nil
	}

	moduleName := g.GetModuleName()
	tmpl := `package resources

import (
	models "{{ModuleName}}/app/Models"
	base "{{ModuleName}}/app/Resources/Base"
)

type {{ModelName}}Resource struct {
	base.{{ModelName}}Resource
}

func New{{ModelName}}Resource() *{{ModelName}}Resource {
	return &{{ModelName}}Resource{}
}

// ToMap transforms a single model. Add custom fields or relations here.
func (r *{{ModelName}}Resource) ToMap(item *models.{{ModelName}}) map[string]interface{} {
	data := r.{{ModelName}}Resource.ToMap(item)
	if data == nil {
		return nil
	}

	// Example: Add custom fields or relations to output:
	// data["custom_field"] = "custom_value"

	return data
}

// ToMapCollection transforms a slice of models.
func (r *{{ModelName}}Resource) ToMapCollection(items []models.{{ModelName}}) []map[string]interface{} {
	if items == nil {
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, len(items))
	for i := range items {
		result[i] = r.ToMap(&items[i])
	}
	return result
}
`
	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)

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
	moduleName := g.GetModuleName()

	tmpl := `// Code generated by Padi Generator. DO NOT EDIT.
package base

import (
	"database/sql"
	"net/http"

	models "{{ModuleName}}/app/Models"
	resources "{{ModuleName}}/app/Resources"
	"github.com/wibiesana/padi_go_core/activerecord"
	"github.com/wibiesana/padi_go_core/query"
	"github.com/wibiesana/padi_go_core/response"
	"github.com/wibiesana/padi_go_core/router"
	"github.com/wibiesana/padi_go_core/validator"
)

type {{ModelName}}Controller struct{}

// Index lists records with pagination and search
func (c *{{ModelName}}Controller) Index(w http.ResponseWriter, r *http.Request) {
	opts := query.ParseOptions(r)
	searchColumns := []string{{{SearchColumns}}}
	meta, records, err := activerecord.Paginate[models.{{ModelName}}](opts, searchColumns...)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}} list")
		return
	}

	res := resources.New{{ModelName}}Resource()
	response.Paginated(w, res.ToMapCollection(records), meta, "{{ModelName}} list retrieved successfully")
}

// All retrieves all records without pagination
func (c *{{ModelName}}Controller) All(w http.ResponseWriter, r *http.Request) {
	records, err := activerecord.All[models.{{ModelName}}]()
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve all {{ModelName}}")
		return
	}

	res := resources.New{{ModelName}}Resource()
	response.Items(w, res.ToMapCollection(records), "All {{ModelName}} retrieved successfully")
}

// Show retrieves a single record by ID
func (c *{{ModelName}}Controller) Show(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.Find[models.{{ModelName}}](id)
	if err != nil || item == nil {
		if err == sql.ErrNoRows || item == nil {
			response.NotFound(w, "{{ModelName}} not found")
			return
		}
		response.InternalServerError(w, "Failed to retrieve {{ModelName}}")
		return
	}

	res := resources.New{{ModelName}}Resource()
	response.Item(w, res.ToMap(item), "{{ModelName}} retrieved successfully")
}

// Store creates a new record
func (c *{{ModelName}}Controller) Store(w http.ResponseWriter, r *http.Request) {
	var item models.{{ModelName}}
	if errs, err := validator.BindJSON(r, &item); err != nil {
		response.UnprocessableEntity(w, errs, "Validation failed")
		return
	}

	if err := activerecord.Save(&item); err != nil {
		response.InternalServerError(w, "Failed to create {{ModelName}}: "+err.Error())
		return
	}

	res := resources.New{{ModelName}}Resource()
	response.Created(w, res.ToMap(&item), "{{ModelName}} created successfully")
}

// Update updates an existing record
func (c *{{ModelName}}Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.Find[models.{{ModelName}}](id)
	if err != nil || item == nil {
		response.NotFound(w, "{{ModelName}} not found")
		return
	}

	if errs, err := validator.BindJSON(r, item); err != nil {
		response.UnprocessableEntity(w, errs, "Validation failed")
		return
	}

	if err := activerecord.Save(item); err != nil {
		response.InternalServerError(w, "Failed to update {{ModelName}}")
		return
	}

	res := resources.New{{ModelName}}Resource()
	response.Item(w, res.ToMap(item), "{{ModelName}} updated successfully")
}

// Destroy deletes a record
func (c *{{ModelName}}Controller) Destroy(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.Find[models.{{ModelName}}](id)
	if err != nil || item == nil {
		response.NotFound(w, "{{ModelName}} not found")
		return
	}

	if err := activerecord.DeleteModel(item); err != nil {
		response.InternalServerError(w, "Failed to delete {{ModelName}}")
		return
	}

	response.Success(w, nil, "{{ModelName}} deleted successfully")
}
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{TableName}}", tableName)
	content = strings.ReplaceAll(content, "{{SearchColumns}}", searchColsStr)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)

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

	moduleName := g.GetModuleName()
	tmpl := `package controllers

import (
	base "{{ModuleName}}/app/Controllers/Base"
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
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)
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
						"raw":  fmt.Sprintf("{{base_url}}/%s?page=1&per_page=15&search=&sort=id&order=desc", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix},
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
						"raw":  fmt.Sprintf("{{base_url}}/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix, "1"},
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
						"raw":  fmt.Sprintf("{{base_url}}/%s", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix},
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
						"raw":  fmt.Sprintf("{{base_url}}/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix, "1"},
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
						"raw":  fmt.Sprintf("{{base_url}}/%s/1", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix, "1"},
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
