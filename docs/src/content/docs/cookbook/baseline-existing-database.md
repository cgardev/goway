---
title: Baselining an Existing Database
description: Bring a database that already has a schema under goway management.
---

When you adopt goway on a database that already has a schema in place, you have two options: run every historical migration from the beginning (which may be slow or risky), or establish a baseline entry at a chosen version and skip past the history.

## What is a baseline?

A baseline is a marker in the schema history table that represents the current state of your database at a specific version. Once a baseline is recorded at, say, version 1, goway ignores all migrations at or below that version and applies only newer ones. This lets you bring an existing production database under migration management without replaying years of history.

## Two approaches

### 1. Explicit baseline (recommended for one-time adoption)

Call `Baseline(ctx)` on your migrator to record a baseline entry with the configured version and description:

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
	db, err := sql.Open("pgx", "postgresql://user:pass@localhost/mydb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator, err := goway.Configure().
		DataSource(db).
		Load(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Record a baseline at version 1
	result, err := migrator.Baseline(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Baseline recorded: version %s, description: %s",
		result.BaselineVersion, result.BaselineDescription)
}
```

After baselining, you may apply migrations at versions 2 and above—or configure a different baseline version if your database is already at a higher evolutionary stage:

```go
migrator, err := goway.Configure().
	DataSource(db).
	BaselineVersion("5").
	BaselineDescription("Adoption on 2026-05-31").
	Load(context.Background())
```

### 2. Automatic baseline (convenient for first-run adoption)

Set `BaselineOnMigrate(true)` to automatically record a baseline the first time `Migrate(ctx)` runs on a schema that has no history table yet:

```go
migrator, err := goway.Configure().
	DataSource(db).
	BaselineOnMigrate(true).
	Load(context.Background())

// On the first migrate call, if the schema exists but the history table
// does not, a baseline entry is inserted automatically.
result, err := migrator.Migrate(context.Background())
```

This is useful in CI/CD when you want the same code path to work both on fresh databases (which skip migrations below the baseline) and on databases being adopted for the first time.

:::note
`BaselineOnMigrate` only triggers if the schema has content but no history table. If a history table already exists, even with failed or pending migrations, `Migrate` proceeds normally without baselining.
:::

## Baseline defaults

If you do not configure baseline settings, goway uses these defaults:

- **BaselineVersion**: `"1"`
- **BaselineDescription**: `"<< Flyway Baseline >>"`

These match Flyway's defaults, ensuring compatibility if you ever migrate a schema between tools.

## Schema history requirements

`Baseline` requires that either:
- The schema history table exists, or
- The target schema exists and can have the table created in it (controlled by `CreateSchemas`)

If the history table does not exist, `Baseline` creates it. If it does exist but contains applied migrations, `Baseline` returns an error—you cannot baseline a schema that has already run migrations.

## Existing Flyway integration

If your database already has a `flyway_schema_history` table from Flyway, goway can adopt it directly. Point goway to the same table name and database, and it will respect the existing history:

```go
migrator, err := goway.Configure().
	DataSource(db).
	Table("flyway_schema_history"). // The table name Flyway created
	Load(context.Background())

result, err := migrator.Migrate(context.Background())
```

goway recognizes and preserves Flyway baseline entries and will skip migrations at or below the recorded baseline version, just as Flyway does. New migrations you apply with goway will follow Flyway's naming scheme and history schema.

:::tip
When migrating from Flyway to goway on an existing database, no changes to the history table are needed. goway uses the same defaults for table name, column schema, and migration naming conventions.
:::

## CLI adoption

From the command line, baseline an existing database with:

```sh
goway -url postgresql://user:pass@localhost/mydb \
      -baseline-version 1 \
      -baseline-description "Adoption on 2026-05-31" \
      baseline
```

Then apply new migrations normally:

```sh
goway -url postgresql://user:pass@localhost/mydb migrate
```

Or enable automatic baselining so the first `migrate` command does both:

```sh
goway -url postgresql://user:pass@localhost/mydb \
      -baseline-on-migrate \
      migrate
```

See [Commands](/goway/reference/commands/) for the complete CLI reference.
