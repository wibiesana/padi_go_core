# Changelog

All notable changes to `padi_go_core` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### 🌟 Added

#### ActiveRecord
- **Out-of-the-Box Eager Loading & Generic Fluent Querying**:
  - Added `ModelQuery[T]` fluent query builder (`NewModelQuery[T]()`).
  - Added `With(relations...)` with support for nested recursive relations (e.g. `comments.author`) and column filtering (e.g. `user:id,username`).
  - Added `Get()`, `First()`, `Find(id)`, `Paginate(opts, searchCols)`, `GetMaps()`, `PaginateMaps(opts, searchCols)`.
  - Added relationship declarations: `BelongsTo`, `HasOne`, `HasMany`, `BelongsToMany`.
  - Added `RegisterRelationModels` and model registry for automatic reflection instantiation of nested relation structs.
  - Added `structTypeInfo` reflection metadata cache (`map[reflect.Type]*structTypeInfo` guarded by `sync.RWMutex`) providing $O(1)$ struct field mapping and eliminating repeated reflective inspection across loops.
  - Added `buildInPlaceholders` buffer helper to build `?, ?, ?` or `$1, $2, $3` SQL in-place without slice allocations.
  - Added lazy `initMu()` helper ensuring pointer-mutex safety across struct value copies.

#### Auth
- Added HTTP request extraction helpers:
  - `ExtractToken(r *http.Request) string`
  - `User(r *http.Request) *JWTClaims`
  - `UserID(r *http.Request) uint`
  - `UserFromContext(ctx context.Context) *JWTClaims`
  - `UserIDFromContext(ctx context.Context) uint`
  - `Check(r *http.Request) bool`
  - `HasRole(r *http.Request, roles ...string) bool`
- Added `VerifyPassword(p1, p2 string) bool` and `VerifyToken(tokenString string) (*JWTClaims, error)` aliases.
- Added `GenerateTokenWithExpiry(userID uint, email, role string, expiry time.Duration, meta ...map[string]interface{}) (string, error)`.

#### Cache
- Added generic type-safe caching helpers:
  - `GetTyped[T any](key string) (T, bool, error)`
  - `RememberTyped[T any](key string, ttl time.Duration, fallbackFn func() (T, error)) (T, error)`
- Added `Clear()` and `ClearMemory()` for full cache invalidation matching PHP `Cache.php`.

#### Config
- Added `GetEnvFloat(key string, defaultVal float64) float64`.
- Added `GetEnvDuration(key string, defaultVal time.Duration) time.Duration`.
- Added `Current() *Config`, `Env(key, defaultVal) string`, `Get(key, defaultVal) string`, `SetEnv(key, value)`.
- Added environment checkers: `IsProduction() bool`, `IsDevelopment() bool`, `IsTesting() bool`.

#### Database
- Added auto-commit / auto-rollback transaction management:
  - `Transaction(fn func(tx *sql.Tx) error) error`
  - `TransactionContext(ctx context.Context, fn func(tx *sql.Tx) error) error`
- Added raw query execution with query telemetry tracking:
  - `Exec(query string, args ...interface{}) (sql.Result, error)`
  - `ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)`
  - `Query(query string, args ...interface{}) (*sql.Rows, error)`
  - `QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)`
  - `QueryRow(query string, args ...interface{}) *sql.Row`
  - `QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row`
- Added `Ping() error`, `Close() error`, and `Stats() sql.DBStats`.

#### Email
- Added multipart MIME attachment handling with base64 streaming.
- Added support for `Cc`, `Bcc`, `ReplyTo`, and `Attachments` in `Message` struct.
- Added `SendHTML(to, subject, htmlBody, attachments...) error`.
- Added `SendText(to, subject, textBody) error`.
- Added `SendTo(toSlice, subject, htmlBody) error`.
- Added `SendTemplate(toSlice, subject, templateStr, data) error` rendering Go `html/template`.
- Added `SendAsync(msg, onComplete...)` for non-blocking background email dispatch.

#### File & Storage
- Added `file.Path(relativePath) string` and `storage.DiskPath(relativePath) string`.
- Added `file.Put(relativePath, bytes)` & `file.PutString(relativePath, text)`.
- Added `file.Get(relativePath)` & `file.GetString(relativePath)`.
- Added `file.SanitizeFileName(name) string`.
- Added `storage.Copy(srcRel, destRel)` & `storage.Move(srcRel, destRel)`.
- Added `storage.MakeDirectory(relDir)` & `storage.DeleteDirectory(relDir)`.
- Added `storage.List(relDir) ([]string, error)`.

#### Logger
- Added Printf-style formatted logging methods:
  - `Infof(format string, args...)`
  - `Warningf(format string, args...)` / `Warnf(format string, args...)`
  - `Warn(message string, context...)`
  - `Errorf(format string, args...)`
  - `Debugf(format string, args...)`
  - `Criticalf(format string, args...)`
- Added `RotateLogs(retentionDays ...int)` manual trigger.

#### Query Builder
- Added generic type-safe execution functions:
  - `query.GetAll[T any](q *Query) ([]T, error)`
  - `query.GetFirst[T any](q *Query) (*T, error)`
  - `query.GetOne[T any](q *Query) (*T, error)`

#### Queue
- Added generic type-safe job handler:
  - `queue.RegisterTyped[T any](name string, handler func(data T) error)`
- Added delayed dispatching:
  - `queue.PushLater(delay time.Duration, jobName string, data interface{}, queueName...) error`
  - `queue.Later(delay time.Duration, jobName string, data interface{}, queueName...) error`
- Added queue monitoring & worker control:
  - `queue.Size(queueName...) (int64, error)`
  - `queue.Clear(queueName...) error`
  - `queue.WorkWithContext(ctx context.Context, queueName string, maxJobs int)`

#### Realtime
- Added `PublishBatch(events []Event)` for multi-event broadcasts.
- Added `Broadcast(data interface{})` to broadcast across all active topics.
- Added `SubscriberCount(topic string) int` and `Topics() []string`.
- Added dynamic URL query parameter fallback (`?topic=` / `?topics=`) in `SubscribeSSE()`.

#### Response
- Added `response.NoContent(w)` (HTTP 204).
- Added `response.Conflict(w, message, errors...)` (HTTP 409).
- Added `response.TooManyRequests(w, message...)` (HTTP 429).
- Added `response.Download(w, r, filePath, customName...)` with `Content-Disposition`.

#### Router
- Added `r.Any(pattern, handler)` matching all HTTP methods.
- Added `r.Static(pattern, rootDir)` serving local filesystem assets.
- Added `ParamInt(r, key, defaultVal...) int`.
- Added `QueryParamFloat(r, key, defaultVal) float64`.
- Added `QueryParamBool(r, key, defaultVal) bool`.
- Added `QueryParamSlice(r, key, separator...) []string`.

#### Validator
- Added generic one-line request binding and validation:
  - `validator.Bind[T any](r *http.Request) (*T, ValidationErrorDetails, error)`
- Added fluent `FormValidator` builder:
  - `v := validator.New(r)` / `validator.New(dataMap)`
  - `v.Required(...)`, `v.Email(...)`, `v.Min(...)`, `v.Max(...)`
  - `v.Passes()`, `v.Fails()`, `v.Errors()`

#### Migrator
- Added `MigrationStatus` struct and `migrator.Status(db) ([]MigrationStatus, error)`.
- Added `migrator.Reset(db)` and `migrator.Fresh(db)`.
- Added `migrator.ClearRegistry()` for test isolation.

#### Wizard
- Added `wizard_test.go` unit test suite for setup wizard instantiation and configuration creation.

---

### ⚡ Performance & DRY Improvements
- **Reflection Metadata Caching**: Eliminated repeated struct inspection by caching parsed fields in `sync.RWMutex`-guarded type metadata maps across ActiveRecord and Query Builder.
- **Zero-Allocation Placeholder Generator**: Implemented `buildInPlaceholders` buffer for fast SQL `IN (...)` queries.
- **String Concat Optimization**: Replaced temporary `+ "\r\n"` string allocations in base64 attachment writer loop with sequential `WriteString` calls.
- **Database Iterator Safety**: Added `rows.Err()` check on all `for rows.Next()` loops.

---

### 🐛 Fixed
- Fixed `go vet` lock-by-value warning on `ActiveRecord` struct by converting `relationsMu sync.RWMutex` to pointer `*sync.RWMutex` with lazy `initMu()`.
- Fixed `rows.Err()` check missing after `for rows.Next()` loop in `migrator.Status`.
