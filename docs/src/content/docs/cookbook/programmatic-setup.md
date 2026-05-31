---
title: Programmatic Setup
description: Run migrations at application startup, including per-module schemas.
---

When you need goway to run migrations as part of your application's initialization—rather than as a separate CLI step—integrate it directly into your startup code. This is common in microservices, modular monoliths, and any server that manages its own schema lifecycle.

## Basic Setup

The simplest pattern is to open your database connection and then call `Configure()`, chain the necessary setters, call `Load()`, and finally call `Migrate()` before your app starts serving requests.

```go
package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/cgardev/goway"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Open the connection pool
	db, err := sql.Open("pgx", "postgresql://user:pass@localhost/mydb")
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	// Configure and run migrations
	migrator, err := goway.Configure().
		DataSource(db).
		Locations("db/migrations").
		Load()
	if err != nil {
		log.Fatalf("loading migrations: %v", err)
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatalf("migrating: %v", err)
	}

	log.Printf("migrated to version %s (%d migrations applied)", 
		result.TargetSchemaVersion, result.MigrationsExecuted)

	// Now start your application
	startServer()
}
```

## Multiple Modules with Per-Schema Setup

In a larger application, you may have multiple features or services, each with its own schema and migration location. Run migrations for each module in sequence:

```go
package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/cgardev/goway"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func migrateModule(db *sql.DB, schema, location string) error {
	migrator, err := goway.Configure().
		DataSource(db).
		Schemas(schema).
		Locations(location).
		Load()
	if err != nil {
		return err
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		return err
	}

	log.Printf("%s: version %s (%d migrations)", 
		schema, result.TargetSchemaVersion, result.MigrationsExecuted)
	return nil
}

func main() {
	db, err := sql.Open("pgx", "postgresql://user:pass@localhost/platform")
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	// Migrate each module's schema in order
	modules := []struct {
		schema   string
		location string
	}{
		{"users", "modules/users/migrations"},
		{"auth", "modules/auth/migrations"},
		{"billing", "modules/billing/migrations"},
	}

	for _, m := range modules {
		if err := migrateModule(db, m.schema, m.location); err != nil {
			log.Fatalf("migrating %s: %v", m.schema, err)
		}
	}

	startServer()
}
```

:::note
When you pass a schema to `Schemas()`, goway automatically creates it if it does not exist (controlled by `CreateSchemas()`, which defaults to true). The schema history table is created in that schema.
:::

## Understanding CreateSchemas and DefaultSchema

**CreateSchemas** controls whether goway automatically creates missing schemas. It defaults to `true`—a safe choice for application startup where you want the schema to exist before your application runs.

**DefaultSchema** explicitly specifies which schema holds the schema history table. If not set, goway uses:
1. The first schema passed to `Schemas()`, if any
2. The database's current schema (e.g., `public` on PostgreSQL)

In most cases, you do not need to set `DefaultSchema`. Use `Schemas()` to declare which schemas the migrator manages, and the first one becomes the default:

```go
// Schema history lives in "core"; migrations also create "users" and "orders"
goway.Configure().
	DataSource(db).
	Schemas("core", "users", "orders").
	Locations("db/migrations").
	Load()
```

If you want the history table in a different schema than the one in `Schemas()`, set it explicitly:

```go
// History table is in "system"; migrations manage "app" and "analytics"
goway.Configure().
	DataSource(db).
	Schemas("app", "analytics").
	DefaultSchema("system").
	Locations("db/migrations").
	Load()
```

## Structured Helper

For cleaner, more testable startup code, wrap the configuration into a helper function:

```go
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cgardev/goway"
)

// RunMigrations runs all pending migrations and returns the outcome.
// If migration fails, the error is returned and the application should not start.
func RunMigrations(ctx context.Context, db *sql.DB, config MigrationConfig) error {
	migrator, err := goway.Configure().
		DataSource(db).
		Schemas(config.Schemas...).
		DefaultSchema(config.DefaultSchema).
		Locations(config.Locations...).
		CreateSchemas(config.CreateSchemas).
		ValidateOnMigrate(config.ValidateOnMigrate).
		Load()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	result, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if result.MigrationsExecuted > 0 {
		fmt.Printf("Applied %d migrations, now at version %s\n",
			result.MigrationsExecuted, result.TargetSchemaVersion)
	} else {
		fmt.Println("Schema is up to date")
	}
	return nil
}

// MigrationConfig holds the settings for a migration run.
type MigrationConfig struct {
	Schemas            []string // Required: schemas to manage
	DefaultSchema      string   // Optional: where the history table lives
	Locations          []string // Optional: defaults to ["db/migration"]
	CreateSchemas      bool     // Optional: defaults to true
	ValidateOnMigrate  bool     // Optional: defaults to true
}
```

Then in your `main()`:

```go
func main() {
	db, err := sql.Open("pgx", "postgresql://user:pass@localhost/mydb")
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	err = database.RunMigrations(context.Background(), db, database.MigrationConfig{
		Schemas:   []string{"public"},
		Locations: []string{"db/migrations"},
	})
	if err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	startServer()
}
```

## Error Handling

Migrations can fail for several reasons: invalid SQL, checksum mismatches, missing files, or schema creation issues. Always treat migration errors as fatal and halt startup:

```go
result, err := migrator.Migrate(ctx)
if err != nil {
	if errors.Is(err, goway.ErrValidationFailed) {
		// A migration's checksum does not match its history record
		log.Fatalf("schema is inconsistent; run goway repair: %v", err)
	}
	if errors.Is(err, goway.ErrFailedMigration) {
		// A previous migration failed and is still in the history
		log.Fatalf("failed migration must be repaired before proceeding: %v", err)
	}
	log.Fatalf("migration failed: %v", err)
}
```

See the [Commands](/goway/reference/commands/) page for recovery strategies using `Validate()` and `Repair()`.

## Transactions and Isolation

Each migration runs inside its own transaction by default, so a failure rolls back all its changes and leaves the database in a consistent state. The migration can then be corrected and retried on the next startup.

Some statements (such as PostgreSQL's `CREATE INDEX CONCURRENTLY` or SQLite's `VACUUM`) cannot run inside a transaction. Mark these migrations with a comment at the top of the script:

```sql
-- goway:noTransaction

CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

When a non-transactional migration fails, a failed history entry is still recorded so that you can inspect and repair it with the `Repair` command.

:::caution
Avoid mixing transactional and non-transactional statements in a single script. Separate them into distinct migration files.
:::

## Context and Timeout

Pass a context with a suitable timeout to `Migrate()` to prevent migrations from hanging indefinitely on a slow database or network:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := migrator.Migrate(ctx)
```

On context cancellation or timeout, any in-flight migration is rolled back (if it was transactional) and an error is returned.
