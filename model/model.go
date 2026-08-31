package model

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/query"
)

// Model is the base interface that every Padi ActiveRecord model must implement
type Model interface {
	TableName() string
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
	HasOne         RelationType = "hasOne"
	HasMany        RelationType = "hasMany"
	BelongsTo      RelationType = "belongsTo"
	BelongsToMany RelationType = "belongsToMany"
)

// Relation definition metadata
type Relation struct {
	Type        RelationType
	ModelName   string
	Table       string
	ForeignKey  string
	LocalKey    string
	OwnerKey    string
	PivotTable  string
	RelatedKey  string
	Columns     []string
}

// Table column cache
var (
	columnsCache   = make(map[string][]string)
	columnsCacheMu sync.RWMutex
)

// HasOne defines a 1-to-1 relationship
func HasOneRel(table string, foreignKey string, localKey ...string) Relation {
	lk := "id"
	if len(localKey) > 0 && localKey[0] != "" {
		lk = localKey[0]
	}
	return Relation{
		Type:       HasOne,
		Table:      table,
		ForeignKey: foreignKey,
		LocalKey:   lk,
	}
}

// HasMany defines a 1-to-many relationship
func HasManyRel(table string, foreignKey string, localKey ...string) Relation {
	lk := "id"
	if len(localKey) > 0 && localKey[0] != "" {
		lk = localKey[0]
	}
	return Relation{
		Type:       HasMany,
		Table:      table,
		ForeignKey: foreignKey,
		LocalKey:   lk,
	}
}

// BelongsTo defines an inverse 1-to-1 or 1-to-many relationship
func BelongsToRel(table string, foreignKey string, ownerKey ...string) Relation {
	ok := "id"
	if len(ownerKey) > 0 && ownerKey[0] != "" {
		ok = ownerKey[0]
	}
	return Relation{
		Type:       BelongsTo,
		Table:      table,
		ForeignKey: ok,
		LocalKey:   foreignKey,
	}
}

// BelongsToMany defines a many-to-many relationship through pivot table
func BelongsToManyRel(table string, pivotTable string, foreignKey string, relatedKey string) Relation {
	return Relation{
		Type:       BelongsToMany,
		Table:      table,
		PivotTable: pivotTable,
		ForeignKey: foreignKey,
		RelatedKey: relatedKey,
		LocalKey:   "id",
	}
}

// ActiveRecord provides Base CRUD helper methods (Find, FindBy, FindOrFail, Save, Delete, Paginate)
type ActiveRecord struct{}

// Find retrieves a single record by primary key (id)
func Find[T Model](id interface{}) (*T, error) {
	var item T
	err := query.New(item.TableName()).Where("id", id).First(&item)
	if err != nil {
		return nil, err
	}
	if hook, ok := any(&item).(AfterLoader); ok {
		hook.AfterLoad()
	}
	return &item, nil
}

// FindOrFail retrieves a single record by primary key (id) or returns an error if not found
func FindOrFail[T Model](id interface{}) (*T, error) {
	item, err := Find[T](id)
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
func All[T Model]() ([]T, error) {
	var zero T
	var records []T
	err := query.New(zero.TableName()).All(&records)
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

// Where starts a query builder for model
func Where[T Model](column string, args ...interface{}) *query.Query {
	var zero T
	return query.New(zero.TableName()).Where(column, args...)
}

// DeleteAll deletes records by condition
func DeleteAll[T Model](column string, val interface{}) (int64, error) {
	var zero T
	db := database.GetDB()
	driver := database.GetDriver()
	ph := "?"
	if driver == "postgres" {
		ph = "$1"
	}
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s = %s", zero.TableName(), column, ph)
	res, err := db.Exec(sqlStr, val)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Save inserts or updates a model instance automatically with Lifecycle Hooks and Timestamps
func Save(m Model) error {
	db := database.GetDB()
	driver := database.GetDriver()
	tableName := m.TableName()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	fieldValues := extractFieldMap(val)
	idVal, hasID := fieldValues["id"]
	isUpdate := false

	if hasID {
		if idNum, ok := idVal.(uint); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(uint64); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(int); ok && idNum > 0 {
			isUpdate = true
		} else if idNum, ok := idVal.(int64); ok && idNum > 0 {
			isUpdate = true
		}
	}

	// 1. BeforeSave Hook
	if hook, ok := m.(BeforeSaver); ok {
		if err := hook.BeforeSave(!isUpdate); err != nil {
			return err
		}
		// Re-extract in case modified in hook
		fieldValues = extractFieldMap(val)
	}

	now := time.Now().UTC()

	if isUpdate {
		// UPDATE
		delete(fieldValues, "id")
		delete(fieldValues, "created_at")
		fieldValues["updated_at"] = now

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

		sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE id = %s", tableName, strings.Join(setClauses, ", "), idPlaceholder)
		_, err := db.Exec(sqlStr, args...)
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
	delete(fieldValues, "id")
	if _, ok := fieldValues["created_at"]; !ok || fieldValues["created_at"] == nil {
		fieldValues["created_at"] = now
	}
	fieldValues["updated_at"] = now

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
		sqlStr += " RETURNING id"
		var newID int64
		err := db.QueryRow(sqlStr, args...).Scan(&newID)
		if err == nil {
			setStructID(val, newID)
			if hook, ok := m.(AfterSaver); ok {
				hook.AfterSave(true)
			}
		}
		return err
	}

	res, err := db.Exec(sqlStr, args...)
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

// Delete removes the record from database with BeforeDelete & AfterDelete hooks
func Delete(m Model) error {
	db := database.GetDB()
	driver := database.GetDriver()
	tableName := m.TableName()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 1. BeforeDelete Hook
	if hook, ok := m.(BeforeDeleter); ok {
		if err := hook.BeforeDelete(); err != nil {
			return err
		}
	}

	fieldValues := extractFieldMap(val)
	idVal, hasID := fieldValues["id"]
	if !hasID {
		return fmt.Errorf("cannot delete model without primary key id")
	}

	ph := "?"
	if driver == "postgres" {
		ph = "$1"
	}

	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE id = %s", tableName, ph)
	_, err := db.Exec(sqlStr, idVal)
	if err != nil {
		return err
	}

	// 2. AfterDelete Hook
	if hook, ok := m.(AfterDeleter); ok {
		hook.AfterDelete()
	}

	return nil
}

// SoftDelete sets deleted_at timestamp if table supports soft delete
func SoftDelete(m Model) error {
	db := database.GetDB()
	driver := database.GetDriver()
	tableName := m.TableName()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if hook, ok := m.(BeforeDeleter); ok {
		if err := hook.BeforeDelete(); err != nil {
			return err
		}
	}

	fieldValues := extractFieldMap(val)
	idVal, hasID := fieldValues["id"]
	if !hasID {
		return fmt.Errorf("cannot soft delete model without primary key id")
	}

	now := time.Now().UTC()
	ph1 := "?"
	ph2 := "?"
	if driver == "postgres" {
		ph1 = "$1"
		ph2 = "$2"
	}

	sqlStr := fmt.Sprintf("UPDATE %s SET deleted_at = %s WHERE id = %s", tableName, ph1, ph2)
	_, err := db.Exec(sqlStr, now, idVal)
	if err != nil {
		return err
	}

	if hook, ok := m.(AfterDeleter); ok {
		hook.AfterDelete()
	}

	return nil
}

// BatchInsert inserts multiple records in chunks
func BatchInsert[T Model](items []T, chunkSize ...int) error {
	if len(items) == 0 {
		return nil
	}

	size := 500
	if len(chunkSize) > 0 && chunkSize[0] > 0 {
		size = chunkSize[0]
	}

	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		for _, item := range chunk {
			if err := Save(item); err != nil {
				return err
			}
		}
	}
	return nil
}

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

// ClearColumnsCache resets cached table columns
func ClearColumnsCache() {
	columnsCacheMu.Lock()
	defer columnsCacheMu.Unlock()
	columnsCache = make(map[string][]string)
}
