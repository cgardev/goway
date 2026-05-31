---
title: Callbacks
description: Run logic before and after migrations using SQL scripts or Go callbacks.
---

Callbacks let you hook into the migration lifecycle to run custom logic at key points. You can run SQL statements via callback scripts discovered by filename, or register programmatic callbacks in Go to execute arbitrary logic.

## Lifecycle Events

There are four callback events, fired at specific points during a `Migrate` run:

- **beforeMigrate**: fires once before any migration is applied
- **afterMigrate**: fires once after all migrations have been applied
- **beforeEachMigrate**: fires before each individual migration
- **afterEachMigrate**: fires after each individual migration

The per-migration events (`beforeEachMigrate`, `afterEachMigrate`) run on the same transaction or executor as the migration itself, so they can participate in the migration's atomicity. The run-level events (`beforeMigrate`, `afterMigrate`) receive the database connection pool.

## SQL Callback Scripts

Callback scripts are SQL files discovered in the configured `Locations` by their filename. A callback filename must match an event name, optionally followed by the `SQLMigrationSeparator` and a description.

### Naming

The format is `<event>[__<description>].sql`, where `<event>` is case-insensitive:

- `beforeMigrate.sql` — runs before any migration
- `afterMigrate.sql` — runs after all migrations
- `beforeEachMigrate.sql` — runs before each migration
- `beforeEachMigrate__pre_checks.sql` — before each migration, with a description
- `afterEachMigrate__cleanup.sql` — after each migration, with a description

The separator is configurable via `Configure().SQLMigrationSeparator(...)` (default: `"__"`).

:::tip
A callback script can contain multiple SQL statements. Like migrations, statements are split on statement delimiters appropriate to your dialect (semicolons in PostgreSQL; also understanding SQLite's `BEGIN`, `CASE`, and `END` blocks).
:::

### Placeholders

Callback scripts support [placeholders](/goway/reference/placeholders/) just like migrations. Built-in placeholders like `${flyway:defaultSchema}` and `${flyway:user}` are always available.

## Programmatic Callbacks

For complex logic, register callbacks in Go by implementing the `Callback` interface or using the `CallbackFunc` adapter:

```go
type Callback interface {
	Handle(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error
}
```

- `event` identifies which lifecycle event is firing (`EventBeforeMigrate`, `EventAfterMigrate`, `EventBeforeEachMigrate`, `EventAfterEachMigrate`)
- `exec` is the executor to run SQL on: a `*sql.DB` for run-level events, or a `*sql.Tx` for per-migration events
- `migration` describes the migration being processed; it is `nil` for run-level events

The `Execer` interface is minimal — it is satisfied by `*sql.DB`, `*sql.Tx`, and `*sql.Conn`:

```go
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

### CallbackFunc Adapter

`CallbackFunc` is a function type that adapts a closure to the `Callback` interface:

```go
type CallbackFunc func(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error
```

This lets you pass a function directly to `Configure().Callbacks()` without defining a type.

### Example

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/cgardev/goway"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, _ := sql.Open("pgx", "postgresql://localhost/mydb")
	defer db.Close()

	beforeEach := goway.CallbackFunc(func(ctx context.Context, event goway.CallbackEvent, exec goway.Execer, migration *goway.MigrationInfo) error {
		if event != goway.EventBeforeEachMigrate {
			return nil
		}
		fmt.Printf("Applying %s: %s\n", migration.Version, migration.Description)
		return nil
	})

	afterEach := goway.CallbackFunc(func(ctx context.Context, event goway.CallbackEvent, exec goway.Execer, migration *goway.MigrationInfo) error {
		if event != goway.EventAfterEachMigrate {
			return nil
		}
		fmt.Printf("Applied %s in %dms\n", migration.Version, migration.ExecutionTime)
		return nil
	})

	migrator, _ := goway.Configure().
		DataSource(db).
		Locations("db/migrations").
		Callbacks(beforeEach, afterEach).
		Load()

	result, _ := migrator.Migrate(context.Background())
	fmt.Printf("Executed %d migrations\n", result.MigrationsExecuted)
}
```

## Order of Execution

During `Migrate`, callbacks fire in this order:

1. Run-level `beforeMigrate` callbacks (SQL scripts, then programmatic)
2. For each pending migration:
   - Per-migration `beforeEachMigrate` callbacks (SQL scripts, then programmatic)
   - Migration statements
   - Per-migration `afterEachMigrate` callbacks (SQL scripts, then programmatic)
   - History record inserted
3. Run-level `afterMigrate` callbacks (SQL scripts, then programmatic)

SQL callback scripts are executed first, followed by programmatic callbacks registered via `Callbacks()`.

## Transaction Scope

- **Run-level events** (`beforeMigrate`, `afterMigrate`) receive the database connection pool (`*sql.DB`) and can manage their own transactions.
- **Per-migration events** (`beforeEachMigrate`, `afterEachMigrate`) receive the same transaction as the migration (when transactional), so they participate in the migration's atomicity. If a callback returns an error, the transaction rolls back and the migration fails.
- **Non-transactional migrations** (those with `-- goway:noTransaction` or `-- flyway:executeInTransaction=false`) receive a dedicated connection instead.

:::caution
If a callback in a per-migration event returns an error, the entire migration is rolled back and marked failed in the history. The error is returned to the caller.
:::

## Error Handling

If any callback returns an error, the migration run stops immediately and the error is returned to the caller. The history table records the state of migrations executed up to that point.

For SQL callback scripts, if a statement fails to parse or execute, the error includes the callback filename and is wrapped with context.

## See Also

- [Callbacks Reference](/goway/reference/callbacks/) for the complete type documentation
- [Writing Migrations](/goway/guides/writing-migrations/) for migration syntax and non-transactional migrations
