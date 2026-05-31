---
title: Getting Started
description: Install goway, write your first migration, and run it against PostgreSQL or SQLite.
---

## Installation

Add goway to your project:

```sh
go get github.com/cgardev/goway
```

The core library depends only on the standard library. You supply the database driver separately.

## Choose a driver

goway supports PostgreSQL and SQLite. Each has its own driver, and you choose the one your application already owns.

**PostgreSQL** via pgx (github.com/jackc/pgx/v5):

```go
import _ "github.com/jackc/pgx/v5/stdlib"
// Then: sql.Open("pgx", "postgres://user:password@localhost:5432/dbname")
```

**SQLite** via modernc.org/sqlite (pure Go, no C dependencies):

```go
import _ "modernc.org/sqlite"
// Then: sql.Open("sqlite", "path/to/database.db")
```

Why does goway not depend on a driver? Because your application may already use a driver for other purposes, and you should never import two driver implementations. goway stays small and lets your application decide.

## Write your first migration

Create a directory for migrations:

```sh
mkdir -p db/migration
```

Inside, create a versioned migration file. The naming convention is `V<version>__<description>.sql`. The version can be a single number or dot-separated (e.g., `1`, `1.0`, `2.1`). The description is any slug; underscores become spaces.

Create `db/migration/V1__create_users.sql`:

```sql
CREATE TABLE users (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);
```

Create `db/migration/V2__add_email_to_users.sql`:

```sql
ALTER TABLE users ADD COLUMN email TEXT;

CREATE UNIQUE INDEX idx_users_email ON users (email);
```

## Run migrations

Here is a complete, runnable example for SQLite. It opens an in-memory database, applies the migrations, and prints the result:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/cgardev/goway"

	_ "modernc.org/sqlite"
)

func main() {
	// Open the database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Configure the migrator
	migrator, err := goway.Configure().
		DataSource(db).
		Locations("filesystem:db/migration").
		Load()
	if err != nil {
		log.Fatal(err)
	}

	// Run migrations
	ctx := context.Background()
	result, err := migrator.Migrate(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Applied %d migration(s)\n", result.MigrationsExecuted)
	fmt.Printf("Schema is now at version %s\n", result.TargetSchemaVersion)
}
```

For **PostgreSQL**, change only the database opening:

```go
import _ "github.com/jackc/pgx/v5/stdlib"

func main() {
	db, err := sql.Open("pgx", "postgres://user:password@localhost:5432/app")
	// ... rest is identical
}
```

## Understanding the result

`Migrate` returns a `*MigrateResult` with:

- **InitialSchemaVersion** — the version before the run
- **TargetSchemaVersion** — the version after the run
- **MigrationsExecuted** — the number of migrations applied
- **Migrations** — a slice of `MigrationInfo` for each migration that ran, containing the version, description, script name, and execution time

If any migration fails, `Migrate` returns an error and stops. No partial state is left; the transaction containing that migration is rolled back.

## Check migration state

Use `Info` to see all migrations and their status without making changes:

```go
information, err := migrator.Info(ctx)
if err != nil {
	log.Fatal(err)
}

for _, migration := range information.Migrations {
	fmt.Printf("%s: %s\n", migration.Version, migration.State)
}
```

The `State` field tells you whether a migration is "Pending", "Success", "Failed", or another status. The `Current` field on `InfoResult` gives the highest version that has been applied.

## What happens under the hood

- **Auto-detection**: The dialect (PostgreSQL or SQLite) is detected automatically from the connection unless you call `.Dialect()` explicitly.
- **Schema history table**: A table named `flyway_schema_history` is created in the default schema. It records every migration: when it ran, by whom, its checksum, and whether it succeeded.
- **Checksums**: Each migration file is checksummed (CRC32) at load time. If the file changes after being applied, validation will catch it.
- **Transactions**: Each migration runs inside its own transaction, so a failure leaves no partial state.
- **Defaults**: Migrations are discovered in `db/migration` by default; the schema history table is created automatically; schemas are created if they do not exist.

See [Configuration](/goway/reference/configuration/) for all settable options.

## Next steps

- [Writing Migrations](/goway/guides/writing-migrations/) — dive deeper into migration naming, repeatable migrations, and placeholders
- [Commands](/goway/reference/commands/) — migrate, info, validate, baseline, repair, clean
- [Configuration](/goway/reference/configuration/) — all fluent configuration methods and their defaults
