---
title: Configuration
description: Complete reference for goway.Configure and every configuration setter.
---

The `Configuration` type collects all settings that control how migrations are discovered and applied. Create one with [`Configure()`](#configure), chain setter methods to customize behavior, and finalize it with [`Load()`](#load) or [`LoadContext()`](#loadcontext) to obtain a ready `Migrator`.

## Configure

```go
func Configure() *Configuration
```

Returns a new `Configuration` with the same defaults as Flyway. Every default is documented with its setter below. All setters return `*Configuration` for method chaining.

## Load and LoadContext

```go
func (c *Configuration) Load() (*Migrator, error)

func (c *Configuration) LoadContext(ctx context.Context) (*Migrator, error)
```

Validates the configuration, auto-detects the dialect if one was not set explicitly, discovers migrations from the configured locations, and returns a ready `Migrator`. Errors include [`ErrNoDataSource`](/goway/reference/commands/), [`ErrNoDialect`](/goway/reference/commands/), and [`ErrUnsupportedDialect`](/goway/reference/commands/).

`LoadContext` respects the supplied context during dialect detection.

:::note
Configuration errors from invalid `BaselineVersion` or `Target` arguments surface at `Load` time, not when the setter was called.
:::

## Configuration Setters

### DataSource

```go
func (c *Configuration) DataSource(db *sql.DB) *Configuration
```

Sets the database connection pool used for all operations. Required; `Load` returns [`ErrNoDataSource`](/goway/reference/commands/) if this is not set.

**Default:** `nil` (required)

### Dialect

```go
func (c *Configuration) Dialect(dialect Dialect) *Configuration
```

Sets the database dialect explicitly. If not set, the dialect is auto-detected from the connection at `Load` time. Use `Postgres()` or `SQLite()` to create dialect instances.

**Default:** `nil` (auto-detect)

### Locations

```go
func (c *Configuration) Locations(locations ...string) *Configuration
```

Replaces the list of file system directories scanned for migrations. Each location may carry a `"filesystem:"` or `"classpath:"` prefix; bare paths are also accepted. Locations are processed in order; the first matching migration found wins.

**Default:** `[]string{"db/migration"}`

### FS

```go
func (c *Configuration) FS(fileSystem fs.FS, paths ...string) *Configuration
```

Registers an additional file system, such as one created by `go:embed`, together with the directory paths inside it that contain migrations. If no paths are provided, defaults to `"."`. Can be called multiple times to register multiple file systems.

**Default:** (none; only filesystem locations are scanned unless this is called)

### Schemas

```go
func (c *Configuration) Schemas(schemas ...string) *Configuration
```

Sets the list of schemas managed by the migrator. The first schema is the default schema in which the schema history table is created (unless overridden by `DefaultSchema`). Schema management is only relevant for PostgreSQL; SQLite does not support schemas.

**Default:** `nil` (no explicit schema list; the default schema is used)

### DefaultSchema

```go
func (c *Configuration) DefaultSchema(schema string) *Configuration
```

Overrides the schema in which the schema history table is created and managed. If not set, the first schema from `Schemas` is used, or the database default if no schemas are configured.

**Default:** `""` (use first schema from `Schemas`, or the database default)

### CreateSchemas

```go
func (c *Configuration) CreateSchemas(create bool) *Configuration
```

Controls whether missing schemas are created automatically before migrations run. Only applicable on PostgreSQL.

**Default:** `true`

### Table

```go
func (c *Configuration) Table(table string) *Configuration
```

Sets the name of the schema history table. The name should not be schema-qualified; the table is created in the default schema.

**Default:** `"flyway_schema_history"`

### BaselineVersion

```go
func (c *Configuration) BaselineVersion(version string) *Configuration
```

Sets the version recorded by the `baseline` command and below which migrations are ignored. The version string is parsed and validated; invalid formats surface as an error from `Load`. Version format is a dot or underscore-separated sequence of integers with arbitrary precision.

**Default:** `"1"`

### BaselineDescription

```go
func (c *Configuration) BaselineDescription(description string) *Configuration
```

Sets the description text recorded by the `baseline` command.

**Default:** `"<< Flyway Baseline >>"`

### BaselineOnMigrate

```go
func (c *Configuration) BaselineOnMigrate(enabled bool) *Configuration
```

Controls whether a non-empty schema without a history table is baselined automatically on the first `migrate` command.

**Default:** `false`

### Placeholders

```go
func (c *Configuration) Placeholders(placeholders map[string]string) *Configuration
```

Sets the placeholder key-value map. Keys are matched case-insensitively. Unresolved placeholders in migration scripts are errors. Built-in placeholders are always available: `${flyway:defaultSchema}`, `${flyway:user}`, `${flyway:table}`, `${flyway:filename}`.

**Default:** `map[string]string{}` (empty; only built-ins available)

### PlaceholderPrefix

```go
func (c *Configuration) PlaceholderPrefix(prefix string) *Configuration
```

Sets the opening delimiter of a placeholder.

**Default:** `"${"`

### PlaceholderSuffix

```go
func (c *Configuration) PlaceholderSuffix(suffix string) *Configuration
```

Sets the closing delimiter of a placeholder.

**Default:** `"}"`

### SQLMigrationPrefix

```go
func (c *Configuration) SQLMigrationPrefix(prefix string) *Configuration
```

Sets the prefix that marks versioned migration scripts. Migration names match the pattern `<prefix><version><separator><description><suffix>`, e.g., `V1__init.sql`.

**Default:** `"V"`

### RepeatableSQLMigrationPrefix

```go
func (c *Configuration) RepeatableSQLMigrationPrefix(prefix string) *Configuration
```

Sets the prefix that marks repeatable migration scripts. Repeatable migrations run after all versioned migrations, and are re-applied whenever their checksum changes. Migration names match the pattern `<prefix>__<description><suffix>`, e.g., `R__index_on_user_id.sql`.

**Default:** `"R"`

### SQLMigrationSeparator

```go
func (c *Configuration) SQLMigrationSeparator(separator string) *Configuration
```

Sets the token between the version and description in versioned migration names.

**Default:** `"__"`

### SQLMigrationSuffixes

```go
func (c *Configuration) SQLMigrationSuffixes(suffixes ...string) *Configuration
```

Sets the recognized file suffixes for migration scripts. Migration files matching any of these suffixes are considered.

**Default:** `[]string{".sql"}`

### ValidateOnMigrate

```go
func (c *Configuration) ValidateOnMigrate(enabled bool) *Configuration
```

Controls whether the `migrate` command runs a validation check before applying migrations. If validation fails, `Migrate` returns an error and no migrations are applied.

**Default:** `true`

### CleanDisabled

```go
func (c *Configuration) CleanDisabled(disabled bool) *Configuration
```

Controls whether the `clean` command is permitted. When `true`, calling `Clean` returns [`ErrCleanDisabled`](/goway/reference/commands/). Set to `false` only when you intentionally need to drop schema objects.

**Default:** `true` (clean is disabled for safety)

### OutOfOrder

```go
func (c *Configuration) OutOfOrder(enabled bool) *Configuration
```

Controls whether migrations with a version lower than the current schema version may still be applied. When `false`, out-of-order migrations are an error.

**Default:** `false`

### Target

```go
func (c *Configuration) Target(version string) *Configuration
```

Sets the highest version that `migrate` will apply. Migrations above the target are not executed. The version string is parsed and validated; invalid formats surface as an error from `Load`.

**Default:** `nil` (no upper limit; all migrations are applied)

### InstalledBy

```go
func (c *Configuration) InstalledBy(user string) *Configuration
```

Overrides the user name recorded for each applied migration in the schema history table.

**Default:** `""` (the database user is recorded)

### Callbacks

```go
func (c *Configuration) Callbacks(callbacks ...Callback) *Configuration
```

Registers programmatic callbacks invoked during a `migrate` run, in addition to SQL callback scripts discovered in the configured locations. See [Callbacks](/goway/reference/callbacks/) for event types and the callback interface.

**Default:** (none)

## Example

```go
package main

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/cgardev/goway"
)

func main() {
	db, err := sql.Open("pgx", "postgres://user:password@localhost/mydb")
	if err != nil {
		log.Fatal(err)
	}

	config := goway.Configure().
		DataSource(db).
		Schemas("public", "data").
		DefaultSchema("public").
		Locations("filesystem:db/migrations", "filesystem:db/seeds").
		Placeholders(map[string]string{
			"schema_version": "1.0.0",
		}).
		ValidateOnMigrate(true)

	migrator, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Applied %d migrations", result.MigrationsExecuted)
}
```
