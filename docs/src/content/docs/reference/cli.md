---
title: Command Line Interface
description: "Complete reference for the cmd/goway command line tool: commands, flags, and URLs."
---

The `goway` CLI is a command-line front end for the goway migration library. It connects to a PostgreSQL or SQLite database and runs one of the six migration commands.

## Installation

Build and run the CLI directly:

```sh
go run github.com/cgardev/goway/cmd/goway [flags] <command>
```

Or install it locally:

```sh
go install github.com/cgardev/goway/cmd/goway@latest
goway [flags] <command>
```

## Database URL

The database URL is required and must be provided either via the `-url` flag or the `GOWAY_URL` environment variable.

### Supported Schemes

**PostgreSQL:**
- `postgres://[user[:password]@][host][:port][/dbname][?param=value...]`
- `postgresql://[user[:password]@][host][:port][/dbname][?param=value...]`

Connects via [pgx](https://github.com/jackc/pgx) and requires the stdlib driver:

```go
import _ "github.com/jackc/pgx/v5/stdlib"
```

**SQLite:**
- `sqlite:./path/to/db.sqlite` or `sqlite:db.sqlite`
- `file:/path/to/db.sqlite` (absolute path) or `file:///path/to/db.sqlite`

Connects via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go):

```go
import _ "modernc.org/sqlite"
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-url` | string | `GOWAY_URL` env | Database URL (required). For example: `postgres://localhost/mydb` or `sqlite:./app.db`. |
| `-locations` | string | `filesystem:db/migration` | Comma-separated list of migration locations. Supports `filesystem:` URLs only in the CLI. |
| `-schemas` | string | (empty) | Comma-separated list of schemas. The first schema is treated as the default schema. |
| `-table` | string | `flyway_schema_history` | Name of the schema history table. |
| `-create-schemas` | bool | `true` | Automatically create missing schemas. |
| `-baseline-version` | string | `1` | Version to record when baselining. |
| `-baseline-description` | string | `<< Flyway Baseline >>` | Description to record when baselining. |
| `-baseline-on-migrate` | bool | `false` | Automatically baseline on the first migrate if no history exists. |
| `-out-of-order` | bool | `false` | Allow migrations to be applied out of order. |
| `-clean-disabled` | bool | `true` | Disable the `clean` command. Set to `false` to enable. |
| `-placeholders` | string | (empty) | Comma-separated key=value pairs for placeholder substitution (e.g., `env=prod,region=us-east-1`). |
| `-target` | string | (empty) | Highest version to apply. Stops migration at this version. |

## Commands

Exactly one command is required.

### migrate

Apply all pending versioned migrations in order, then repeatable migrations if any have changed checksums.

```sh
goway -url postgres://localhost/mydb migrate
```

Output on success (when migrations were applied):

```
Successfully applied 3 migration(s) to version 2.1:
  1          initial schema
  2          add users table
  2.1        add index on user_id
```

Output when already up to date:

```
Schema is up to date. No migration necessary. (version 2.1)
```

### info

Display the migration history and current schema version in a table.

```sh
goway -url postgres://localhost/mydb info
```

Output:

```
Version  Description    Type   State    Installed On
1        initial schema SQL    Success  2026-05-31 10:15:42
2        add users      SQL    Success  2026-05-31 10:16:05
2.1      add index      SQL    Success  2026-05-31 10:16:15
R        seed data      SQL    Success  2026-05-31 10:16:20

Current version: 2.1
```

### validate

Verify that all recorded migrations match their checksum and are in valid order.

```sh
goway -url postgres://localhost/mydb validate
```

Output on success:

```
Validation successful: 4 migration(s) validated.
```

Output on failure (written to stderr):

```
Validation failed:
  1: Checksum mismatch for V1__initial_schema.sql
  2: Migration V1.5__alter_table.sql not found
```

### baseline

Record the current schema version without applying any migrations. Useful when adopting goway on an existing database.

```sh
goway -url postgres://localhost/mydb baseline
```

Output:

```
Successfully baselined schema at version 1.
```

If already baselined:

```
Schema is already baselined at version 1.
```

### repair

Remove failed migration entries from the history and update checksums of successful migrations to match their current content.

```sh
goway -url postgres://localhost/mydb repair
```

Output:

```
Repair complete: removed 1 failed entry(ies), realigned 2 checksum(s).
```

### clean

Drop all database objects in all configured schemas. **This command is disabled by default** for safety. Enable with `-clean-disabled=false`.

```sh
goway -url postgres://localhost/mydb -clean-disabled=false clean
```

Output:

```
Successfully cleaned schema(s): public, audit.
```

## Examples

### PostgreSQL with Default Settings

```sh
goway -url postgres://user:password@localhost:5432/myapp migrate
```

### PostgreSQL with Custom Schema and Placeholders

```sh
goway \
  -url postgres://localhost/myapp \
  -schemas app_schema,app_audit \
  -placeholders "env=production,region=us-east-1" \
  migrate
```

### SQLite with Target Version

Apply migrations only up to version 1.2, leaving later migrations pending:

```sh
goway -url sqlite:./app.db -target 1.2 migrate
```

### SQLite with Out-of-Order Support

Enable out-of-order migration execution (useful in development):

```sh
goway -url sqlite:./app.db -out-of-order=true migrate
```

### Using Environment Variables

```sh
export GOWAY_URL=postgres://localhost/mydb
goway migrate
goway info
goway validate
```

### Custom History Table

```sh
goway \
  -url postgres://localhost/mydb \
  -table schema_versions \
  migrate
```

### Baseline an Existing Database

Record the current state at version 2.0 without running any migrations:

```sh
goway \
  -url postgres://localhost/mydb \
  -baseline-version 2.0 \
  -baseline-description "Production cutover" \
  baseline
```

## Error Codes

The CLI exits with code 1 on any error. Error messages are printed to stderr, structured as:

```
Error: <reason>
```

Common error conditions:
- Missing or invalid database URL
- Unsupported URL scheme (only `postgres://`, `postgresql://`, `sqlite:`, `file:`)
- No command specified or unknown command
- Validation failures
- Database connection errors
- Migration execution failures

## Return Values

| Command | Success | Returns |
|---------|---------|---------|
| `migrate` | Migrations applied or schema already up to date | 0 |
| `info` | Prints migration history and current version | 0 |
| `validate` | All migrations valid | 0 |
| `baseline` | Schema baselined or already baselined | 0 |
| `repair` | Repairs complete (may be 0 repairs) | 0 |
| `clean` | Schemas cleaned | 0 |
| Any error | Error message to stderr | 1 |

:::tip
The CLI automatically detects the database dialect from the URL scheme, but you can use the Go API if you need to override or detect the dialect programmatically. See the [API reference](/goway/reference/) for details.
:::
