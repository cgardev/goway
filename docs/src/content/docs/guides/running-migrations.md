---
title: Running Migrations
description: Build a migrator with the fluent configuration and run migrate, info, and validate.
---

## Overview

The typical workflow is to configure migrations at application startup, then invoke the migrate command. The configuration is fluent—each setter returns the `Configuration` so calls can be chained—and finalized by calling `Load()`, which returns a `Migrator` ready to run commands.

## Configuration and Loading

All goway operations begin with `Configure()`, a function that returns a `*Configuration` with Flyway-compatible defaults. From there, you chain setters for your data source, dialect, migration locations, and any other options:

```go
package main

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/cgardev/goway"
)

func main() {
	db, err := sql.Open("pgx", "postgres://localhost/mydb")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	migrator, err := goway.Configure().
		DataSource(db).
		Locations("db/migration").
		Load(context.Background())
	if err != nil {
		panic(err)
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		panic(err)
	}
	println("Applied", result.MigrationsExecuted, "migrations")
}
```

### Data Source

Every configuration must include a database connection set with `DataSource()`. The connection pool is used for all operations and must remain open for the lifetime of commands:

```go
db, err := sql.Open("pgx", "postgres://localhost/mydb")
// or
db, err := sql.Open("sqlite", "file.db")
```

### Dialect Detection

By default, the dialect is detected automatically from the database when you call `Load()` or `LoadContext()`. If detection fails or you need to override it, set the dialect explicitly:

```go
migrator, err := goway.Configure().
	DataSource(db).
	Dialect(goway.Postgres()).
	Load(context.Background())
```

Currently supported dialects are `Postgres()` and `SQLite()`.

### Migration Locations

Migrations are discovered from one or more locations. The default is `"db/migration"` relative to the working directory. Locations can use `"filesystem:"` or `"classpath:"` prefixes (or no prefix), and `Load()` tolerates missing directories:

```go
goway.Configure().
	Locations("filesystem:db/migration", "db/migrations/custom").
	Load(ctx)
```

### Embedding Migrations with go:embed

For a production binary, embed migrations directly using Go's `embed` package and register the embedded file system with `FS()`:

```go
import "embed"

//go:embed migrations/*.sql
var migrations embed.FS

migrator, err := goway.Configure().
	DataSource(db).
	FS(migrations, "migrations").
	Load(context.Background())
```

You can register multiple embedded file systems, and they are scanned in the order added, before any filesystem locations.

## Running Migrate

The `Migrate()` command brings the database schema up to date by applying every pending migration in ascending version order. It is safe to call multiple times and idempotent—migrations that have already been applied are skipped.

### Step-by-Step

When you call `Migrate()`, the migrator performs these steps:

1. **Acquires an exclusive lock** to ensure only one migrator runs at a time. On PostgreSQL this is a session advisory lock; SQLite is single-writer by nature.
2. **Creates missing schemas** if configured with `Schemas()` and `CreateSchemas(true)` (the default).
3. **Creates the schema history table** if it does not exist. This table records every migration applied and is created in the default schema.
4. **Validates pending migrations** if `ValidateOnMigrate(true)` (the default). Validation checks for issues like missing scripts or checksum mismatches.
5. **Fires beforeMigrate callbacks**, allowing custom logic before any migration runs.
6. **Applies each pending migration in a transaction** (unless the script opts out with `-- goway:noTransaction`). Statements are executed in order, and the history row is recorded in the same transaction, so a failure rolls back both and leaves no partial state.
7. **Fires afterMigrate callbacks** after all migrations succeed.

If any step fails, the error is returned and no further migrations are applied. A failed migration can be retried after the script is corrected.

### MigrateResult

The result includes the initial and target schema versions, the number of migrations executed, and detailed information about each migration applied:

```go
result, err := migrator.Migrate(ctx)
if err != nil {
	panic(err)
}

println("Initial version:", result.InitialSchemaVersion)
println("Target version:", result.TargetSchemaVersion)
println("Applied", result.MigrationsExecuted, "migrations")

for _, migration := range result.Migrations {
	println("Applied", migration.Version, "-", migration.Description)
}
```

:::note
If validation fails during migrate and `ValidateOnMigrate(true)`, the error wraps `goway.ErrValidationFailed`. Use `errors.Is(err, goway.ErrValidationFailed)` to distinguish validation errors from other failures.
:::

### At Application Startup

A common pattern is to run migrations automatically on application startup, before serving requests:

```go
func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator, err := goway.Configure().
		DataSource(db).
		Locations("db/migration").
		Load(context.Background())
	if err != nil {
		log.Fatal("load migrator:", err)
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatal("migrate:", err)
	}
	log.Printf("applied %d migrations (v%s → v%s)",
		result.MigrationsExecuted,
		result.InitialSchemaVersion,
		result.TargetSchemaVersion)

	// now safe to use the database
	runServer(db)
}
```

## Inspecting State with Info

The `Info()` command reports the state of every resolved migration (both applied and pending) without changing the database:

```go
result, err := migrator.Info(ctx)
if err != nil {
	panic(err)
}

println("Current schema version:", result.Current)
for _, migration := range result.Migrations {
	println(migration.Version, "-", migration.Description, "-", migration.State)
}
```

The `State` field is one of:
- `Success` – applied and checksum matches
- `Pending` – not yet applied
- `Failed` – applied but execution failed (can be repaired)
- `Out of Order` – applied but has a version lower than the current one
- `Missing` – was applied but the script no longer exists
- `Outdated` – repeatable migration that is out of date (checksum mismatch)

This is useful for diagnostics and logging.

## Validating Migrations with Validate

The `Validate()` command checks that applied migrations match the resolved scripts. It detects problems such as:
- A checksum mismatch (script modified after application)
- A failed migration
- A missing migration (was applied but script is gone)
- A pending migration (resolved but not yet applied)

```go
result, err := migrator.Validate(ctx)
if err != nil {
	panic(err)
}

if !result.Valid {
	for _, e := range result.Errors {
		println("Error:", e.Message)
	}
	panic("validation failed")
}

println("All", result.ValidationCount, "migrations are valid")
```

Validation does not require any database changes and can be run repeatedly. When validation fails, the returned error wraps `goway.ErrValidationFailed`; you can still read the result to inspect the specific problems.

:::caution
Do not modify migration scripts after they have been applied. Doing so will cause checksum validation to fail. If you need to make a change after application, create a new migration (preferably a repeatable one for idempotent changes).
:::

## Configuration Reference

For a complete list of configuration options and their defaults, see the [Configuration Reference](/goway/reference/configuration/).

For detailed command signatures and result types, see the [Commands Reference](/goway/reference/commands/).

## Advanced Topics

- **Embedding migrations**: See [Embedding Migrations](/goway/cookbook/embedding-migrations/) for patterns using `go:embed` and `FS()`.
- **Writing migrations**: See [Writing Migrations](/goway/guides/writing-migrations/) for migration naming, syntax, and repeatable patterns.
- **Non-transactional scripts**: Use `-- goway:noTransaction` to run statements like PostgreSQL's `CREATE INDEX CONCURRENTLY` outside a transaction.
- **Placeholders**: Customize placeholder delimiters with `PlaceholderPrefix()` and `PlaceholderSuffix()`, and supply values with `Placeholders()`.
- **Callbacks**: Use `Callbacks()` to register programmatic hooks (beforeMigrate, afterMigrate, etc.), or place SQL scripts in locations named `beforeMigrate.sql`, `afterMigrate.sql`, etc.
