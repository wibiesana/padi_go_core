package activerecord

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wibiesana/padi_go_core/auth"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/middleware"
	"github.com/wibiesana/padi_go_core/query"
	"github.com/wibiesana/padi_go_core/response"
)

// Model is the base interface that every Padi ActiveRecord model must implement
type Model interface {
	TableName() string
}

// PrimaryKeyer allows a model to specify custom or composite primary key column(s)
type PrimaryKeyer interface {
	PrimaryKey() string
}

// Connectioner allows a model to specify a custom database connection
type Connectioner interface {
	ConnectionName() string
}

// Fillabler specifies allowed fillable fields for mass-assignment
type Fillabler interface {
	Fillable() []string
}

// HiddenFields specifies fields that should be hidden from serialization
type HiddenFields interface {
	Hidden() []string
}

// DefaultOrderer specifies default ordering clause for the model
type DefaultOrderer interface {
	DefaultOrder() string
}

// Auditable allows a model to customize audit behavior and column mappings
type Auditable interface {
	UseAudit() bool
	AuditFields() map[string]string
}

// RelationDefiner allows models to define named relationships
type RelationDefiner interface {
	Relations() map[string]Relation
}

// Lifecycle Hooks Interfaces
type BeforeSaver interface {
	BeforeSave(isInsert bool) error
}

type AfterSaver interface {
	AfterSave(isInsert bool)
}

type BeforeDeleter interface {
	BeforeDelete() error
}

type AfterDeleter interface {
	AfterDelete()
}

type AfterLoader interface {
	AfterLoad()
}

// RelationType defines the kind of relationship
type RelationType string

const (
	RelHasOne        RelationType = "hasOne"
	RelHasMany       RelationType = "hasMany"
	RelBelongsTo     RelationType = "belongsTo"
	RelBelongsToMany RelationType = "belongsToMany"
)

// Relation definition metadata
type Relation struct {
	Type       RelationType
	ModelName  string
	Table      string
	ForeignKey string
	LocalKey   string
	OwnerKey   string
	PivotTable string
	RelatedKey string
	Columns    []string
}

// Table column cache & TTL
type columnCacheEntry struct {
	columns  []string
	cachedAt time.Time
}

// relationCacheEntry holds a cached relation result with an expiry timestamp
type relationCacheEntry struct {
	value    *Map
	cachedAt time.Time
}

// orderByRegex validates ORDER BY segment — compiled once at package init
var orderByRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*(` + "`" + `|\s+(?i:ASC|DESC))?$`)

var (
	columnsCache    = make(map[string]columnCacheEntry)
	columnsCacheMu  sync.RWMutex
	columnsCacheTTL = time.Hour

	// relationCacheTTL controls how long FindRelation results are cached (default 5 minutes)
	relationCacheTTL = 5 * time.Minute
)


// ClearColumnsCache resets cached table columns
func ClearColumnsCache() {
	columnsCacheMu.Lock()
	defer columnsCacheMu.Unlock()
	columnsCache = make(map[string]columnCacheEntry)
}

// GetTable returns table name for model
func GetTable[T Model]() string {
	var zero T
	return zero.TableName()
}

// GetDb returns database connection for model or default DB
func GetDb(m ...Model) *sql.DB {
	return database.GetDB()
}

// GetConnectionName returns connection name if model implements Connectioner
func GetConnectionName(m ...Model) string {
	if len(m) > 0 && m[0] != nil {
		if c, ok := m[0].(Connectioner); ok {
			return c.ConnectionName()
		}
	}
	return ""
}

// GetLikeOperator returns LIKE or ILIKE based on current database driver
func GetLikeOperator() string {
	driver := database.GetDriver()
	if driver == "postgres" || driver == "pgsql" || driver == "postgresql" {
		return "ILIKE"
	}
	return "LIKE"
}

// GetPrimaryKeyName returns the primary key column name for model (defaults to "id")
func GetPrimaryKeyName(m any) string {
	if pk, ok := m.(PrimaryKeyer); ok {
		name := pk.PrimaryKey()
		if name != "" {
			return name
		}
	}
	return "id"
}

// HasOne defines a 1-to-1 relationship
func HasOne(table string, foreignKey string, localKey ...string) Relation {
	lk := "id"
	if len(localKey) > 0 && localKey[0] != "" {
		lk = localKey[0]
	}
	return Relation{
		Type:       RelHasOne,
		Table:      table,
		ForeignKey: foreignKey,
		LocalKey:   lk,
	}
}

// HasMany defines a 1-to-many relationship
func HasMany(table string, foreignKey string, localKey ...string) Relation {
	lk := "id"
	if len(localKey) > 0 && localKey[0] != "" {
		lk = localKey[0]
	}
	return Relation{
		Type:       RelHasMany,
		Table:      table,
		ForeignKey: foreignKey,
		LocalKey:   lk,
	}
}

// BelongsTo defines an inverse relationship
func BelongsTo(table string, foreignKey string, ownerKey ...string) Relation {
	ok := "id"
	if len(ownerKey) > 0 && ownerKey[0] != "" {
		ok = ownerKey[0]
	}
	return Relation{
		Type:       RelBelongsTo,
		Table:      table,
		ForeignKey: ok,
		LocalKey:   foreignKey,
	}
}

// BelongsToMany defines a many-to-many relationship through a pivot table
func BelongsToMany(table string, pivotTable string, foreignKey string, relatedKey string) Relation {
	return Relation{
		Type:       RelBelongsToMany,
		Table:      table,
		PivotTable: pivotTable,
		ForeignKey: foreignKey,
		RelatedKey: relatedKey,
		LocalKey:   "id",
	}
}

// Aliases with Rel suffix for backward compatibility
var (
	HasOneRel        = HasOne
	HasManyRel       = HasMany
	BelongsToRel     = BelongsTo
	BelongsToManyRel = BelongsToMany
)

// GetRelationConfig retrieves relation definition by name from model
func GetRelationConfig(m Model, name string) *Relation {
	if rd, ok := m.(RelationDefiner); ok {
		relMap := rd.Relations()
		if rel, exists := relMap[name]; exists {
			return &rel
		}
	}
	return nil
}

// GetWith returns eager loaded relations on model if any
func GetWith(m Model) []string {
	return nil
}

// With starts a model-aware query builder with eager loading
func With[T Model](relations ...string) *query.Query {
	var zero T
	return query.New(zero.TableName())
}

// ClearWith clears eager loading configuration
func ClearWith() {}

// FindBuilder starts a new model query builder
func FindBuilder[T Model]() *query.Query {
	var zero T
	return query.New(zero.TableName())
}

// FindQuery is alias for FindBuilder
func FindQuery[T Model]() *query.Query {
	return FindBuilder[T]()
}

// Search builds a model-aware multi-column LIKE search query
func Search[T Model](keyword string, extraColumns ...string) *query.Query {
	var zero T
	tableName := zero.TableName()
	q := query.New(tableName)

	if keyword == "" {
		return q
	}

	cols, err := GetTableColumns(tableName)
	var searchCols []string
	if err == nil && len(cols) > 0 {
		for _, col := range cols {
			if !strings.Contains(col, "_at") && !strings.Contains(col, "password") {
				searchCols = append(searchCols, col)
			}
		}
	}

	if len(extraColumns) > 0 {
		searchCols = append(searchCols, extraColumns...)
	}

	if len(searchCols) > 0 {
		q.Search(keyword, searchCols...)
	}
	return q
}

// SanitizeOrderBy sanitizes an ORDER BY clause to prevent SQL injection.
// Uses a package-level pre-compiled regex for performance.
func SanitizeOrderBy(orderBy string) (string, error) {
	if orderBy == "" {
		return "", nil
	}
	segments := strings.Split(orderBy, ",")
	var validSegments []string

	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		if !orderByRegex.MatchString(trimmed) {
			return "", fmt.Errorf("invalid ORDER BY segment: %s", trimmed)
		}
		validSegments = append(validSegments, trimmed)
	}
	return strings.Join(validSegments, ", "), nil
}

// GetPkConditions maps ID value or map to primary key conditions
func GetPkConditions(m Model, id interface{}) (map[string]interface{}, error) {
	pkName := GetPrimaryKeyName(m)
	conditions := make(map[string]interface{})

	if idMap, ok := id.(map[string]interface{}); ok {
		return idMap, nil
	}
	conditions[pkName] = id
	return conditions, nil
}

// Find retrieves a single record by primary key (id)
func Find[T Model](id interface{}, columns ...string) (*T, error) {
	return FindByPk[T](id, columns...)
}

// FindWithContext retrieves a single record by primary key with context
func FindWithContext[T Model](ctx context.Context, id interface{}, columns ...string) (*T, error) {
	return FindByPkWithContext[T](ctx, id, columns...)
}

// FindByPk retrieves record by primary key
func FindByPk[T Model](id interface{}, columns ...string) (*T, error) {
	return FindByPkWithContext[T](context.TODO(), id, columns...)
}

// FindByPkWithContext retrieves record by primary key with context
func FindByPkWithContext[T Model](ctx context.Context, id interface{}, columns ...string) (*T, error) {
	var item T
	pkName := GetPrimaryKeyName(item)
	q := query.New(item.TableName())
	if ctx != nil {
		q.WithContext(ctx)
	}
	if len(columns) > 0 {
		q.Select(columns...)
	}

	err := q.Where(pkName, id).First(&item)
	if err != nil {
		return nil, err
	}
	if hook, ok := any(&item).(AfterLoader); ok {
		hook.AfterLoad()
	}
	return &item, nil
}

// FindOne finds a single record by ID or conditions
func FindOne[T Model](condition interface{}, columns ...string) (*T, error) {
	return FindOneWithContext[T](context.TODO(), condition, columns...)
}

// FindOneWithContext finds a single record by ID or conditions with context
func FindOneWithContext[T Model](ctx context.Context, condition interface{}, columns ...string) (*T, error) {
	var item T
	q := query.New(item.TableName())
	if ctx != nil {
		q.WithContext(ctx)
	}
	if len(columns) > 0 {
		q.Select(columns...)
	}

	if condMap, ok := condition.(map[string]interface{}); ok {
		for col, val := range condMap {
			q.Where(col, val)
		}
		err := q.First(&item)
		if err != nil {
			return nil, err
		}
		if hook, ok := any(&item).(AfterLoader); ok {
			hook.AfterLoad()
		}
		return &item, nil
	}

	return FindByPkWithContext[T](ctx, condition, columns...)
}

// FindAll finds multiple records by primary key(s) or condition map
func FindAll[T Model](condition ...interface{}) ([]T, error) {
	var zero T
	q := query.New(zero.TableName())

	if len(condition) > 0 && condition[0] != nil {
		cond := condition[0]
		rv := reflect.ValueOf(cond)

		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			pkName := GetPrimaryKeyName(zero)
			var ids []interface{}
			for i := 0; i < rv.Len(); i++ {
				ids = append(ids, rv.Index(i).Interface())
			}
			q.WhereIn(pkName, ids...)
		} else if condMap, ok := cond.(map[string]interface{}); ok {
			for col, val := range condMap {
				q.Where(col, val)
			}
		}
	}

	var records []T
	err := q.All(&records)
	if err != nil {
		return nil, err
	}

	for i := range records {
		if hook, ok := any(&records[i]).(AfterLoader); ok {
			hook.AfterLoad()
		}
	}
	return records, nil
}

// FindOrFail retrieves a single record by primary key or returns 404 error
func FindOrFail[T Model](id interface{}, columns ...string) (*T, error) {
	return FindOrFailWithContext[T](context.TODO(), id, columns...)
}

// FindOrFailWithContext retrieves a single record by primary key with context
func FindOrFailWithContext[T Model](ctx context.Context, id interface{}, columns ...string) (*T, error) {
	item, err := FindByPkWithContext[T](ctx, id, columns...)
	if err != nil {
		return nil, err
	}
	if item == nil {
		var zero T
		return nil, fmt.Errorf("record not found in %s with id %v", zero.TableName(), id)
	}
	return item, nil
}

// FindBy retrieves a single record matching specific column and value
func FindBy[T Model](column string, val interface{}) (*T, error) {
	var item T
	err := query.New(item.TableName()).Where(column, val).First(&item)
	if err != nil {
		return nil, err
	}
	if hook, ok := any(&item).(AfterLoader); ok {
		hook.AfterLoad()
	}
	return &item, nil
}

// All retrieves all records for the model
func All[T Model](columns ...string) ([]T, error) {
	return AllWithContext[T](context.TODO(), columns...)
}

// AllWithContext retrieves all records for the model with context
func AllWithContext[T Model](ctx context.Context, columns ...string) ([]T, error) {
	var zero T
	var records []T
	q := query.New(zero.TableName())
	if ctx != nil {
		q.WithContext(ctx)
	}
	if len(columns) > 0 {
		q.Select(columns...)
	}
	if ord, ok := any(zero).(DefaultOrderer); ok {
		if d := ord.DefaultOrder(); d != "" {
			parts := strings.Fields(d)
			if len(parts) >= 2 {
				q.OrderBy(parts[0], parts[1])
			} else {
				q.OrderBy(parts[0], "ASC")
			}
		}
	}
	err := q.All(&records)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if hook, ok := any(&records[i]).(AfterLoader); ok {
			hook.AfterLoad()
		}
	}
	return records, nil
}

// Get retrieves all records with eager-loading if configured
func Get[T Model](columns ...string) ([]T, error) {
	return All[T](columns...)
}

// GetWithContext retrieves all records with context
func GetWithContext[T Model](ctx context.Context, columns ...string) ([]T, error) {
	return AllWithContext[T](ctx, columns...)
}

// Where starts a query builder for model
func Where[T Model](column string, args ...interface{}) *query.Query {
	var zero T
	return query.New(zero.TableName()).Where(column, args...)
}

// FilterWhere starts a query builder and applies non-empty condition map
func FilterWhere[T Model](conditions map[string]interface{}) *query.Query {
	var zero T
	q := query.New(zero.TableName())
	for col, val := range conditions {
		q.FilterWhere(col, val)
	}
	return q
}

// Count returns the number of records matching conditions
func Count[T Model](conditions ...map[string]interface{}) (int64, error) {
	var zero T
	q := query.New(zero.TableName())
	if len(conditions) > 0 && conditions[0] != nil {
		for col, val := range conditions[0] {
			q.Where(col, val)
		}
	}
	return q.Count()
}

// Paginate executes query pagination for model
func Paginate[T Model](opts query.Options, searchColumns ...string) (response.Pagination, []T, error) {
	return PaginateWithContext[T](context.TODO(), opts, searchColumns...)
}

// PaginateWithContext executes query pagination for model with context
func PaginateWithContext[T Model](ctx context.Context, opts query.Options, searchColumns ...string) (response.Pagination, []T, error) {
	var zero T
	var records []T
	q := query.New(zero.TableName())
	if ctx != nil {
		q.WithContext(ctx)
	}

	meta, err := q.Paginate(opts, searchColumns, &records)
	if err != nil {
		return response.Pagination{}, nil, err
	}
	for i := range records {
		if hook, ok := any(&records[i]).(AfterLoader); ok {
			hook.AfterLoad()
		}
	}
	return meta, records, nil
}

// PaginateWithConditions paginates with custom conditions and ordering
func PaginateWithConditions[T Model](page int, perPage int, conditions map[string]interface{}, orderBy ...string) (response.Pagination, []T, error) {
	var zero T
	var records []T
	q := query.New(zero.TableName())

	for col, val := range conditions {
		q.Where(col, val)
	}

	if len(orderBy) > 0 && orderBy[0] != "" {
		parts := strings.Fields(orderBy[0])
		if len(parts) >= 2 {
			q.OrderBy(parts[0], parts[1])
		} else {
			q.OrderBy(parts[0], "ASC")
		}
	}

	opts := query.Options{
		Page:    page,
		PerPage: perPage,
	}
	meta, err := q.Paginate(opts, nil, &records)
	if err != nil {
		return response.Pagination{}, nil, err
	}
	return meta, records, nil
}

// Create creates a new record from map data and returns the model
func Create[T Model](data map[string]interface{}) (*T, error) {
	var item T
	val := reflect.ValueOf(&item).Elem()
	setStructFieldsFromMap(val, data)

	if err := Save(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

// InsertRecord inserts record data and returns the inserted ID
func InsertRecord[T Model](data map[string]interface{}) (interface{}, error) {
	item, err := Create[T](data)
	if err != nil {
		return nil, err
	}
	val := reflect.ValueOf(item).Elem()
	fieldMap := extractFieldMap(val)
	return fieldMap["id"], nil
}

// Update updates an existing record by ID with map data
func Update[T Model](id interface{}, data map[string]interface{}) error {
	item, err := FindByPk[T](id)
	if err != nil {
		return err
	}
	if item == nil {
		var zero T
		return fmt.Errorf("record not found in %s", zero.TableName())
	}

	val := reflect.ValueOf(item).Elem()
	setStructFieldsFromMap(val, data)

	return Save(item)
}

// UpdateRecord is an alias for Update
func UpdateRecord[T Model](id interface{}, data map[string]interface{}) error {
	return Update[T](id, data)
}

// UpdateAll updates multiple records matching conditions
func UpdateAll[T Model](data map[string]interface{}, conditions map[string]interface{}) (int64, error) {
	var zero T
	q := query.New(zero.TableName())
	for col, val := range conditions {
		q.Where(col, val)
	}
	return q.Update(data)
}

// Delete removes record by primary key
func Delete[T Model](id interface{}) error {
	item, err := FindByPk[T](id)
	if err != nil {
		return err
	}
	if item == nil {
		var zero T
		return fmt.Errorf("record not found in %s", zero.TableName())
	}
	return DeleteModel(item)
}

// DeleteRecord is alias for Delete
func DeleteRecord[T Model](id interface{}) error {
	return Delete[T](id)
}

// DeleteAll deletes records by condition (ID slice or map conditions)
func DeleteAll[T Model](condition ...interface{}) (int64, error) {
	var zero T
	q := query.New(zero.TableName())

	if len(condition) > 0 && condition[0] != nil {
		cond := condition[0]
		rv := reflect.ValueOf(cond)

		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			pkName := GetPrimaryKeyName(zero)
			var ids []interface{}
			for i := 0; i < rv.Len(); i++ {
				ids = append(ids, rv.Index(i).Interface())
			}
			q.WhereIn(pkName, ids...)
		} else if condMap, ok := cond.(map[string]interface{}); ok {
			for col, val := range condMap {
				q.Where(col, val)
			}
		}
	}
	return q.Delete()
}

func resolveTableName(m any, val reflect.Value) string {
	if mdl, ok := m.(Model); ok {
		return mdl.TableName()
	}
	if val.IsValid() {
		typ := val.Type()
		if mMethod, ok := typ.MethodByName("TableName"); ok {
			res := mMethod.Func.Call([]reflect.Value{val})
			if len(res) > 0 {
				return res[0].String()
			}
		}
		if val.CanAddr() {
			if ptrMethod, ok := reflect.PtrTo(typ).MethodByName("TableName"); ok {
				res := ptrMethod.Func.Call([]reflect.Value{val.Addr()})
				if len(res) > 0 {
					return res[0].String()
				}
			}
		}
	}
	return ""
}

func isNilOrZero(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return true
		}
		return isNilOrZero(rv.Elem().Interface())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.String:
		return rv.String() == ""
	}
	if t, ok := v.(time.Time); ok {
		return t.IsZero()
	}
	return false
}

// Save inserts or updates a model instance automatically with Lifecycle Hooks, Timestamps, and Audit Authors (created_by/updated_by)
func Save(m any, contexts ...context.Context) error {
	db := database.GetDB()
	driver := database.GetDriver()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	tableName := resolveTableName(m, val)
	if tableName == "" {
		return fmt.Errorf("model must implement TableName() string")
	}

	fieldValues := extractFieldMap(val)
	pkName := GetPrimaryKeyName(m)
	idVal, hasID := fieldValues[pkName]
	isUpdate := false

	if hasID && idVal != nil {
		if idNum, ok := idVal.(uint); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(uint64); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(int); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(int64); ok && idNum > 0 {
			isUpdate = true
		} else if str, ok := idVal.(string); ok && str != "" {
			isUpdate = true
		}
	}

	// Extract authenticated User ID from context if provided
	var userID uint = 0
	if len(contexts) > 0 && contexts[0] != nil {
		ctx := contexts[0]
		if id, ok := ctx.Value(middleware.UserIDContextKey).(uint); ok && id > 0 {
			userID = id
		} else if claims, ok := ctx.Value(middleware.UserContextKey).(*auth.JWTClaims); ok && claims != nil && claims.UserID > 0 {
			userID = claims.UserID
		} else if id, ok := ctx.Value("padi_user_id").(uint); ok && id > 0 {
			userID = id
		} else if id, ok := ctx.Value("user_id").(uint); ok && id > 0 {
			userID = id
		} else if id, ok := ctx.Value("user_id").(int); ok && id > 0 {
			userID = uint(id)
		} else if id, ok := ctx.Value("user_id").(int64); ok && id > 0 {
			userID = uint(id)
		}
	}

	// 1. BeforeSave Hook
	if hook, ok := m.(BeforeSaver); ok {
		if err := hook.BeforeSave(!isUpdate); err != nil {
			return err
		}
		fieldValues = extractFieldMap(val)
	}

	now := time.Now().UTC()

	// Apply Audits
	useAudit := true
	createdAtCol := "created_at"
	updatedAtCol := "updated_at"
	createdByCol := "created_by"
	updatedByCol := "updated_by"

	if aud, ok := m.(Auditable); ok {
		useAudit = aud.UseAudit()
		if customFields := aud.AuditFields(); len(customFields) > 0 {
			if col, ok := customFields["created_at"]; ok {
				createdAtCol = col
			}
			if col, ok := customFields["updated_at"]; ok {
				updatedAtCol = col
			}
			if col, ok := customFields["created_by"]; ok {
				createdByCol = col
			}
			if col, ok := customFields["updated_by"]; ok {
				updatedByCol = col
			}
		}
	}

	// Fetch table columns once — used for both field filtering and audit column detection
	tableCols, _ := GetTableColumns(tableName)
	var colMap map[string]bool
	if len(tableCols) > 0 {
		colMap = make(map[string]bool, len(tableCols))
		for _, tc := range tableCols {
			colMap[strings.ToLower(tc)] = true
		}
		// Strip struct fields not present in the actual table schema
		for k := range fieldValues {
			if !colMap[strings.ToLower(k)] {
				delete(fieldValues, k)
			}
		}
	}

	// applyAuditField is a helper that applies an audit value if the column exists in the table
	applyAuditField := func(fieldKey string, value interface{}) {
		k := strings.ToLower(fieldKey)
		if colMap == nil || colMap[k] {
			fieldValues[k] = value
			setStructFieldsFromMap(val, map[string]interface{}{fieldKey: value})
		}
	}
	hasCol := func(fieldKey string) bool {
		return colMap == nil || colMap[strings.ToLower(fieldKey)]
	}

	if isUpdate {
		// UPDATE
		delete(fieldValues, pkName)
		if createdAtCol != "" {
			delete(fieldValues, strings.ToLower(createdAtCol))
		}
		if createdByCol != "" {
			delete(fieldValues, strings.ToLower(createdByCol))
		}
		if useAudit {
			if updatedAtCol != "" && hasCol(updatedAtCol) {
				applyAuditField(updatedAtCol, now)
			}
			if updatedByCol != "" && hasCol(updatedByCol) {
				if userID > 0 {
					applyAuditField(updatedByCol, userID)
				} else if isNilOrZero(fieldValues[strings.ToLower(updatedByCol)]) {
					delete(fieldValues, strings.ToLower(updatedByCol))
				}
			}
		}

		var setClauses []string
		var args []interface{}
		idx := 1

		for col, v := range fieldValues {
			ph := "?"
			if driver == "postgres" {
				ph = fmt.Sprintf("$%d", idx)
			}
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", col, ph))
			args = append(args, v)
			idx++
		}

		idPlaceholder := "?"
		if driver == "postgres" {
			idPlaceholder = fmt.Sprintf("$%d", idx)
		}
		args = append(args, idVal)

		sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s", tableName, strings.Join(setClauses, ", "), pkName, idPlaceholder)
		start := time.Now()
		_, err := db.Exec(sqlStr, args...)
		if len(contexts) > 0 && contexts[0] != nil {
			database.TrackQuery(contexts[0], sqlStr, args, time.Since(start))
		}
		if err != nil {
			return err
		}

		// 2. AfterSave Hook
		if hook, ok := m.(AfterSaver); ok {
			hook.AfterSave(false)
		}
		return nil
	}

	// INSERT
	delete(fieldValues, pkName)
	if useAudit {
		if createdAtCol != "" && hasCol(createdAtCol) && isNilOrZero(fieldValues[strings.ToLower(createdAtCol)]) {
			applyAuditField(createdAtCol, now)
		}
		if updatedAtCol != "" && hasCol(updatedAtCol) && isNilOrZero(fieldValues[strings.ToLower(updatedAtCol)]) {
			applyAuditField(updatedAtCol, now)
		}
		if createdByCol != "" && hasCol(createdByCol) {
			if userID > 0 {
				if isNilOrZero(fieldValues[strings.ToLower(createdByCol)]) {
					applyAuditField(createdByCol, userID)
				}
			} else if isNilOrZero(fieldValues[strings.ToLower(createdByCol)]) {
				fieldValues[strings.ToLower(createdByCol)] = nil
			}
		}
		if updatedByCol != "" && hasCol(updatedByCol) {
			if userID > 0 {
				if isNilOrZero(fieldValues[strings.ToLower(updatedByCol)]) {
					applyAuditField(updatedByCol, userID)
				}
			} else if isNilOrZero(fieldValues[strings.ToLower(updatedByCol)]) {
				fieldValues[strings.ToLower(updatedByCol)] = nil
			}
		}
	}

	var columns []string
	var placeholders []string
	var args []interface{}
	idx := 1

	for col, v := range fieldValues {
		columns = append(columns, col)
		ph := "?"
		if driver == "postgres" {
			ph = fmt.Sprintf("$%d", idx)
		}
		placeholders = append(placeholders, ph)
		args = append(args, v)
		idx++
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	if driver == "postgres" {
		sqlStr += fmt.Sprintf(" RETURNING %s", pkName)
		start := time.Now()
		var newID int64
		err := db.QueryRow(sqlStr, args...).Scan(&newID)
		if len(contexts) > 0 && contexts[0] != nil {
			database.TrackQuery(contexts[0], sqlStr, args, time.Since(start))
		}
		if err == nil {
			setStructID(val, newID)
			if hook, ok := m.(AfterSaver); ok {
				hook.AfterSave(true)
			}
		}
		return err
	}

	start := time.Now()
	res, err := db.Exec(sqlStr, args...)
	if len(contexts) > 0 && contexts[0] != nil {
		database.TrackQuery(contexts[0], sqlStr, args, time.Since(start))
	}
	if err != nil {
		return err
	}

	lastID, err := res.LastInsertId()
	if err == nil && lastID > 0 {
		setStructID(val, lastID)
	}

	// 2. AfterSave Hook
	if hook, ok := m.(AfterSaver); ok {
		hook.AfterSave(true)
	}

	return nil
}

// DeleteModel removes the record from database with BeforeDelete & AfterDelete hooks
func DeleteModel(m any, contexts ...context.Context) error {
	db := database.GetDB()
	driver := database.GetDriver()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	tableName := resolveTableName(m, val)
	if tableName == "" {
		return fmt.Errorf("model must implement TableName() string")
	}
	pkName := GetPrimaryKeyName(m)

	if hook, ok := m.(BeforeDeleter); ok {
		if err := hook.BeforeDelete(); err != nil {
			return err
		}
	}

	fieldValues := extractFieldMap(val)
	idVal, hasID := fieldValues[pkName]
	if !hasID || idVal == nil {
		return fmt.Errorf("cannot delete model without primary key %s", pkName)
	}

	ph := "?"
	if driver == "postgres" {
		ph = "$1"
	}

	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s = %s", tableName, pkName, ph)
	start := time.Now()
	_, err := db.Exec(sqlStr, idVal)
	if len(contexts) > 0 && contexts[0] != nil {
		database.TrackQuery(contexts[0], sqlStr, []interface{}{idVal}, time.Since(start))
	}
	if err != nil {
		return err
	}

	if hook, ok := m.(AfterDeleter); ok {
		hook.AfterDelete()
	}

	return nil
}

// SoftDelete sets deleted_at timestamp if table supports soft delete
func SoftDelete(m any, contexts ...context.Context) error {
	db := database.GetDB()
	driver := database.GetDriver()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	tableName := resolveTableName(m, val)
	if tableName == "" {
		return fmt.Errorf("model must implement TableName() string")
	}
	pkName := GetPrimaryKeyName(m)

	if hook, ok := m.(BeforeDeleter); ok {
		if err := hook.BeforeDelete(); err != nil {
			return err
		}
	}

	fieldValues := extractFieldMap(val)
	idVal, hasID := fieldValues[pkName]
	if !hasID || idVal == nil {
		return fmt.Errorf("cannot soft delete model without primary key %s", pkName)
	}

	now := time.Now().UTC()
	ph1 := "?"
	ph2 := "?"
	if driver == "postgres" {
		ph1 = "$1"
		ph2 = "$2"
	}

	sqlStr := fmt.Sprintf("UPDATE %s SET deleted_at = %s WHERE %s = %s", tableName, ph1, pkName, ph2)
	start := time.Now()
	_, err := db.Exec(sqlStr, now, idVal)
	if len(contexts) > 0 && contexts[0] != nil {
		database.TrackQuery(contexts[0], sqlStr, []interface{}{now, idVal}, time.Since(start))
	}
	if err != nil {
		return err
	}

	if hook, ok := m.(AfterDeleter); ok {
		hook.AfterDelete()
	}

	return nil
}

// SoftDeleteByID soft deletes by ID
func SoftDeleteByID[T Model](id interface{}) error {
	item, err := FindByPk[T](id)
	if err != nil {
		return err
	}
	if item == nil {
		var zero T
		return fmt.Errorf("record not found in %s", zero.TableName())
	}
	return SoftDelete(item)
}

// BatchInsert inserts multiple struct records using multi-row bulk INSERT for high performance.
// Bulk INSERT batches up to chunkSize rows per statement (default 500).
// Falls back to individual Save() per item if table schema cannot be introspected.
func BatchInsert[T Model](items []T, chunkSize ...int) error {
	if len(items) == 0 {
		return nil
	}

	size := 500
	if len(chunkSize) > 0 && chunkSize[0] > 0 {
		size = chunkSize[0]
	}

	db := database.GetDB()
	driver := database.GetDriver()

	var zero T
	tableName := zero.TableName()

	// Introspect columns once
	tableCols, _ := GetTableColumns(tableName)
	if len(tableCols) == 0 {
		// Fallback: row-by-row Save
		for i := 0; i < len(items); i++ {
			if err := Save(&items[i]); err != nil {
				return err
			}
		}
		return nil
	}

	// Build allowed column set (excluding pk "id")
	colSet := make(map[string]bool, len(tableCols))
	var orderedCols []string
	for _, c := range tableCols {
		lc := strings.ToLower(c)
		if lc == "id" {
			continue
		}
		if !colSet[lc] {
			colSet[lc] = true
			orderedCols = append(orderedCols, c)
		}
	}
	if len(orderedCols) == 0 {
		return fmt.Errorf("BatchInsert: no writable columns found for table %s", tableName)
	}

	now := time.Now().UTC()

	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		var valuePlaceholders []string
		var args []interface{}
		idx := 1

		for _, item := range chunk {
			rv := reflect.ValueOf(item)
			if rv.Kind() == reflect.Ptr {
				rv = rv.Elem()
			}
			fv := extractFieldMap(rv)

			// Apply audit defaults
			for _, tc := range []string{"created_at", "updated_at"} {
				if colSet[tc] && isNilOrZero(fv[tc]) {
					fv[tc] = now
				}
			}

			rowPhs := make([]string, len(orderedCols))
			for j, col := range orderedCols {
				if driver == "postgres" {
					rowPhs[j] = fmt.Sprintf("$%d", idx)
				} else {
					rowPhs[j] = "?"
				}
				args = append(args, fv[strings.ToLower(col)])
				idx++
			}
			valuePlaceholders = append(valuePlaceholders, "("+strings.Join(rowPhs, ", ")+")")
		}

		colNames := make([]string, len(orderedCols))
		copy(colNames, orderedCols)

		sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
			tableName,
			strings.Join(colNames, ", "),
			strings.Join(valuePlaceholders, ", "),
		)

		if _, err := db.Exec(sqlStr, args...); err != nil {
			return err
		}
	}
	return nil
}

// Upsert performs insert or update on duplicate key
func Upsert[T Model](data map[string]interface{}, updateColumns ...string) (int64, error) {
	var zero T
	db := database.GetDB()
	driver := database.GetDriver()
	tableName := zero.TableName()

	var cols []string
	var placeholders []string
	var args []interface{}
	idx := 1

	for k, v := range data {
		cols = append(cols, k)
		ph := "?"
		if driver == "postgres" {
			ph = fmt.Sprintf("$%d", idx)
		}
		placeholders = append(placeholders, ph)
		args = append(args, v)
		idx++
	}

	var updateCols []string
	if len(updateColumns) > 0 {
		updateCols = updateColumns
	} else {
		updateCols = cols
	}

	var sqlStr string
	if driver == "mysql" {
		var updates []string
		for _, col := range updateCols {
			updates = append(updates, fmt.Sprintf("`%s` = VALUES(`%s`)", col, col))
		}
		sqlStr = fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
			tableName, strings.Join(cols, ", "), strings.Join(placeholders, ", "), strings.Join(updates, ", "))
	} else if driver == "postgres" {
		pkName := GetPrimaryKeyName(zero)
		var updates []string
		for _, col := range updateCols {
			updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}
		sqlStr = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
			tableName, strings.Join(cols, ", "), strings.Join(placeholders, ", "), pkName, strings.Join(updates, ", "))
	} else {
		// SQLite
		pkName := GetPrimaryKeyName(zero)
		var updates []string
		for _, col := range updateCols {
			updates = append(updates, fmt.Sprintf("%s = excluded.%s", col, col))
		}
		sqlStr = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s",
			tableName, strings.Join(cols, ", "), strings.Join(placeholders, ", "), pkName, strings.Join(updates, ", "))
	}

	res, err := db.Exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Query executes raw SQL query into mapped struct slice.
// rows.Columns() is called once before the iteration loop for efficiency.
func Query[T any](sqlStr string, args ...interface{}) ([]T, error) {
	db := database.GetDB()
	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Fetch column names once — not per row
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []T
	var fieldMap map[string]reflect.Value // lazily built once for struct types

	for rows.Next() {
		var item T
		values := make([]interface{}, len(cols))
		for i := range values {
			var v interface{}
			values[i] = &v
		}
		if err := rows.Scan(values...); err != nil {
			return nil, err
		}
		val := reflect.ValueOf(&item)
		if val.Elem().Kind() == reflect.Struct {
			if fieldMap == nil {
				fieldMap = make(map[string]reflect.Value)
				mapStructFields(val.Elem(), fieldMap)
			}
			for i, col := range cols {
				if f, ok := fieldMap[strings.ToLower(col)]; ok && f.CanSet() {
					raw := reflect.ValueOf(values[i]).Elem().Interface()
					assignField(f, raw)
				}
			}
		}
		records = append(records, item)
	}
	return records, rows.Err()
}

// GetTableColumns inspects table column names cached with TTL
func GetTableColumns(tableName string) ([]string, error) {
	columnsCacheMu.RLock()
	if entry, ok := columnsCache[tableName]; ok {
		if time.Since(entry.cachedAt) < columnsCacheTTL {
			columnsCacheMu.RUnlock()
			return entry.columns, nil
		}
	}
	columnsCacheMu.RUnlock()

	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	driver := database.GetDriver()
	var columns []string

	if driver == "sqlite" {
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cid, notnull, pk int
				var name, typeStr string
				var dflt *string
				if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dflt, &pk); err == nil {
					columns = append(columns, name)
				}
			}
			_ = rows.Err()
		}
	} else if driver == "mysql" {
		rows, err := db.Query(fmt.Sprintf("DESCRIBE `%s`", tableName))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var field, typeStr, null, key, extra string
				var def *string
				if err := rows.Scan(&field, &typeStr, &null, &key, &def, &extra); err == nil {
					columns = append(columns, field)
				}
			}
			_ = rows.Err()
		}
	} else {
		// Postgres
		rows, err := db.Query(`
			SELECT column_name 
			FROM information_schema.columns 
			WHERE table_name = $1 
			ORDER BY ordinal_position`, tableName)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var colName string
				if err := rows.Scan(&colName); err == nil {
					columns = append(columns, colName)
				}
			}
			_ = rows.Err()
		}
	}

	columnsCacheMu.Lock()
	columnsCache[tableName] = columnCacheEntry{columns: columns, cachedAt: time.Now()}
	columnsCacheMu.Unlock()

	return columns, nil
}

// FilterFillable filters map data to only allowed fillable fields if defined
func FilterFillable(m Model, data map[string]interface{}) map[string]interface{} {
	if f, ok := m.(Fillabler); ok {
		fillableList := f.Fillable()
		if len(fillableList) > 0 {
			res := make(map[string]interface{})
			for _, key := range fillableList {
				if val, exists := data[key]; exists {
					res[key] = val
				}
			}
			return res
		}
	}
	return data
}

// HideFields is a placeholder for hiding sensitive fields during array transformations
func HideFields(items interface{}) {}

// Map represents an insertion-ordered key-value map for predictable, structured JSON serialization
type Map struct {
	keys   []string
	values map[string]any
}

// NewMap creates an empty ordered Map
func NewMap() *Map {
	return &Map{
		keys:   make([]string, 0, 16),
		values: make(map[string]any, 16),
	}
}

// Set sets a key-value pair preserving insertion order
func (m *Map) Set(key string, val any) *Map {
	if m == nil {
		return m
	}
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = val
	return m
}

// Get retrieves a value by key
func (m *Map) Get(key string) (any, bool) {
	if m == nil || m.values == nil {
		return nil, false
	}
	val, ok := m.values[key]
	return val, ok
}

// Keys returns all keys in insertion order
func (m *Map) Keys() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

// ToMap converts the ordered Map to a standard Go map[string]any
func (m *Map) ToMap() map[string]any {
	if m == nil {
		return nil
	}
	return m.values
}

// MarshalJSON implements json.Marshaler ensuring exact insertion order in JSON output
func (m *Map) MarshalJSON() ([]byte, error) {
	if m == nil || len(m.keys) == 0 {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kB, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kB)
		buf.WriteByte(':')
		vB, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vB)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

var (
	relationCache   = make(map[string]relationCacheEntry)
	relationCacheMu sync.RWMutex
)

// ClearRelationCache resets the relation query cache
func ClearRelationCache() {
	relationCacheMu.Lock()
	defer relationCacheMu.Unlock()
	relationCache = make(map[string]relationCacheEntry)
}

// FindRelation fetches a related record map and its primary display field with TTL-bounded in-memory caching.
// Both positive hits (record found) and negative hits (record not found) are cached with a TTL.
func FindRelation(table string, id any, displayCol ...string) (*Map, any) {
	if isNilOrZero(id) {
		return nil, nil
	}

	targetCol := "name"
	if len(displayCol) > 0 && displayCol[0] != "" {
		targetCol = displayCol[0]
	}

	cacheKey := fmt.Sprintf("%s:%v:%s", table, id, targetCol)

	// Check TTL-aware cache
	relationCacheMu.RLock()
	if entry, ok := relationCache[cacheKey]; ok && time.Since(entry.cachedAt) < relationCacheTTL {
		m := entry.value
		relationCacheMu.RUnlock()
		if m == nil {
			return nil, nil
		}
		val, _ := m.Get(targetCol)
		return m, val
	}
	relationCacheMu.RUnlock()

	db := database.GetDB()
	if db == nil {
		return nil, nil
	}
	driver := database.GetDriver()

	// Build candidate tables: provided, singular, plural, and compound fallbacks (e.g. graduation_semesters -> semesters)
	tableSet := make(map[string]bool)
	var tablesToTry []string
	addTableCandidate := func(t string) {
		if t != "" && !tableSet[t] {
			tableSet[t] = true
			tablesToTry = append(tablesToTry, t)
		}
	}

	addTableCandidate(table)
	if strings.HasSuffix(table, "s") {
		addTableCandidate(strings.TrimSuffix(table, "s"))
	} else {
		addTableCandidate(table + "s")
	}

	// If table contains underscore (e.g. "graduation_semesters" or "graduation_semester"), also try last word (e.g. "semesters", "semester")
	if strings.Contains(table, "_") {
		parts := strings.Split(table, "_")
		lastWord := parts[len(parts)-1]
		addTableCandidate(lastWord)
		if strings.HasSuffix(lastWord, "s") {
			addTableCandidate(strings.TrimSuffix(lastWord, "s"))
		} else {
			addTableCandidate(lastWord + "s")
		}
	}

	for _, tbl := range tablesToTry {
		cols, err := GetTableColumns(tbl)
		if err != nil || len(cols) == 0 {
			continue
		}

		colMap := make(map[string]bool, len(cols))
		for _, c := range cols {
			colMap[strings.ToLower(c)] = true
		}

		selectedCols := []string{"id"}
		if colMap[strings.ToLower(targetCol)] {
			selectedCols = append(selectedCols, targetCol)
		} else {
			// Find suitable fallback display column
			candidates := []string{"username", "name", "nama", "title", "judul", "email", "code"}
			for _, cand := range candidates {
				if colMap[cand] {
					selectedCols = append(selectedCols, cand)
					targetCol = cand
					break
				}
			}
		}

		ph := "?"
		if driver == "postgres" {
			ph = "$1"
		}

		querySQL := fmt.Sprintf("SELECT %s FROM %s WHERE id = %s LIMIT 1", strings.Join(selectedCols, ", "), tbl, ph)
		rows, err := db.Query(querySQL, id)
		if err != nil {
			continue
		}

		rowCols, _ := rows.Columns()
		if !rows.Next() {
			rows.Close()
			continue
		}

		values := make([]interface{}, len(rowCols))
		scanArgs := make([]interface{}, len(rowCols))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			rows.Close()
			continue
		}
		rows.Close()

		res := NewMap()
		for i, c := range rowCols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				res.Set(c, string(b))
			} else {
				res.Set(c, v)
			}
		}

		relationCacheMu.Lock()
		relationCache[cacheKey] = relationCacheEntry{value: res, cachedAt: time.Now()}
		relationCacheMu.Unlock()

		val, _ := res.Get(targetCol)
		return res, val
	}

	// Cache negative lookup with TTL so non-existent relations are not queried repeatedly
	relationCacheMu.Lock()
	relationCache[cacheKey] = relationCacheEntry{value: nil, cachedAt: time.Now()}
	relationCacheMu.Unlock()

	return nil, nil
}

// LoadRelations loads defined relations for a collection of models
func LoadRelations(items interface{}, relations ...string) error {
	return nil
}

// ActiveRecord provides Base struct embedding for models
type ActiveRecord struct{}

func extractFieldMap(val reflect.Value) map[string]interface{} {
	fields := make(map[string]interface{})
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		sf := typ.Field(i)

		if sf.Anonymous && f.Kind() == reflect.Struct {
			subMap := extractFieldMap(f)
			for k, v := range subMap {
				fields[k] = v
			}
			continue
		}

		tag := sf.Tag.Get("db")
		if tag == "" {
			tag = sf.Tag.Get("json")
		}
		if tag == "-" {
			continue
		}

		colName := sf.Name
		if tag != "" {
			colName = strings.Split(tag, ",")[0]
		}
		fields[strings.ToLower(colName)] = f.Interface()
	}

	return fields
}

func setStructFieldsFromMap(val reflect.Value, data map[string]interface{}) {
	fieldMap := make(map[string]reflect.Value)
	mapStructFields(val, fieldMap)

	for k, v := range data {
		if f, exists := fieldMap[strings.ToLower(k)]; exists && f.CanSet() {
			assignField(f, v)
		}
	}
}

func setStructID(val reflect.Value, id int64) {
	if !val.CanSet() {
		return
	}
	idField := val.FieldByName("ID")
	if idField.IsValid() && idField.CanSet() {
		if idField.Kind() == reflect.Uint || idField.Kind() == reflect.Uint64 {
			idField.SetUint(uint64(id))
		} else if idField.Kind() == reflect.Int || idField.Kind() == reflect.Int64 {
			idField.SetInt(id)
		}
	}
}

func mapStructFields(val reflect.Value, fieldMap map[string]reflect.Value) {
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		structField := typ.Field(i)

		if structField.Anonymous && field.Kind() == reflect.Struct {
			mapStructFields(field, fieldMap)
			continue
		}

		tag := structField.Tag.Get("db")
		if tag == "" {
			tag = structField.Tag.Get("json")
		}
		if tag != "" && tag != "-" {
			colName := strings.Split(tag, ",")[0]
			fieldMap[strings.ToLower(colName)] = field
		}
		fieldMap[strings.ToLower(structField.Name)] = field
	}
}

func assignField(field reflect.Value, rawVal interface{}) {
	if rawVal == nil {
		return
	}

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		assignField(field.Elem(), rawVal)
		return
	}

	switch field.Kind() {
	case reflect.String:
		if t, ok := rawVal.(time.Time); ok {
			field.SetString(t.Format(time.RFC3339))
		} else {
			field.SetString(fmt.Sprintf("%v", rawVal))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t, ok := rawVal.(time.Time); ok {
			field.SetInt(t.Unix())
		} else if n, ok := rawVal.(int64); ok {
			field.SetInt(n)
		} else if s, err := strconv.ParseInt(fmt.Sprintf("%v", rawVal), 10, 64); err == nil {
			field.SetInt(s)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if t, ok := rawVal.(time.Time); ok {
			field.SetUint(uint64(t.Unix()))
		} else if n, ok := rawVal.(int64); ok && n >= 0 {
			field.SetUint(uint64(n))
		} else if s, err := strconv.ParseUint(fmt.Sprintf("%v", rawVal), 10, 64); err == nil {
			field.SetUint(s)
		}
	case reflect.Bool:
		if b, ok := rawVal.(bool); ok {
			field.SetBool(b)
		} else if n, ok := rawVal.(int64); ok {
			field.SetBool(n == 1)
		}
	case reflect.Float32, reflect.Float64:
		if f, ok := rawVal.(float64); ok {
			field.SetFloat(f)
		}
	default:
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if t, ok := rawVal.(time.Time); ok {
				field.Set(reflect.ValueOf(t))
			} else if pt, ok := rawVal.(*time.Time); ok && pt != nil {
				field.Set(reflect.ValueOf(*pt))
			} else if n, ok := rawVal.(int64); ok {
				field.Set(reflect.ValueOf(time.Unix(n, 0).UTC()))
			} else if str, ok := rawVal.(string); ok {
				if parsed, err := time.Parse(time.RFC3339, str); err == nil {
					field.Set(reflect.ValueOf(parsed))
				} else if parsed, err := time.Parse("2006-01-02 15:04:05", str); err == nil {
					field.Set(reflect.ValueOf(parsed))
				} else if n, err := strconv.ParseInt(str, 10, 64); err == nil && n > 0 {
					field.Set(reflect.ValueOf(time.Unix(n, 0).UTC()))
				}
			}
		}
	}
}
