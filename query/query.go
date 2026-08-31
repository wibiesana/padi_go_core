package query

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/response"
	"github.com/wibiesana/padi-core/router"
)

// Options pagination and query parameters
type Options struct {
	Page    int
	PerPage int
	Sort    string
	Order   string
	Search  string
}

// ParseOptions extracts standard pagination query params
func ParseOptions(r *http.Request) Options {
	page := router.QueryParamInt(r, "page", 1)
	if page < 1 {
		page = 1
	}

	perPage := router.QueryParamInt(r, "per_page", 15)
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 100 {
		perPage = 100
	}

	sort := router.QueryParam(r, "sort", "id")
	order := strings.ToUpper(router.QueryParam(r, "order", "DESC"))
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	search := router.QueryParam(r, "search", "")

	return Options{
		Page:    page,
		PerPage: perPage,
		Sort:    sort,
		Order:   order,
		Search:  search,
	}
}

// Query is the fluent SQL Query Builder (Zero-bloat & Native)
type Query struct {
	db        *sql.DB
	driver    string
	table     string
	selects   []string
	wheres    []string
	args      []interface{}
	joins     []string
	orderBys  []string
	groupBys  []string
	limitVal  int
	offsetVal int
}

// New creates a new fluent Query Builder for a table
func New(tableName string, db ...*sql.DB) *Query {
	activeDB := database.GetDB()
	if len(db) > 0 && db[0] != nil {
		activeDB = db[0]
	}

	return &Query{
		db:       activeDB,
		driver:   database.GetDriver(),
		table:    tableName,
		selects:  []string{"*"},
		limitVal: -1,
	}
}

// Select specifies columns to select
func (q *Query) Select(columns ...string) *Query {
	if len(columns) > 0 {
		q.selects = columns
	}
	return q
}

// Where adds a standard condition (e.g. Where("status", "=", "active") or Where("role", "admin"))
func (q *Query) Where(column string, args ...interface{}) *Query {
	op := "="
	var val interface{}

	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}

	placeholder := "?"
	if q.driver == "postgres" {
		placeholder = fmt.Sprintf("$%d", len(q.args)+1)
	}

	q.wheres = append(q.wheres, fmt.Sprintf("%s %s %s", column, op, placeholder))
	q.args = append(q.args, val)
	return q
}

// WhereIn adds an IN clause
func (q *Query) WhereIn(column string, values ...interface{}) *Query {
	if len(values) == 0 {
		return q
	}

	var placeholders []string
	for _, v := range values {
		ph := "?"
		if q.driver == "postgres" {
			ph = fmt.Sprintf("$%d", len(q.args)+1)
		}
		placeholders = append(placeholders, ph)
		q.args = append(q.args, v)
	}

	q.wheres = append(q.wheres, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")))
	return q
}

// WhereLike adds a LIKE search condition
func (q *Query) WhereLike(column string, pattern string) *Query {
	likeOp := "LIKE"
	if q.driver == "postgres" {
		likeOp = "ILIKE"
	}

	ph := "?"
	if q.driver == "postgres" {
		ph = fmt.Sprintf("$%d", len(q.args)+1)
	}

	q.wheres = append(q.wheres, fmt.Sprintf("%s %s %s", column, likeOp, ph))
	q.args = append(q.args, pattern)
	return q
}

// Search adds multi-column OR LIKE condition
func (q *Query) Search(keyword string, columns ...string) *Query {
	if keyword == "" || len(columns) == 0 {
		return q
	}

	likeOp := "LIKE"
	if q.driver == "postgres" {
		likeOp = "ILIKE"
	}

	var orClauses []string
	searchVal := "%" + keyword + "%"

	for _, col := range columns {
		ph := "?"
		if q.driver == "postgres" {
			ph = fmt.Sprintf("$%d", len(q.args)+len(orClauses)+1)
		}
		orClauses = append(orClauses, fmt.Sprintf("%s %s %s", col, likeOp, ph))
	}

	for range columns {
		q.args = append(q.args, searchVal)
	}

	q.wheres = append(q.wheres, "("+strings.Join(orClauses, " OR ")+")")
	return q
}

// OrderBy adds order clause
func (q *Query) OrderBy(column string, direction ...string) *Query {
	dir := "ASC"
	if len(direction) > 0 && strings.ToUpper(direction[0]) == "DESC" {
		dir = "DESC"
	}
	q.orderBys = append(q.orderBys, fmt.Sprintf("%s %s", column, dir))
	return q
}

// Limit sets limit
func (q *Query) Limit(limit int) *Query {
	q.limitVal = limit
	return q
}

// Offset sets offset
func (q *Query) Offset(offset int) *Query {
	q.offsetVal = offset
	return q
}

// BuildSQL compiles the current query state into SQL string and arguments
func (q *Query) BuildSQL() (string, []interface{}) {
	var sqlStr strings.Builder
	sqlStr.WriteString(fmt.Sprintf("SELECT %s FROM %s", strings.Join(q.selects, ", "), q.table))

	if len(q.joins) > 0 {
		sqlStr.WriteString(" " + strings.Join(q.joins, " "))
	}

	if len(q.wheres) > 0 {
		sqlStr.WriteString(" WHERE " + strings.Join(q.wheres, " AND "))
	}

	if len(q.groupBys) > 0 {
		sqlStr.WriteString(" GROUP BY " + strings.Join(q.groupBys, ", "))
	}

	if len(q.orderBys) > 0 {
		sqlStr.WriteString(" ORDER BY " + strings.Join(q.orderBys, ", "))
	}

	if q.limitVal >= 0 {
		sqlStr.WriteString(fmt.Sprintf(" LIMIT %d", q.limitVal))
	}

	if q.offsetVal > 0 {
		sqlStr.WriteString(fmt.Sprintf(" OFFSET %d", q.offsetVal))
	}

	return sqlStr.String(), q.args
}

// Count returns total number of matching records
func (q *Query) Count() (int64, error) {
	clone := *q
	clone.selects = []string{"COUNT(*)"}
	clone.limitVal = -1
	clone.offsetVal = 0

	querySQL, args := clone.BuildSQL()
	var total int64
	err := q.db.QueryRow(querySQL, args...).Scan(&total)
	return total, err
}

// First fetches a single row and maps to struct pointer
func (q *Query) First(dest interface{}) error {
	q.Limit(1)
	querySQL, args := q.BuildSQL()

	rows, err := q.db.Query(querySQL, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		return sql.ErrNoRows
	}

	return scanStruct(rows, dest)
}

// All fetches all matching rows and maps into a slice pointer
func (q *Query) All(destSlice interface{}) error {
	querySQL, args := q.BuildSQL()
	rows, err := q.db.Query(querySQL, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	sliceVal := reflect.ValueOf(destSlice)
	if sliceVal.Kind() != reflect.Ptr || sliceVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("destSlice must be a pointer to a slice")
	}

	sliceElem := sliceVal.Elem()
	itemType := sliceElem.Type().Elem()

	for rows.Next() {
		itemPtr := reflect.New(itemType)
		if err := scanStruct(rows, itemPtr.Interface()); err != nil {
			return err
		}
		sliceElem.Set(reflect.Append(sliceElem, itemPtr.Elem()))
	}

	return rows.Err()
}

// Paginate executes query with pagination and returns meta
func (q *Query) Paginate(opts Options, searchColumns []string, destSlice interface{}) (response.Pagination, error) {
	if opts.Search != "" && len(searchColumns) > 0 {
		q.Search(opts.Search, searchColumns...)
	}

	total, err := q.Count()
	if err != nil {
		return response.Pagination{}, err
	}

	if opts.Sort != "" {
		q.OrderBy(opts.Sort, opts.Order)
	}

	offset := (opts.Page - 1) * opts.PerPage
	q.Limit(opts.PerPage).Offset(offset)

	if err := q.All(destSlice); err != nil {
		return response.Pagination{}, err
	}

	lastPage := int(math.Ceil(float64(total) / float64(opts.PerPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	from := offset + 1
	to := offset + opts.PerPage
	if int64(to) > total {
		to = int(total)
	}
	if total == 0 {
		from = 0
		to = 0
	}

	meta := response.Pagination{
		Total:       total,
		PerPage:     opts.PerPage,
		CurrentPage: opts.Page,
		LastPage:    lastPage,
		From:        from,
		To:          to,
	}

	return meta, nil
}

// scanStruct helper maps SQL row columns into struct fields using json or db tags
func scanStruct(rows *sql.Rows, dest interface{}) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to a struct")
	}

	structVal := destVal.Elem()
	fieldMap := make(map[string]reflect.Value)
	mapStructFields(structVal, fieldMap)

	values := make([]interface{}, len(cols))
	for i, col := range cols {
		colLower := strings.ToLower(col)
		if field, exists := fieldMap[colLower]; exists && field.CanSet() {
			values[i] = reflect.New(reflect.PtrTo(field.Type())).Interface()
		} else {
			var unused interface{}
			values[i] = &unused
		}
	}

	if err := rows.Scan(values...); err != nil {
		return err
	}

	for i, col := range cols {
		colLower := strings.ToLower(col)
		if field, exists := fieldMap[colLower]; exists && field.CanSet() {
			valPtr := reflect.ValueOf(values[i]).Elem()
			if !valPtr.IsNil() {
				rawVal := valPtr.Elem().Interface()
				assignField(field, rawVal)
			}
		}
	}

	return nil
}

func mapStructFields(val reflect.Value, fieldMap map[string]reflect.Value) {
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		structField := typ.Field(i)

		// Support embedded structs (e.g. base.User)
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

	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", rawVal))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, ok := rawVal.(int64); ok {
			field.SetInt(n)
		} else if s, err := strconv.ParseInt(fmt.Sprintf("%v", rawVal), 10, 64); err == nil {
			field.SetInt(s)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, ok := rawVal.(int64); ok && n >= 0 {
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
			} else if str, ok := rawVal.(string); ok {
				if parsed, err := time.Parse(time.RFC3339, str); err == nil {
					field.Set(reflect.ValueOf(parsed))
				} else if parsed, err := time.Parse("2006-01-02 15:04:05", str); err == nil {
					field.Set(reflect.ValueOf(parsed))
				}
			}
		}
	}
}
