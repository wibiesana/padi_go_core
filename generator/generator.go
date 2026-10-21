package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wibiesana/padi_go_core/activerecord"
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
	realtime        bool
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
		realtime:        false,
	}
}

// WithRealtime enables or disables real-time SSE broadcasting in generated controllers
func (g *Generator) WithRealtime(enabled bool) *Generator {
	g.realtime = enabled
	return g
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

func getNonEmptyCondition(col ColumnInfo) string {
	fieldName := ColumnToFieldName(col.Name)
	if strings.Contains(col.GoType, "*") {
		if strings.Contains(col.GoType, "int") || strings.Contains(col.GoType, "uint") {
			return fmt.Sprintf("item.%s != nil && *item.%s > 0", fieldName, fieldName)
		}
		if strings.Contains(col.GoType, "string") {
			return fmt.Sprintf("item.%s != nil && *item.%s != \"\"", fieldName, fieldName)
		}
		return fmt.Sprintf("item.%s != nil", fieldName)
	}
	if strings.Contains(col.GoType, "int") || strings.Contains(col.GoType, "uint") {
		return fmt.Sprintf("item.%s > 0", fieldName)
	}
	if strings.Contains(col.GoType, "string") {
		return fmt.Sprintf("item.%s != \"\"", fieldName)
	}
	return fmt.Sprintf("item.%s != nil", fieldName)
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
				hasDefault := dflt != nil
				goType := MapSQLTypeToGoType(typeStr, isNullable)

				validateTag := ""
				if !isNullable && !isPK && !hasDefault && !strings.Contains(name, "_at") && name != "slug" && name != "status" {
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
				hasDefault := def != nil || strings.Contains(strings.ToLower(extra), "default")
				goType := MapSQLTypeToGoType(typeStr, isNullable)

				validateTag := ""
				if !isNullable && !isPK && !hasDefault && !strings.Contains(field, "_at") && field != "slug" && field != "status" {
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
			SELECT column_name, data_type, is_nullable, column_default 
			FROM information_schema.columns 
			WHERE table_name = $1 
			ORDER BY ordinal_position`, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var colName, dataType, isNullableStr string
				var colDefault *string
				if err := rows.Scan(&colName, &dataType, &isNullableStr, &colDefault); err == nil {
					isPK := colName == "id"
					isNullable := isNullableStr == "YES"
					hasDefault := colDefault != nil
					goType := MapSQLTypeToGoType(dataType, isNullable)

					validateTag := ""
					if !isNullable && !isPK && !hasDefault && !strings.Contains(colName, "_at") && colName != "slug" && colName != "status" {
						validateTag = "validate:\"required\""
					}

					columns = append(columns, ColumnInfo{
						Name:         colName,
						DataType:     dataType,
						GoType:       goType,
						IsNullable:   isNullable,
						IsPrimaryKey: isPK,
						JSONName:     colName,
						ValidateTag:  validateTag,
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

// ForeignKeyInfo holds foreign key metadata discovered from DB
type ForeignKeyInfo struct {
	ColumnName           string
	ReferencedTableName  string
	ReferencedColumnName string
	RelationName         string
}

// GetTableForeignKeys inspects foreign keys from the database schema
func (g *Generator) GetTableForeignKeys(tableName string) []ForeignKeyInfo {
	var fks []ForeignKeyInfo
	if g.db == nil {
		return nil
	}
	driver := database.GetDriver()
	if driver == "sqlite" {
		rows, err := g.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", tableName))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, seq int
				var table, from, to string
				var onUpdate, onDelete, match string
				if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err == nil {
					fks = append(fks, ForeignKeyInfo{
						ColumnName:           from,
						ReferencedTableName:  table,
						ReferencedColumnName: to,
					})
				}
			}
		}
	} else if driver == "mysql" {
		rows, err := g.db.Query(`
			SELECT COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = ?
			  AND REFERENCED_TABLE_NAME IS NOT NULL
		`, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var col, refTable, refCol string
				if err := rows.Scan(&col, &refTable, &refCol); err == nil {
					fks = append(fks, ForeignKeyInfo{
						ColumnName:           col,
						ReferencedTableName:  refTable,
						ReferencedColumnName: refCol,
					})
				}
			}
		}
	} else if driver == "postgres" {
		rows, err := g.db.Query(`
			SELECT kcu.column_name, ccu.table_name, ccu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1
		`, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var col, refTable, refCol string
				if err := rows.Scan(&col, &refTable, &refCol); err == nil {
					fks = append(fks, ForeignKeyInfo{
						ColumnName:           col,
						ReferencedTableName:  refTable,
						ReferencedColumnName: refCol,
					})
				}
			}
		}
	}

	usedNames := make(map[string]bool)
	for i := range fks {
		relName := strings.TrimSuffix(fks[i].ColumnName, "_id")
		if relName == "" || fks[i].ColumnName == "id" {
			relName = strings.TrimSuffix(fks[i].ReferencedTableName, "s")
			if relName == "" {
				relName = fks[i].ReferencedTableName
			}
		}

		if usedNames[relName] {
			if fks[i].ColumnName != "" && fks[i].ColumnName != "id" && !usedNames[fks[i].ColumnName] {
				relName = fks[i].ColumnName
			} else {
				base := relName
				idx := 2
				for usedNames[relName] {
					relName = fmt.Sprintf("%s_%d", base, idx)
					idx++
				}
			}
		}
		usedNames[relName] = true
		fks[i].RelationName = relName
	}
	return fks
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

	kebabPath := strings.ReplaceAll(strings.ToLower(tableName), "_", "-")
	snakePath := strings.ToLower(tableName)

	routeMarker := fmt.Sprintf(`Route("/%s"`, kebabPath)
	protectedRouteMarker := fmt.Sprintf(`protected.Route("/%s"`, kebabPath)

	// Check if already registered
	if strings.Contains(content, routeMarker) || strings.Contains(content, protectedRouteMarker) {
		fmt.Printf("✓ Route /%s is already registered in %s\n", kebabPath, routesFile)
		return nil
	}

	buildSnippet := func(prefix string) string {
		if kebabPath != snakePath {
			return fmt.Sprintf(`
		// %s CRUD Resource
		reg%s := func(r router.Route) {
			ctrl := controllers.New%sController()
			r.Get("/", ctrl.Index)
			r.Get("/all", ctrl.All)
			r.Post("/", ctrl.Store)
			r.Get("/{id}", ctrl.Show)
			r.Put("/{id}", ctrl.Update)
			r.Delete("/{id}", ctrl.Destroy)
		}
		%sRoute("/%s", reg%s)
		%sRoute("/%s", reg%s)
`, modelName, modelName, modelName, prefix, kebabPath, modelName, prefix, snakePath, modelName)
		}

		return fmt.Sprintf(`
		// %s CRUD Resource
		%sRoute("/%s", func(r router.Route) {
			ctrl := controllers.New%sController()
			r.Get("/", ctrl.Index)
			r.Get("/all", ctrl.All)
			r.Post("/", ctrl.Store)
			r.Get("/{id}", ctrl.Show)
			r.Put("/{id}", ctrl.Update)
			r.Delete("/{id}", ctrl.Destroy)
		})
`, modelName, prefix, kebabPath, modelName)
	}

	// Target insertion: Look for protected API Resources block
	apiResourcesTarget := "// Users CRUD Resource"
	if idx := strings.Index(content, apiResourcesTarget); idx != -1 {
		routeSnippet := buildSnippet("protected.")
		newContent := content[:idx] + routeSnippet + "\t\t" + content[idx:]
		if err := os.WriteFile(routesFile, []byte(newContent), 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Automatically registered protected route [/%s] in %s\n", kebabPath, routesFile)
		return nil
	}

	// Secondary Target: Look for r.Group or r.Mux.Group
	for _, target := range []string{
		"r.Group(func(protected router.Route) {",
		"r.Group(func(protected chi.Router) {",
		"r.Mux.Group(func(protected router.Route) {",
		"r.Mux.Group(func(protected chi.Router) {",
	} {
		if idx := strings.LastIndex(content, target); idx != -1 {
			routeSnippet := buildSnippet("protected.")
			insertPos := idx + len(target)
			newContent := content[:insertPos] + routeSnippet + content[insertPos:]
			if err := os.WriteFile(routesFile, []byte(newContent), 0644); err != nil {
				return err
			}
			fmt.Printf("✓ Automatically registered protected route [/%s] in %s\n", kebabPath, routesFile)
			return nil
		}
	}

	// Fallback: Insert before last closing brace in RegisterRoutes
	if lastBrace := strings.LastIndex(content, "}"); lastBrace != -1 {
		fallbackSnippet := buildSnippet("r.")
		newContent := content[:lastBrace] + fallbackSnippet + content[lastBrace:]
		if err := os.WriteFile(routesFile, []byte(newContent), 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Automatically registered route [/%s] in %s\n", kebabPath, routesFile)
		return nil
	}

	return nil
}

// isSensitiveColumn checks if a column name matches sensitive field patterns (password, token, secret, key)
func isSensitiveColumn(colName string) bool {
	lower := strings.ToLower(colName)
	for _, pattern := range []string{"password", "token", "secret", "key"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func (g *Generator) generateBaseModel(tableName, modelName string, columns []ColumnInfo) error {
	dir := filepath.Join(g.baseDir, "app", "Models", "Base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var fieldsCode strings.Builder
	fieldsCode.WriteString("\tactiverecord.ActiveRecord\n")
	hasTime := false
	var fillableCols []string
	pkCol := "id"
	var relationsCode strings.Builder
	hasRelations := false

	var hiddenCols []string
	for _, col := range columns {
		fieldName := ColumnToFieldName(col.Name)
		if strings.Contains(col.GoType, "time.Time") {
			hasTime = true
		}

		jsonTag := col.JSONName
		if isSensitiveColumn(col.Name) {
			jsonTag = "-"
			hiddenCols = append(hiddenCols, fmt.Sprintf(`"%s"`, col.Name))
		}
		tag := fmt.Sprintf(`db:"%s" json:"%s"`, col.Name, jsonTag)
		if col.ValidateTag != "" {
			tag += " " + col.ValidateTag
		}

		fieldsCode.WriteString(fmt.Sprintf("\t%s %s `%s`\n", fieldName, col.GoType, tag))

		if col.IsPrimaryKey {
			pkCol = col.Name
		} else if !strings.Contains(col.Name, "_at") {
			fillableCols = append(fillableCols, fmt.Sprintf(`"%s"`, col.Name))
		}
	}

	// Detect relationships from database foreign keys
	fks := g.GetTableForeignKeys(tableName)
	for _, fk := range fks {
		relName := fk.RelationName
		targetPK := fk.ReferencedColumnName
		if targetPK == "" {
			targetPK = "id"
		}
		displayCol := activerecord.GetTableDisplayColumn(fk.ReferencedTableName)
		if displayCol == targetPK {
			relationsCode.WriteString(fmt.Sprintf("\t\t\"%s\": activerecord.BelongsTo(\"%s\", \"%s\", \"%s\").WithColumns(\"%s\"),\n", relName, fk.ReferencedTableName, fk.ColumnName, targetPK, targetPK))
		} else {
			relationsCode.WriteString(fmt.Sprintf("\t\t\"%s\": activerecord.BelongsTo(\"%s\", \"%s\", \"%s\").WithColumns(\"%s\", \"%s\"),\n", relName, fk.ReferencedTableName, fk.ColumnName, targetPK, targetPK, displayCol))
		}
		hasRelations = true
	}

	var imports strings.Builder
	imports.WriteString("\t\"github.com/wibiesana/padi_go_core/activerecord\"\n")
	if hasTime {
		imports.WriteString("\t\"time\"\n")
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

	importsBlock := fmt.Sprintf("\nimport (\n%s)\n", imports.String())

	content := fmt.Sprintf(`// Code generated by Padi Generator. DO NOT EDIT.
package base
%s
type %s struct {
%s}

func init() {
	activerecord.RegisterModel(&%s{})
}

func (%s) TableName() string {
	return "%s"
}

func (%s) PrimaryKey() string {
	return "%s"
}

func (%s) Fillable() []string {
	return []string{%s}
}

func (%s) Hidden() []string {
	return []string{%s}
}
%s`, importsBlock, modelName, fieldsCode.String(), modelName, modelName, tableName, modelName, pkCol, modelName, strings.Join(fillableCols, ", "), modelName, strings.Join(hiddenCols, ", "), relationsMethod)

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
	var relationsCode strings.Builder

	for _, col := range columns {
		if isSensitiveColumn(col.Name) {
			continue
		}
		fieldName := ColumnToFieldName(col.Name)
		fieldsMapping.WriteString(fmt.Sprintf("\tm.Set(\"%s\", item.%s)\n", col.JSONName, fieldName))
	}

	fks := g.GetTableForeignKeys(tableName)
	for _, fk := range fks {
		relName := fk.RelationName
		fieldName := ColumnToFieldName(fk.ColumnName)
		displayCol := activerecord.GetTableDisplayColumn(fk.ReferencedTableName)
		relationsCode.WriteString(fmt.Sprintf(`
	if !m.Has("%s") && item.%s != 0 {
		if rel, _ := activerecord.FindRelation("%s", item.%s, "%s"); rel != nil {
			m.Set("%s", rel)
		}
	}
`, relName, fieldName, fk.ReferencedTableName, fieldName, displayCol, relName))
	}

	importBlock := fmt.Sprintf(`import (
	"github.com/wibiesana/padi_go_core/activerecord"
	models "%s/app/Models"
)`, moduleName)

	tmpl := `// Code generated by Padi Generator. DO NOT EDIT.
package base

{{ImportBlock}}

type {{ModelName}}Resource struct{}

// ToMap converts a single model instance to an ordered map
func (r *{{ModelName}}Resource) ToMap(item *models.{{ModelName}}) *activerecord.Map {
	if item == nil {
		return nil
	}
	m := activerecord.NewMap()
{{FieldsMapping}}
	// Attach eager-loaded relations if available
	if rels := activerecord.GetModelRelations(item); len(rels) > 0 {
		for k, v := range rels {
			m.Set(k, v)
		}
	}
{{RelationsCode}}
	return m
}

// ToMapCollection converts a slice of models to a slice of ordered maps
func (r *{{ModelName}}Resource) ToMapCollection(items []models.{{ModelName}}) []*activerecord.Map {
	if items == nil {
		return []*activerecord.Map{}
	}
	result := make([]*activerecord.Map, len(items))
	for i := range items {
		result[i] = r.ToMap(&items[i])
	}
	return result
}
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)
	content = strings.ReplaceAll(content, "{{ImportBlock}}", importBlock)
	content = strings.ReplaceAll(content, "{{FieldsMapping}}", fieldsMapping.String())
	content = strings.ReplaceAll(content, "{{RelationsCode}}", relationsCode.String())

	return os.WriteFile(targetFile, []byte(content), 0644)
}

func (g *Generator) generateConcreteResource(modelName string) error {
	dir := filepath.Join(g.baseDir, "app", "Resources")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	targetFile := filepath.Join(dir, modelName+"Resource.go")
	if _, err := os.Stat(targetFile); err == nil {
		return nil
	}

	moduleName := g.GetModuleName()
	tmpl := `package resources

import (
	"github.com/wibiesana/padi_go_core/activerecord"
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
func (r *{{ModelName}}Resource) ToMap(item *models.{{ModelName}}) *activerecord.Map {
	m := r.{{ModelName}}Resource.ToMap(item)
	if m == nil {
		return nil
	}

	// Example: Add custom fields or relations to output:
	// m.Set("custom_field", "custom_value")

	return m
}

// ToMapCollection transforms a slice of models.
func (r *{{ModelName}}Resource) ToMapCollection(items []models.{{ModelName}}) []*activerecord.Map {
	if items == nil {
		return []*activerecord.Map{}
	}
	result := make([]*activerecord.Map, len(items))
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

	var withRelations []string
	fks := g.GetTableForeignKeys(tableName)
	for _, fk := range fks {
		relName := fk.RelationName
		targetPK := fk.ReferencedColumnName
		if targetPK == "" {
			targetPK = "id"
		}
		displayCol := activerecord.GetTableDisplayColumn(fk.ReferencedTableName)
		if displayCol == targetPK {
			withRelations = append(withRelations, fmt.Sprintf(`"%s:%s"`, relName, targetPK))
		} else {
			withRelations = append(withRelations, fmt.Sprintf(`"%s:%s,%s"`, relName, targetPK, displayCol))
		}
	}
	withRelationsStr := strings.Join(withRelations, ", ")
	moduleName := g.GetModuleName()

	realtimeImport := ""
	realtimeStoreBroadcast := ""
	realtimeUpdateBroadcast := ""
	realtimeDestroyBroadcast := ""

	if g.realtime {
		realtimeImport = "\n\t\"github.com/wibiesana/padi_go_core/realtime\""
		realtimeStoreBroadcast = fmt.Sprintf(`
	// 📡 Broadcast real-time CREATE event
	realtime.Publish("%s", map[string]interface{}{
		"action": "CREATE",
		"data":   res.ToMap(&item),
	})
`, strings.ToLower(tableName))
		realtimeUpdateBroadcast = fmt.Sprintf(`
	// 📡 Broadcast real-time UPDATE event
	realtime.Publish("%s", map[string]interface{}{
		"action": "UPDATE",
		"data":   res.ToMap(item),
	})
`, strings.ToLower(tableName))
		realtimeDestroyBroadcast = fmt.Sprintf(`
	// 📡 Broadcast real-time DELETE event
	realtime.Publish("%s", map[string]interface{}{
		"action": "DELETE",
		"id":     id,
	})
`, strings.ToLower(tableName))
	}

	tmpl := `// Code generated by Padi Generator. DO NOT EDIT.
package base

import (
	"net/http"
	"strings"

	models "{{ModuleName}}/app/Models"
	resources "{{ModuleName}}/app/Resources"
	"github.com/wibiesana/padi_go_core/activerecord"
	"github.com/wibiesana/padi_go_core/query"{{RealtimeImport}}
	"github.com/wibiesana/padi_go_core/response"
	"github.com/wibiesana/padi_go_core/router"
	"github.com/wibiesana/padi_go_core/validator"
)

type {{ModelName}}Controller struct{}

// Index lists records with pagination, search, and dynamic eager loading (?with=...)
func (c *{{ModelName}}Controller) Index(w http.ResponseWriter, r *http.Request) {
	opts := query.ParseOptions(r)
	searchColumns := []string{{{SearchColumns}}}
	meta, records, err := activerecord.PaginateWithContext[models.{{ModelName}}](r.Context(), opts, searchColumns...)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}} list")
		return
	}

	withRelations := []string{{{WithRelations}}}
	if r.URL.Query().Get("with") != "" {
		_ = activerecord.LoadRelationsFromRequest(r, &records)
	} else if len(withRelations) > 0 {
		_ = activerecord.LoadRelations(&records, withRelations...)
	}

	res := resources.New{{ModelName}}Resource()
	response.Paginated(w, res.ToMapCollection(records), meta, "{{ModelName}} list retrieved successfully")
}

// All retrieves all records with search, sorting, and optional dynamic eager loading (?with=...)
func (c *{{ModelName}}Controller) All(w http.ResponseWriter, r *http.Request) {
	opts := query.ParseOptions(r)
	searchColumns := []string{{{SearchColumns}}}
	records, err := activerecord.AllWithOptsWithContext[models.{{ModelName}}](r.Context(), opts, searchColumns...)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve all {{ModelName}}")
		return
	}

	withRelations := []string{{{WithRelations}}}
	if r.URL.Query().Get("with") != "" {
		_ = activerecord.LoadRelationsFromRequest(r, &records)
	} else if len(withRelations) > 0 {
		_ = activerecord.LoadRelations(&records, withRelations...)
	}

	res := resources.New{{ModelName}}Resource()
	response.Items(w, res.ToMapCollection(records), "All {{ModelName}} retrieved successfully")
}

// Show retrieves a single record by ID with optional dynamic eager loading (?with=...)
func (c *{{ModelName}}Controller) Show(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.FindWithContext[models.{{ModelName}}](r.Context(), id)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}}")
		return
	}
	if item == nil {
		response.NotFound(w, "{{ModelName}} not found")
		return
	}

	withRelations := []string{{{WithRelations}}}
	if r.URL.Query().Get("with") != "" {
		_ = activerecord.LoadRelationsFromRequest(r, item)
	} else if len(withRelations) > 0 {
		_ = activerecord.LoadRelations(item, withRelations...)
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

	if err := activerecord.Insert(&item, r.Context()); err != nil {
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "duplicate") || strings.Contains(errLower, "unique") {
			response.Conflict(w, "{{ModelName}} already exists with the given data")
			return
		}
		response.InternalServerError(w, "Failed to create {{ModelName}}: "+err.Error())
		return
	}

	withRelations := []string{{{WithRelations}}}
	if len(withRelations) > 0 {
		_ = activerecord.LoadRelations(&item, withRelations...)
	}

	res := resources.New{{ModelName}}Resource(){{RealtimeStoreBroadcast}}
	response.Created(w, res.ToMap(&item), "{{ModelName}} created successfully")
}

// Update updates an existing record
func (c *{{ModelName}}Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.FindWithContext[models.{{ModelName}}](r.Context(), id)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}}")
		return
	}
	if item == nil {
		response.NotFound(w, "{{ModelName}} not found")
		return
	}

	if errs, err := validator.BindJSON(r, item); err != nil {
		response.UnprocessableEntity(w, errs, "Validation failed")
		return
	}

	if err := activerecord.Save(item, r.Context()); err != nil {
		response.InternalServerError(w, "Failed to update {{ModelName}}")
		return
	}

	withRelations := []string{{{WithRelations}}}
	if len(withRelations) > 0 {
		_ = activerecord.LoadRelations(item, withRelations...)
	}

	res := resources.New{{ModelName}}Resource(){{RealtimeUpdateBroadcast}}
	response.Item(w, res.ToMap(item), "{{ModelName}} updated successfully")
}

// Destroy deletes a record
func (c *{{ModelName}}Controller) Destroy(w http.ResponseWriter, r *http.Request) {
	id, err := router.ParamUint(r, "id")
	if err != nil {
		response.BadRequest(w, "Invalid ID parameter")
		return
	}

	item, err := activerecord.FindWithContext[models.{{ModelName}}](r.Context(), id)
	if err != nil {
		response.InternalServerError(w, "Failed to retrieve {{ModelName}}")
		return
	}
	if item == nil {
		response.NotFound(w, "{{ModelName}} not found")
		return
	}

	if err := activerecord.DeleteModel(item, r.Context()); err != nil {
		response.InternalServerError(w, "Failed to delete {{ModelName}}")
		return
	}
{{RealtimeDestroyBroadcast}}
	response.Success(w, nil, "{{ModelName}} deleted successfully")
}
`

	content := strings.ReplaceAll(tmpl, "{{ModelName}}", modelName)
	content = strings.ReplaceAll(content, "{{TableName}}", tableName)
	content = strings.ReplaceAll(content, "{{SearchColumns}}", searchColsStr)
	content = strings.ReplaceAll(content, "{{WithRelations}}", withRelationsStr)
	content = strings.ReplaceAll(content, "{{ModuleName}}", moduleName)
	content = strings.ReplaceAll(content, "{{RealtimeImport}}", realtimeImport)
	content = strings.ReplaceAll(content, "{{RealtimeStoreBroadcast}}", realtimeStoreBroadcast)
	content = strings.ReplaceAll(content, "{{RealtimeUpdateBroadcast}}", realtimeUpdateBroadcast)
	content = strings.ReplaceAll(content, "{{RealtimeDestroyBroadcast}}", realtimeDestroyBroadcast)

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

	routePrefix := strings.ReplaceAll(strings.ToLower(tableName), "_", "-")

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
		} else if strings.HasSuffix(strings.ToLower(col.Name), "_url") || strings.Contains(strings.ToLower(col.Name), "image") {
			sampleBodyMap[col.JSONName] = "https://example.com/sample.jpg"
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
		"auth": map[string]interface{}{
			"type": "bearer",
			"bearer": []map[string]string{
				{
					"key":   "token",
					"value": "{{token}}",
					"type":  "string",
				},
			},
		},
		"variable": []map[string]string{
			{
				"key":   "base_url",
				"value": "http://localhost:8080",
				"type":  "string",
			},
			{
				"key":   "token",
				"value": "",
				"type":  "string",
			},
		},
		"item": []map[string]interface{}{
			{
				"name": fmt.Sprintf("Get All %ss (Paginated)", modelName),
				"request": map[string]interface{}{
					"method": "GET",
					"header": []interface{}{},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/%s?page=1&per-page=25&search=&sort_by=id&order=desc", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix},
						"query": []map[string]string{
							{"key": "page", "value": "1"},
							{"key": "per-page", "value": "25"},
							{"key": "search", "value": ""},
							{"key": "sort_by", "value": "id"},
							{"key": "order", "value": "desc"},
						},
					},
				},
				"response": []interface{}{},
			},
			{
				"name": fmt.Sprintf("Get All %ss (No Pagination)", modelName),
				"request": map[string]interface{}{
					"method": "GET",
					"header": []interface{}{},
					"url": map[string]interface{}{
						"raw":  fmt.Sprintf("{{base_url}}/%s/all?search=&sort=id&order=desc", routePrefix),
						"host": []string{"{{base_url}}"},
						"path": []string{routePrefix, "all"},
						"query": []map[string]string{
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
