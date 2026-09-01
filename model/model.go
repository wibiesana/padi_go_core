package model

import (
	"context"
	"database/sql"

	"github.com/wibiesana/padi_go_core/activerecord"
	"github.com/wibiesana/padi_go_core/query"
	"github.com/wibiesana/padi_go_core/response"
)

// Type aliases to activerecord
type Model = activerecord.Model
type PrimaryKeyer = activerecord.PrimaryKeyer
type Connectioner = activerecord.Connectioner
type Fillabler = activerecord.Fillabler
type HiddenFields = activerecord.HiddenFields
type DefaultOrderer = activerecord.DefaultOrderer
type Auditable = activerecord.Auditable
type RelationDefiner = activerecord.RelationDefiner

type BeforeSaver = activerecord.BeforeSaver
type AfterSaver = activerecord.AfterSaver
type BeforeDeleter = activerecord.BeforeDeleter
type AfterDeleter = activerecord.AfterDeleter
type AfterLoader = activerecord.AfterLoader

type RelationType = activerecord.RelationType
type Relation = activerecord.Relation
type ActiveRecord = activerecord.ActiveRecord

const (
	RelHasOne        = activerecord.RelHasOne
	RelHasMany       = activerecord.RelHasMany
	RelBelongsTo     = activerecord.RelBelongsTo
	RelBelongsToMany = activerecord.RelBelongsToMany
)

// Forwarding functions
func ClearColumnsCache() {
	activerecord.ClearColumnsCache()
}

func GetTable[T Model]() string {
	return activerecord.GetTable[T]()
}

func GetDb(m ...Model) *sql.DB {
	return activerecord.GetDb(m...)
}

func GetConnectionName(m ...Model) string {
	return activerecord.GetConnectionName(m...)
}

func GetLikeOperator() string {
	return activerecord.GetLikeOperator()
}

func GetPrimaryKeyName(m any) string {
	return activerecord.GetPrimaryKeyName(m)
}

func HasOne(table string, foreignKey string, localKey ...string) Relation {
	return activerecord.HasOne(table, foreignKey, localKey...)
}

func HasMany(table string, foreignKey string, localKey ...string) Relation {
	return activerecord.HasMany(table, foreignKey, localKey...)
}

func BelongsTo(table string, foreignKey string, ownerKey ...string) Relation {
	return activerecord.BelongsTo(table, foreignKey, ownerKey...)
}

func BelongsToMany(table string, pivotTable string, foreignKey string, relatedKey string) Relation {
	return activerecord.BelongsToMany(table, pivotTable, foreignKey, relatedKey)
}

var (
	HasOneRel        = activerecord.HasOneRel
	HasManyRel       = activerecord.HasManyRel
	BelongsToRel     = activerecord.BelongsToRel
	BelongsToManyRel = activerecord.BelongsToManyRel
)

func GetRelationConfig(m Model, name string) *Relation {
	return activerecord.GetRelationConfig(m, name)
}

func GetWith(m Model) []string {
	return activerecord.GetWith(m)
}

func With[T Model](relations ...string) *query.Query {
	return activerecord.With[T](relations...)
}

func ClearWith() {
	activerecord.ClearWith()
}

func FindBuilder[T Model]() *query.Query {
	return activerecord.FindBuilder[T]()
}

func FindQuery[T Model]() *query.Query {
	return activerecord.FindQuery[T]()
}

func Search[T Model](keyword string, extraColumns ...string) *query.Query {
	return activerecord.Search[T](keyword, extraColumns...)
}

func SanitizeOrderBy(orderBy string) (string, error) {
	return activerecord.SanitizeOrderBy(orderBy)
}

func GetPkConditions(m Model, id interface{}) (map[string]interface{}, error) {
	return activerecord.GetPkConditions(m, id)
}

func Find[T Model](id interface{}, contexts ...context.Context) (*T, error) {
	return activerecord.Find[T](id, contexts...)
}

func FindByPk[T Model](id interface{}, contexts ...context.Context) (*T, error) {
	return activerecord.FindByPk[T](id, contexts...)
}

func FindOne[T Model](condition interface{}, contexts ...context.Context) (*T, error) {
	return activerecord.FindOne[T](condition, contexts...)
}

func FindAll[T Model](condition ...interface{}) ([]T, error) {
	return activerecord.FindAll[T](condition...)
}

func FindOrFail[T Model](id interface{}, contexts ...context.Context) (*T, error) {
	return activerecord.FindOrFail[T](id, contexts...)
}

func FindBy[T Model](column string, val interface{}) (*T, error) {
	return activerecord.FindBy[T](column, val)
}

func All[T Model](contexts ...context.Context) ([]T, error) {
	return activerecord.All[T](contexts...)
}

func Get[T Model](contexts ...context.Context) ([]T, error) {
	return activerecord.Get[T](contexts...)
}

func Where[T Model](column string, args ...interface{}) *query.Query {
	return activerecord.Where[T](column, args...)
}

func FilterWhere[T Model](conditions map[string]interface{}) *query.Query {
	return activerecord.FilterWhere[T](conditions)
}

func Count[T Model](conditions ...map[string]interface{}) (int64, error) {
	return activerecord.Count[T](conditions...)
}

func Paginate[T Model](opts query.Options, searchColumns []string, contexts ...context.Context) (response.Pagination, []T, error) {
	return activerecord.Paginate[T](opts, searchColumns, contexts...)
}

func PaginateWithConditions[T Model](page int, perPage int, conditions map[string]interface{}, orderBy ...string) (response.Pagination, []T, error) {
	return activerecord.PaginateWithConditions[T](page, perPage, conditions, orderBy...)
}

func Create[T Model](data map[string]interface{}) (*T, error) {
	return activerecord.Create[T](data)
}

func InsertRecord[T Model](data map[string]interface{}) (interface{}, error) {
	return activerecord.InsertRecord[T](data)
}

func Update[T Model](id interface{}, data map[string]interface{}) error {
	return activerecord.Update[T](id, data)
}

func UpdateRecord[T Model](id interface{}, data map[string]interface{}) error {
	return activerecord.UpdateRecord[T](id, data)
}

func UpdateAll[T Model](data map[string]interface{}, conditions map[string]interface{}) (int64, error) {
	return activerecord.UpdateAll[T](data, conditions)
}

func Delete[T Model](id interface{}) error {
	return activerecord.Delete[T](id)
}

func DeleteRecord[T Model](id interface{}) error {
	return activerecord.DeleteRecord[T](id)
}

func DeleteAll[T Model](condition ...interface{}) (int64, error) {
	return activerecord.DeleteAll[T](condition...)
}

func Save(m any, contexts ...context.Context) error {
	return activerecord.Save(m, contexts...)
}

func DeleteModel(m any, contexts ...context.Context) error {
	return activerecord.DeleteModel(m, contexts...)
}

func SoftDelete(m any, contexts ...context.Context) error {
	return activerecord.SoftDelete(m, contexts...)
}

func SoftDeleteByID[T Model](id interface{}) error {
	return activerecord.SoftDeleteByID[T](id)
}

func BatchInsert[T Model](items []T, chunkSize ...int) error {
	return activerecord.BatchInsert[T](items, chunkSize...)
}

func Upsert[T Model](data map[string]interface{}, updateColumns ...string) (int64, error) {
	return activerecord.Upsert[T](data, updateColumns...)
}

func Query[T any](sqlStr string, args ...interface{}) ([]T, error) {
	return activerecord.Query[T](sqlStr, args...)
}

func GetTableColumns(tableName string) ([]string, error) {
	return activerecord.GetTableColumns(tableName)
}

func FilterFillable(m Model, data map[string]interface{}) map[string]interface{} {
	return activerecord.FilterFillable(m, data)
}

func HideFields(items interface{}) {
	activerecord.HideFields(items)
}

func LoadRelations(items interface{}, relations ...string) error {
	return activerecord.LoadRelations(items, relations...)
}
