package query

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/response"
	"github.com/wibiesana/padi-core/router"
)

const Version = "2.1.11"

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

// Query is the fluent SQL Query Builder (Zero-bloat, Native & Parameterized)
type Query struct {
	db        *sql.DB
	driver    string
	table     string
	selects   []string
	distinct  bool
	autoIlike bool
	wheres    []string
	args      []interface{}
	joins     []string
	orderBys  []string
	groupBys  []string
	havings   []string
	havingArgs []interface{}
	limitVal  int
	offsetVal int
	lockVal   string
	unions    []string
}

// New creates a new fluent Query Builder for a table
func New(tableName string, db ...*sql.DB) *Query {
	activeDB := database.GetDB()
	if len(db) > 0 && db[0] != nil {
		activeDB = db[0]
	}

	return &Query{
		db:        activeDB,
		driver:    database.GetDriver(),
		table:     tableName,
		selects:   []string{"*"},
		limitVal:  -1,
		autoIlike: true,
	}
}

// Find starts a new query builder (static constructor matching PHP Query::find)
func Find(connectionName ...string) *Query {
	return New("")
}

// From sets table name to query from
func (q *Query) From(table string) *Query {
	q.table = table
	return q
}

// Table is an alias for From
func (q *Query) Table(table string) *Query {
	return q.From(table)
}

// AutoIlike enables or disables automatic ILIKE conversion for PostgreSQL
func (q *Query) AutoIlike(value ...bool) *Query {
	if len(value) > 0 {
		q.autoIlike = value[0]
	} else {
		q.autoIlike = true
	}
	return q
}

// Reset clears all query state
func (q *Query) Reset() *Query {
	q.selects = []string{"*"}
	q.distinct = false
	q.wheres = nil
	q.args = nil
	q.joins = nil
	q.orderBys = nil
	q.groupBys = nil
	q.havings = nil
	q.havingArgs = nil
	q.limitVal = -1
	q.offsetVal = 0
	q.lockVal = ""
	q.unions = nil
	return q
}

// Clone creates a deep copy of the Query builder
func (q *Query) Clone() *Query {
	c := *q
	c.selects = append([]string(nil), q.selects...)
	c.wheres = append([]string(nil), q.wheres...)
	c.args = append([]interface{}(nil), q.args...)
	c.joins = append([]string(nil), q.joins...)
	c.orderBys = append([]string(nil), q.orderBys...)
	c.groupBys = append([]string(nil), q.groupBys...)
	c.havings = append([]string(nil), q.havings...)
	c.havingArgs = append([]interface{}(nil), q.havingArgs...)
	c.unions = append([]string(nil), q.unions...)
	return &c
}

// Select specifies columns to select (replaces existing)
func (q *Query) Select(columns ...string) *Query {
	if len(columns) > 0 {
		q.selects = columns
	}
	return q
}

// AddSelect appends columns to existing select
func (q *Query) AddSelect(columns ...string) *Query {
	if len(q.selects) == 1 && q.selects[0] == "*" {
		q.selects = columns
	} else {
		q.selects = append(q.selects, columns...)
	}
	return q
}

// Distinct enables or disables DISTINCT
func (q *Query) Distinct(value ...bool) *Query {
	if len(value) > 0 {
		q.distinct = value[0]
	} else {
		q.distinct = true
	}
	return q
}

func (q *Query) nextPlaceholder() string {
	if q.driver == "postgres" {
		return fmt.Sprintf("$%d", len(q.args)+1)
	}
	return "?"
}

// Where adds a standard condition (replaces existing WHERE clauses)
func (q *Query) Where(column string, args ...interface{}) *Query {
	q.wheres = nil
	q.args = nil
	return q.AndWhere(column, args...)
}

// AndWhere adds an AND WHERE condition
func (q *Query) AndWhere(column string, args ...interface{}) *Query {
	if column == "" {
		return q
	}

	op := "="
	var val interface{}

	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}

	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("%s %s %s", column, op, ph)

	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	q.args = append(q.args, val)
	return q
}

// OrWhere adds an OR WHERE condition
func (q *Query) OrWhere(column string, args ...interface{}) *Query {
	if column == "" {
		return q
	}

	op := "="
	var val interface{}

	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}

	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("%s %s %s", column, op, ph)

	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "OR", clause)
	}
	q.args = append(q.args, val)
	return q
}

// WhereIn adds a WHERE IN condition
func (q *Query) WhereIn(column string, values ...interface{}) *Query {
	if len(values) == 0 {
		return q
	}
	var placeholders []string
	for _, v := range values {
		ph := q.nextPlaceholder()
		placeholders = append(placeholders, ph)
		q.args = append(q.args, v)
	}

	clause := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereNotIn adds a WHERE NOT IN condition
func (q *Query) WhereNotIn(column string, values ...interface{}) *Query {
	if len(values) == 0 {
		return q
	}
	var placeholders []string
	for _, v := range values {
		ph := q.nextPlaceholder()
		placeholders = append(placeholders, ph)
		q.args = append(q.args, v)
	}

	clause := fmt.Sprintf("%s NOT IN (%s)", column, strings.Join(placeholders, ", "))
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereNot adds a negated condition (WHERE NOT (...))
func (q *Query) WhereNot(column string, args ...interface{}) *Query {
	op := "="
	var val interface{}
	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}
	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("NOT (%s %s %s)", column, op, ph)
	q.wheres = []string{clause}
	q.args = []interface{}{val}
	return q
}

// AndWhereNot adds an AND NOT condition
func (q *Query) AndWhereNot(column string, args ...interface{}) *Query {
	op := "="
	var val interface{}
	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}
	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("NOT (%s %s %s)", column, op, ph)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	q.args = append(q.args, val)
	return q
}

// OrWhereNot adds an OR NOT condition
func (q *Query) OrWhereNot(column string, args ...interface{}) *Query {
	op := "="
	var val interface{}
	if len(args) == 1 {
		val = args[0]
	} else if len(args) >= 2 {
		op = fmt.Sprintf("%v", args[0])
		val = args[1]
	}
	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("NOT (%s %s %s)", column, op, ph)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "OR", clause)
	}
	q.args = append(q.args, val)
	return q
}

// WhereRaw adds a raw SQL WHERE condition with bound parameters
func (q *Query) WhereRaw(expression string, params ...interface{}) *Query {
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, expression)
	} else {
		q.wheres = append(q.wheres, "AND", expression)
	}
	q.args = append(q.args, params...)
	return q
}

// WhereBetween adds a BETWEEN condition
func (q *Query) WhereBetween(column string, min, max interface{}) *Query {
	ph1 := q.nextPlaceholder()
	q.args = append(q.args, min)
	ph2 := q.nextPlaceholder()
	q.args = append(q.args, max)

	clause := fmt.Sprintf("%s BETWEEN %s AND %s", column, ph1, ph2)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereNotBetween adds a NOT BETWEEN condition
func (q *Query) WhereNotBetween(column string, min, max interface{}) *Query {
	ph1 := q.nextPlaceholder()
	q.args = append(q.args, min)
	ph2 := q.nextPlaceholder()
	q.args = append(q.args, max)

	clause := fmt.Sprintf("%s NOT BETWEEN %s AND %s", column, ph1, ph2)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereNull adds IS NULL condition
func (q *Query) WhereNull(column string) *Query {
	clause := fmt.Sprintf("%s IS NULL", column)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereNotNull adds IS NOT NULL condition
func (q *Query) WhereNotNull(column string) *Query {
	clause := fmt.Sprintf("%s IS NOT NULL", column)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// WhereLike adds a LIKE condition (auto-converts to ILIKE in Postgres if autoIlike is true)
func (q *Query) WhereLike(column string, pattern string) *Query {
	likeOp := "LIKE"
	if q.autoIlike && (q.driver == "postgres" || q.driver == "pgsql" || q.driver == "postgresql") {
		likeOp = "ILIKE"
	}
	if !strings.Contains(pattern, "%") {
		pattern = "%" + pattern + "%"
	}
	ph := q.nextPlaceholder()
	clause := fmt.Sprintf("%s %s %s", column, likeOp, ph)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	q.args = append(q.args, pattern)
	return q
}

// FilterWhere skips condition if val is nil, empty string, or empty slice
func (q *Query) FilterWhere(column string, args ...interface{}) *Query {
	if len(args) == 0 {
		return q
	}
	val := args[0]
	if len(args) >= 2 {
		val = args[1]
	}
	if val == nil {
		return q
	}
	if str, ok := val.(string); ok && str == "" {
		return q
	}
	rv := reflect.ValueOf(val)
	if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array || rv.Kind() == reflect.Map) && rv.Len() == 0 {
		return q
	}
	return q.AndWhere(column, args...)
}

// FilterWhereMap applies a map of conditions, skipping null/empty values
func (q *Query) FilterWhereMap(conditions map[string]interface{}) *Query {
	for col, val := range conditions {
		q.FilterWhere(col, val)
	}
	return q
}

// AndFilterWhere alias for FilterWhere
func (q *Query) AndFilterWhere(column string, args ...interface{}) *Query {
	return q.FilterWhere(column, args...)
}

// OrFilterWhere adds an OR condition if value is non-empty
func (q *Query) OrFilterWhere(column string, args ...interface{}) *Query {
	if len(args) == 0 {
		return q
	}
	val := args[0]
	if len(args) >= 2 {
		val = args[1]
	}
	if val == nil {
		return q
	}
	if str, ok := val.(string); ok && str == "" {
		return q
	}
	rv := reflect.ValueOf(val)
	if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array || rv.Kind() == reflect.Map) && rv.Len() == 0 {
		return q
	}
	return q.OrWhere(column, args...)
}

// WhereExists adds an EXISTS (subquery) condition
func (q *Query) WhereExists(subquery *Query) *Query {
	subSQL, subArgs := subquery.BuildSQL()
	clause := fmt.Sprintf("EXISTS (%s)", subSQL)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	q.args = append(q.args, subArgs...)
	return q
}

// WhereNotExists adds a NOT EXISTS (subquery) condition
func (q *Query) WhereNotExists(subquery *Query) *Query {
	subSQL, subArgs := subquery.BuildSQL()
	clause := fmt.Sprintf("NOT EXISTS (%s)", subSQL)
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	q.args = append(q.args, subArgs...)
	return q
}

// Search adds a multi-column OR LIKE condition
func (q *Query) Search(keyword string, columns ...string) *Query {
	if keyword == "" || len(columns) == 0 {
		return q
	}

	likeOp := "LIKE"
	if q.autoIlike && (q.driver == "postgres" || q.driver == "pgsql" || q.driver == "postgresql") {
		likeOp = "ILIKE"
	}

	var orClauses []string
	searchVal := "%" + keyword + "%"

	for _, col := range columns {
		ph := q.nextPlaceholder()
		orClauses = append(orClauses, fmt.Sprintf("%s %s %s", col, likeOp, ph))
		q.args = append(q.args, searchVal)
	}

	clause := "(" + strings.Join(orClauses, " OR ") + ")"
	if len(q.wheres) == 0 {
		q.wheres = append(q.wheres, clause)
	} else {
		q.wheres = append(q.wheres, "AND", clause)
	}
	return q
}

// Join adds an INNER JOIN clause
func (q *Query) Join(table string, on string) *Query {
	q.joins = append(q.joins, fmt.Sprintf("INNER JOIN %s ON %s", table, on))
	return q
}

// InnerJoin adds an INNER JOIN clause
func (q *Query) InnerJoin(table string, on string) *Query {
	return q.Join(table, on)
}

// LeftJoin adds a LEFT JOIN clause
func (q *Query) LeftJoin(table string, on string) *Query {
	q.joins = append(q.joins, fmt.Sprintf("LEFT JOIN %s ON %s", table, on))
	return q
}

// RightJoin adds a RIGHT JOIN clause
func (q *Query) RightJoin(table string, on string) *Query {
	q.joins = append(q.joins, fmt.Sprintf("RIGHT JOIN %s ON %s", table, on))
	return q
}

// OrderBy sets ORDER BY clause
func (q *Query) OrderBy(column string, direction ...string) *Query {
	dir := "ASC"
	if len(direction) > 0 && strings.ToUpper(direction[0]) == "DESC" {
		dir = "DESC"
	}
	q.orderBys = []string{fmt.Sprintf("%s %s", column, dir)}
	return q
}

// AddOrderBy appends to ORDER BY clause
func (q *Query) AddOrderBy(column string, direction ...string) *Query {
	dir := "ASC"
	if len(direction) > 0 && strings.ToUpper(direction[0]) == "DESC" {
		dir = "DESC"
	}
	q.orderBys = append(q.orderBys, fmt.Sprintf("%s %s", column, dir))
	return q
}

// GroupBy sets GROUP BY clause
func (q *Query) GroupBy(columns ...string) *Query {
	q.groupBys = columns
	return q
}

// AddGroupBy appends to GROUP BY clause
func (q *Query) AddGroupBy(columns ...string) *Query {
	q.groupBys = append(q.groupBys, columns...)
	return q
}

// Having sets HAVING condition
func (q *Query) Having(condition string, args ...interface{}) *Query {
	q.havings = []string{condition}
	q.havingArgs = args
	return q
}

// AndHaving appends AND HAVING condition
func (q *Query) AndHaving(condition string, args ...interface{}) *Query {
	if len(q.havings) == 0 {
		q.havings = append(q.havings, condition)
	} else {
		q.havings = append(q.havings, "AND", condition)
	}
	q.havingArgs = append(q.havingArgs, args...)
	return q
}

// OrHaving appends OR HAVING condition
func (q *Query) OrHaving(condition string, args ...interface{}) *Query {
	if len(q.havings) == 0 {
		q.havings = append(q.havings, condition)
	} else {
		q.havings = append(q.havings, "OR", condition)
	}
	q.havingArgs = append(q.havingArgs, args...)
	return q
}

// Limit sets query limit
func (q *Query) Limit(limit int) *Query {
	q.limitVal = limit
	return q
}

// Offset sets query offset
func (q *Query) Offset(offset int) *Query {
	q.offsetVal = offset
	return q
}

// Lock applies row locking clause (e.g. FOR UPDATE, FOR SHARE)
func (q *Query) Lock(value ...string) *Query {
	if len(value) > 0 && value[0] != "" {
		q.lockVal = value[0]
	} else {
		q.lockVal = "FOR UPDATE"
	}
	return q
}

// ForUpdate applies FOR UPDATE pessimistic lock
func (q *Query) ForUpdate() *Query {
	return q.Lock("FOR UPDATE")
}

// ForShare applies FOR SHARE pessimistic lock
func (q *Query) ForShare() *Query {
	return q.Lock("FOR SHARE")
}

// Union adds a UNION clause
func (q *Query) Union(subquery *Query, all ...bool) *Query {
	subSQL, subArgs := subquery.BuildSQL()
	unionType := "UNION"
	if len(all) > 0 && all[0] {
		unionType = "UNION ALL"
	}
	q.unions = append(q.unions, fmt.Sprintf("%s %s", unionType, subSQL))
	q.args = append(q.args, subArgs...)
	return q
}

// BuildSQL compiles current query state into SQL string and arguments
func (q *Query) BuildSQL() (string, []interface{}) {
	var sqlStr strings.Builder
	sqlStr.WriteString("SELECT ")
	if q.distinct {
		sqlStr.WriteString("DISTINCT ")
	}
	sqlStr.WriteString(strings.Join(q.selects, ", "))
	sqlStr.WriteString(" FROM ")
	sqlStr.WriteString(q.table)

	if len(q.joins) > 0 {
		sqlStr.WriteString(" ")
		sqlStr.WriteString(strings.Join(q.joins, " "))
	}

	if len(q.wheres) > 0 {
		sqlStr.WriteString(" WHERE ")
		sqlStr.WriteString(strings.Join(q.wheres, " "))
	}

	if len(q.groupBys) > 0 {
		sqlStr.WriteString(" GROUP BY ")
		sqlStr.WriteString(strings.Join(q.groupBys, ", "))
	}

	if len(q.havings) > 0 {
		sqlStr.WriteString(" HAVING ")
		sqlStr.WriteString(strings.Join(q.havings, " "))
	}

	if len(q.orderBys) > 0 {
		sqlStr.WriteString(" ORDER BY ")
		sqlStr.WriteString(strings.Join(q.orderBys, ", "))
	}

	if q.limitVal >= 0 {
		sqlStr.WriteString(" LIMIT ")
		sqlStr.WriteString(strconv.Itoa(q.limitVal))
	}

	if q.offsetVal > 0 {
		sqlStr.WriteString(" OFFSET ")
		sqlStr.WriteString(strconv.Itoa(q.offsetVal))
	}

	if len(q.unions) > 0 {
		sqlStr.WriteString(" ")
		sqlStr.WriteString(strings.Join(q.unions, " "))
	}

	if q.lockVal != "" {
		sqlStr.WriteString(" ")
		sqlStr.WriteString(q.lockVal)
	}

	allArgs := append([]interface{}(nil), q.args...)
	if len(q.havingArgs) > 0 {
		allArgs = append(allArgs, q.havingArgs...)
	}

	return sqlStr.String(), allArgs
}

// RawSQL returns raw interpolated SQL string for debugging
func (q *Query) RawSQL() string {
	sqlStr, args := q.BuildSQL()
	for _, arg := range args {
		var valStr string
		switch v := arg.(type) {
		case string:
			valStr = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
		case nil:
			valStr = "NULL"
		case bool:
			if v {
				valStr = "1"
			} else {
				valStr = "0"
			}
		default:
			valStr = fmt.Sprintf("%v", v)
		}
		if q.driver == "postgres" {
			re := regexp.MustCompile(`\$\d+`)
			loc := re.FindStringIndex(sqlStr)
			if loc != nil {
				sqlStr = sqlStr[:loc[0]] + valStr + sqlStr[loc[1]:]
			}
		} else {
			sqlStr = strings.Replace(sqlStr, "?", valStr, 1)
		}
	}
	return sqlStr
}

// Count returns total number of matching records
func (q *Query) Count(col ...string) (int64, error) {
	clone := q.Clone()
	countCol := "*"
	if len(col) > 0 && col[0] != "" {
		countCol = col[0]
	}
	clone.selects = []string{fmt.Sprintf("COUNT(%s)", countCol)}
	clone.limitVal = -1
	clone.offsetVal = 0
	clone.orderBys = nil

	querySQL, args := clone.BuildSQL()
	var total int64
	err := q.db.QueryRow(querySQL, args...).Scan(&total)
	return total, err
}

// Exists checks if any record matches
func (q *Query) Exists() (bool, error) {
	clone := q.Clone()
	clone.selects = []string{"1"}
	clone.limitVal = 1
	clone.offsetVal = 0

	querySQL, args := clone.BuildSQL()
	var val int
	err := q.db.QueryRow(querySQL, args...).Scan(&val)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Sum calculates SUM of column
func (q *Query) Sum(column string) (float64, error) {
	clone := q.Clone()
	clone.selects = []string{fmt.Sprintf("SUM(%s)", column)}
	clone.limitVal = -1
	clone.offsetVal = 0
	clone.orderBys = nil

	querySQL, args := clone.BuildSQL()
	var total sql.NullFloat64
	err := q.db.QueryRow(querySQL, args...).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// Average calculates AVG of column
func (q *Query) Average(column string) (float64, error) {
	clone := q.Clone()
	clone.selects = []string{fmt.Sprintf("AVG(%s)", column)}
	clone.limitVal = -1
	clone.offsetVal = 0
	clone.orderBys = nil

	querySQL, args := clone.BuildSQL()
	var avg sql.NullFloat64
	err := q.db.QueryRow(querySQL, args...).Scan(&avg)
	if err != nil {
		return 0, err
	}
	return avg.Float64, nil
}

// Avg is an alias for Average
func (q *Query) Avg(column string) (float64, error) {
	return q.Average(column)
}

// Min calculates MIN of column
func (q *Query) Min(column string) (interface{}, error) {
	clone := q.Clone()
	clone.selects = []string{fmt.Sprintf("MIN(%s)", column)}
	clone.limitVal = -1
	clone.offsetVal = 0
	clone.orderBys = nil

	querySQL, args := clone.BuildSQL()
	var val interface{}
	err := q.db.QueryRow(querySQL, args...).Scan(&val)
	return val, err
}

// Max calculates MAX of column
func (q *Query) Max(column string) (interface{}, error) {
	clone := q.Clone()
	clone.selects = []string{fmt.Sprintf("MAX(%s)", column)}
	clone.limitVal = -1
	clone.offsetVal = 0
	clone.orderBys = nil

	querySQL, args := clone.BuildSQL()
	var val interface{}
	err := q.db.QueryRow(querySQL, args...).Scan(&val)
	return val, err
}

// Scalar executes query and scans into dest pointer
func (q *Query) Scalar(dest interface{}) error {
	q.Limit(1)
	querySQL, args := q.BuildSQL()
	return q.db.QueryRow(querySQL, args...).Scan(dest)
}

// Column executes query and returns a slice of values for a single column
func (q *Query) Column(columnName string) ([]interface{}, error) {
	q.Select(columnName)
	querySQL, args := q.BuildSQL()
	rows, err := q.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []interface{}
	for rows.Next() {
		var val interface{}
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, rows.Err()
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

// One is alias for First
func (q *Query) One(dest interface{}) error {
	return q.First(dest)
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

// Insert executes INSERT query and returns inserted ID
func (q *Query) Insert(data map[string]interface{}) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	var columns []string
	var placeholders []string
	var args []interface{}
	idx := 1

	for col, val := range data {
		columns = append(columns, col)
		ph := "?"
		if q.driver == "postgres" {
			ph = fmt.Sprintf("$%d", idx)
		}
		placeholders = append(placeholders, ph)
		args = append(args, val)
		idx++
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	if q.driver == "postgres" {
		sqlStr += " RETURNING id"
		var id int64
		err := q.db.QueryRow(sqlStr, args...).Scan(&id)
		return id, err
	}

	res, err := q.db.Exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update executes UPDATE query matching current where conditions
func (q *Query) Update(data map[string]interface{}) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	var setClauses []string
	var args []interface{}
	idx := 1

	for col, val := range data {
		ph := "?"
		if q.driver == "postgres" {
			ph = fmt.Sprintf("$%d", idx)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", col, ph))
		args = append(args, val)
		idx++
	}

	var sqlStr strings.Builder
	sqlStr.WriteString("UPDATE ")
	sqlStr.WriteString(q.table)
	sqlStr.WriteString(" SET ")
	sqlStr.WriteString(strings.Join(setClauses, ", "))

	if len(q.wheres) > 0 {
		whereClauses := q.wheres
		if q.driver == "postgres" {
			whereClauses = make([]string, len(q.wheres))
			copy(whereClauses, q.wheres)
		}
		sqlStr.WriteString(" WHERE ")
		sqlStr.WriteString(strings.Join(whereClauses, " "))
		args = append(args, q.args...)
	}

	res, err := q.db.Exec(sqlStr.String(), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Delete executes DELETE query matching current where conditions
func (q *Query) Delete() (int64, error) {
	var sqlStr strings.Builder
	sqlStr.WriteString("DELETE FROM ")
	sqlStr.WriteString(q.table)
	if len(q.wheres) > 0 {
		sqlStr.WriteString(" WHERE ")
		sqlStr.WriteString(strings.Join(q.wheres, " "))
	}
	res, err := q.db.Exec(sqlStr.String(), q.args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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
