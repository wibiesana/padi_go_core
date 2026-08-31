package model

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/query"
)

// Model interface for all Padi ActiveRecord models
type Model interface {
	TableName() string
}

// ActiveRecord provides Base CRUD helper methods (Find, Save, Delete)
type ActiveRecord struct{}

// Find retrieves a single record by primary key (id)
func Find(tableName string, id interface{}, dest Model) error {
	return query.New(tableName).Where("id", id).First(dest)
}

// FindBy retrieves a single record matching conditions
func FindBy(tableName string, column string, val interface{}, dest Model) error {
	return query.New(tableName).Where(column, val).First(dest)
}

// Save inserts or updates a model instance automatically
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
		return err
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

	return nil
}

// Delete removes the record from database
func Delete(m Model) error {
	db := database.GetDB()
	driver := database.GetDriver()
	tableName := m.TableName()

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
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
	return err
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
