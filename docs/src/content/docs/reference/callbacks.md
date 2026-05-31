---
title: Callbacks
description: The callback events, the Callback interface, CallbackFunc, and SQL callback scripts.
---

Callbacks let you run custom code at specific points during a migrate operation. You can register programmatic callbacks in Go or discover SQL callback scripts in your migration locations.

## Callback Events

Four events fire during a migrate run. Each is identified by a `CallbackEvent` string constant:

| Event | Constant | String Value | Timing |
|-------|----------|--------------|--------|
| Run-start | `EventBeforeMigrate` | `"beforeMigrate"` | Once, before any migration is applied |
| Run-end | `EventAfterMigrate` | `"afterMigrate"` | Once, after all migrations have been applied |
| Per-migration start | `EventBeforeEachMigrate` | `"beforeEachMigrate"` | Before each individual migration |
| Per-migration end | `EventAfterEachMigrate` | `"afterEachMigrate"` | After each individual migration |

The `beforeMigrate` and `afterMigrate` callbacks fire at the run level; `beforeEachMigrate` and `afterEachMigrate` fire within each migration's scope. This distinction affects what database connection and transaction context they receive.

## Event Order

During a `Migrate()` call, events fire in this order:

1. **beforeMigrate** (run-level, receives *sql.DB)
2. For each pending migration:
   - **beforeEachMigrate** (migration-level, on the migration's executor)
   - Migration statements execute
   - **afterEachMigrate** (migration-level, on the migration's executor)
3. **afterMigrate** (run-level, receives *sql.DB)

If any callback returns an error, the migrate stops and the error is returned. For migrations within a transaction, the transaction rolls back; for non-transactional migrations, a failed history row is recorded.

## The Callback Interface

Implement the `Callback` interface to register a programmatic callback:

```go
type Callback interface {
	Handle(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error
}
```

### Handle Signature

- **ctx** (`context.Context`): The context passed to the migrator command (e.g., `Migrate(ctx)`).
- **event** (`CallbackEvent`): The event being fired—one of the four constants above.
- **exec** (`Execer`): The database executor. For `beforeEachMigrate` and `afterEachMigrate`, this is the migration's transaction (or connection for non-transactional migrations). For `beforeMigrate` and `afterMigrate`, this is the `*sql.DB` pool.
- **migration** (`*MigrationInfo`): For per-migration events, a pointer to the `MigrationInfo` of the migration being applied. For run-level events, this is `nil`.

### Execer Interface

The `Execer` interface defines what operations you can perform:

```go
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

It is satisfied by `*sql.DB`, `*sql.Tx`, and `*sql.Conn`. A callback receives the appropriate executor for its event:

- Run-level callbacks (`beforeMigrate`, `afterMigrate`) receive the `*sql.DB` pool.
- Per-migration callbacks (`beforeEachMigrate`, `afterEachMigrate`) receive the executor of the migration's transaction (or dedicated connection if the migration opted out of transactions).

## CallbackFunc

For simple cases, adapt an ordinary function to the `Callback` interface using `CallbackFunc`:

```go
type CallbackFunc func(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error

func (f CallbackFunc) Handle(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error {
	return f(ctx, event, exec, migration)
}
```

This allows you to pass a function directly to `Configure().Callbacks(...)` without defining a type.

## SQL Callback Scripts

In addition to programmatic callbacks, goway discovers SQL callback scripts in your configured migration locations. These are named to match a callback event and a naming pattern.

### Naming Convention

SQL callback scripts are named after the event, optionally followed by the separator and a description:

- **beforeMigrate.sql** – Runs once before any migration.
- **afterMigrate.sql** – Runs once after all migrations.
- **beforeEachMigrate.sql** – Runs before each migration.
- **beforeEachMigrate__<description>.sql** – Per-migration variant with a description (separator configurable).
- **afterEachMigrate.sql** – Runs after each migration.
- **afterEachMigrate__<description>.sql** – Per-migration variant with a description.

The separator is configurable via `Configure().SQLMigrationSeparator(...)` and defaults to `__`.

### Example Script Names

```
beforeMigrate.sql
afterMigrate.sql
beforeEachMigrate.sql
beforeEachMigrate__log_start.sql
afterEachMigrate__log_end.sql
afterEachMigrate__vacuum.sql
```

All matching scripts are discovered and executed in alphabetical order for their event.

### Execution Context

Like programmatic callbacks, SQL scripts run on their event's executor:

- Scripts for `beforeMigrate` and `afterMigrate` run on the `*sql.DB` pool.
- Scripts for `beforeEachMigrate` and `afterEachMigrate` run on the migration's transaction (or connection).

### Placeholder Support

SQL callback scripts support the same [placeholder substitution](/goway/reference/placeholders/) as migration scripts. Built-in placeholders like `${flyway:user}` and `${flyway:defaultSchema}` are always available.

## Registering Callbacks

Pass `Callback` implementations to `Configure().Callbacks(...)`:

```go
m, err := goway.Configure().
	DataSource(db).
	Callbacks(
		myCallback1,
		myCallback2,
		goway.CallbackFunc(func(ctx context.Context, event goway.CallbackEvent, exec goway.Execer, migration *goway.MigrationInfo) error {
			// inline callback logic
			return nil
		}),
	).
	Load()
if err != nil {
	return err
}
```

Callbacks are registered in order and invoked in order for their event. SQL callback scripts are run before programmatic callbacks for the same event.

## Complete Example

Here is a complete example that registers both SQL callback scripts and a programmatic callback:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/cgardev/goway"
)

type MigrationLogger struct {
	logTable string
}

func (l *MigrationLogger) Handle(ctx context.Context, event goway.CallbackEvent, exec goway.Execer, migration *goway.MigrationInfo) error {
	switch event {
	case goway.EventBeforeMigrate:
		return l.logEvent(ctx, exec, "BEFORE_MIGRATE", migration)
	case goway.EventAfterMigrate:
		return l.logEvent(ctx, exec, "AFTER_MIGRATE", migration)
	case goway.EventBeforeEachMigrate:
		return l.logEvent(ctx, exec, "BEFORE_EACH", migration)
	case goway.EventAfterEachMigrate:
		return l.logEvent(ctx, exec, "AFTER_EACH", migration)
	}
	return nil
}

func (l *MigrationLogger) logEvent(ctx context.Context, exec goway.Execer, eventName string, migration *goway.MigrationInfo) error {
	version := "N/A"
	if migration != nil {
		version = migration.Version
	}
	query := fmt.Sprintf(
		"INSERT INTO %s (event, version, timestamp) VALUES ($1, $2, NOW())",
		l.logTable,
	)
	_, err := exec.ExecContext(ctx, query, eventName, version)
	return err
}

func main() {
	db, err := sql.Open("pgx", "postgres://user:pass@localhost/mydb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create the callback logger
	logger := &MigrationLogger{logTable: "migration_log"}

	// Configure and load the migrator with both SQL scripts and a programmatic callback.
	// Assuming your db/migration directory contains:
	//   - beforeMigrate.sql
	//   - afterEachMigrate__log_completion.sql
	m, err := goway.Configure().
		DataSource(db).
		Locations("db/migration").
		Callbacks(logger).
		Load()
	if err != nil {
		log.Fatal(err)
	}

	// Run migrations; all callbacks fire in order
	result, err := m.Migrate(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Applied %d migrations\n", result.MigrationsExecuted)
}
```

In this example:

1. The `beforeMigrate.sql` script (if present) runs once at the start.
2. For each migration, `beforeEachMigrate__*.sql` scripts run, then the migration statements, then `afterEachMigrate__*.sql` scripts.
3. The `MigrationLogger` callback also fires, logging each event to a table.
4. The `afterMigrate.sql` script (if present) runs once at the end.

:::tip
Use per-migration callbacks to validate state, log progress, or clean up temporary objects. Use run-level callbacks to set up and tear down resources that span multiple migrations.
:::

:::note
SQL callback scripts support the same placeholders as migrations and can reference objects in your configured schemas. The `${flyway:filename}` placeholder resolves to the script file name.
:::
