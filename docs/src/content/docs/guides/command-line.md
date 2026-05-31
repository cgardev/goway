---
title: Command Line
description: Run migrations from the goway CLI instead of from your application.
---

The `goway` CLI provides a command-line interface to run migrations without embedding the tool in your application. This is useful for standalone deployments, CI/CD pipelines, and development workflows.

## Installation

Install the CLI by running it directly with `go run`:

```sh
go run github.com/cgardev/goway/cmd/goway -help
```

Or build it into a binary:

```sh
go build github.com/cgardev/goway/cmd/goway
./goway -help
```

## Database Connection

Every command requires a database connection specified via the `-url` flag or the `GOWAY_URL` environment variable. If both are set, `-url` takes precedence.

### URL Schemes

**PostgreSQL:**
- `postgres://user:password@host:5432/database`
- `postgresql://user:password@host:5432/database`

Both schemes are supported. The driver is [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx).

**SQLite:**
- `sqlite:./path/to/database.db`
- `file:./path/to/database.db`

Both schemes work with the pure-Go [`modernc.org/sqlite`](https://modernc.org/sqlite) driver. Relative paths are resolved from the current working directory.

### Example

```sh
# Using a flag
goway -url "postgres://localhost/mydb" migrate

# Using an environment variable
export GOWAY_URL="sqlite:./app.db"
goway migrate
```

## Common Flags

The most frequently used flags are:

**`-locations` (default: `filesystem:db/migration`)**
Comma-separated list of migration directories or embedded filesystem paths. Use `filesystem:` prefix for filesystem locations.

```sh
goway -url postgres://localhost/mydb \
  -locations "filesystem:sql/migrations, filesystem:sql/repeatable" \
  migrate
```

**`-schemas`**
Comma-separated list of schemas to use. The first schema is the default. Relevant for PostgreSQL; ignored for SQLite.

```sh
goway -url postgres://localhost/mydb \
  -schemas "public, audit" \
  migrate
```

**`-table` (default: `flyway_schema_history`)**
Name of the migration history table.

```sh
goway -url postgres://localhost/mydb \
  -table "schema_migrations" \
  migrate
```

**`-create-schemas` (default: `true`)**
Automatically create missing schemas on migration.

```sh
goway -url postgres://localhost/mydb \
  -create-schemas=false \
  migrate
```

**`-target`**
Apply migrations only up to a specific version. Useful for rolling back to a known state (though note: goway does not support downmigrations; this stops at a particular version).

```sh
goway -url postgres://localhost/mydb \
  -target "2.5" \
  migrate
```

**`-placeholders`**
Comma-separated `key=value` pairs for placeholder substitution in migration scripts.

```sh
goway -url postgres://localhost/mydb \
  -placeholders "env=production, app_schema=app_data" \
  migrate
```

For a complete list of flags, run `goway -help`. See the [CLI reference](/goway/reference/cli/) for details on baseline, repair, and clean options.

## Commands

Every invocation takes exactly one command:

### migrate

Apply all pending migrations and move the schema to the target version.

```sh
goway -url postgres://localhost/mydb migrate
```

Output on success:
```
Successfully applied 3 migration(s) to version 2.1:
  1                init
  1.1              add_users_table
  2.1              add_index
```

If the schema is already up to date:
```
Schema is up to date. No migration necessary. (version 2.1)
```

#### Baseline on Migrate

Use `-baseline-on-migrate` to automatically baseline an existing schema on the first run, then apply new migrations. This is useful when adopting goway on a project with existing schema.

```sh
goway -url postgres://localhost/mydb \
  -baseline-on-migrate \
  migrate
```

:::note
This flag is a no-op if a baseline entry already exists in the history table.
:::

### info

Display the current migration status and history. Shows all applied, pending, and failed migrations.

```sh
goway -url postgres://localhost/mydb info
```

Output:
```
Version  Description      Type   State      Installed On
1        init             SQL    Success    2024-12-15 14:23:01
1.1      add_users_table  SQL    Success    2024-12-15 14:23:02
2        add_posts_table  SQL    Pending
<none>   cleanup          SQL    Pending

Current version: 1.1
```

Columns:
- **Version**: Migration version number; `<none>` for repeatable migrations.
- **Description**: Human-readable migration name.
- **Type**: `SQL`, `BASELINE`, `SCHEMA`, or `DELETED`.
- **State**: `Success`, `Pending`, `Failed`, `Out of Order`, `Missing`, or other [migration states](/goway/reference/schema-history/).
- **Installed On**: When the migration ran; empty if pending.

### validate

Check the integrity of applied migrations. Validates that migration checksums match the recorded values and that there are no gaps or conflicts.

```sh
goway -url postgres://localhost/mydb validate
```

Output on success:
```
Validation successful: 5 migration(s) validated.
```

On failure, lists each problem:
```
Validation failed:
  1.1: Checksum mismatch (expected 123456, got 654321)
  2.0: Missing migration (not found on disk)
```

:::caution
Validation is enabled by default during migrate. Use this command to validate without applying migrations.
:::

### baseline

Record the current schema state as a baseline without applying any migration. Use this when adopting goway on a project with existing database schema.

```sh
goway -url postgres://localhost/mydb baseline
```

Output:
```
Successfully baselined schema at version 1.
```

If a baseline already exists:
```
Schema is already baselined at version 1.
```

Options:
- `-baseline-version` (default: `1`): Version to record.
- `-baseline-description` (default: `<< Flyway Baseline >>`): Description text.

```sh
goway -url postgres://localhost/mydb \
  -baseline-version "2024.1" \
  -baseline-description "Legacy schema snapshot" \
  baseline
```

### repair

Remove failed migration entries and re-align migration checksums. Use this after fixing a broken migration script or correcting a corrupted history entry.

```sh
goway -url postgres://localhost/mydb repair
```

Output:
```
Repair complete: removed 1 failed entry(ies), realigned 0 checksum(s).
```

:::caution
Repair modifies the migration history table. Use only when you are confident the database and migration files are in a consistent state.
:::

### clean

Delete all objects (tables, views, indexes, schemas) in the target schemas, leaving the history table intact. **Disabled by default.**

```sh
# Enable with -clean-disabled=false
goway -url postgres://localhost/mydb \
  -clean-disabled=false \
  clean
```

Output:
```
Successfully cleaned schema(s): public, audit.
```

:::caution
Clean is destructive and disabled by default for safety. Do not enable in production. Use only in development or test environments.
:::

## Exit Codes

- `0`: Command succeeded.
- `1`: Command failed (invalid URL, unsupported database, migration error, validation failure, etc.).

## Error Messages

Common errors and how to resolve them:

**`a database URL is required, set -url or GOWAY_URL`**
Provide a database URL via `-url` flag or `GOWAY_URL` environment variable.

**`unsupported database URL; use a postgres:// or sqlite: URL`**
Check the URL format. Only PostgreSQL (`postgres://`, `postgresql://`) and SQLite (`sqlite:`, `file:`) are supported.

**`Validation failed: ... Checksum mismatch`**
A migration file has been modified after execution. Either revert the file or use `repair` to update the recorded checksum.

**`clean-disabled is true; cannot clean`**
Pass `-clean-disabled=false` to enable the clean command. Only do this in development or test environments.

## See Also

- [Writing Migrations](/goway/guides/writing-migrations/)
- [Using Goway in Your Application](/goway/cookbook/embedding-migrations/)
- [Migration States](/goway/reference/schema-history/)
- [Full CLI Reference](/goway/reference/cli/)
