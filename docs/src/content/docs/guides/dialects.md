---
title: Dialects
description: How goway supports PostgreSQL and SQLite, and what differs between them.
---

goway supports two databases: PostgreSQL and SQLite. Each has its own dialect implementation that handles differences in SQL syntax, schema handling, and features.

## Dialect auto-detection

When you configure goway with a live database connection, the dialect is detected automatically by attempting to query a database-specific function. SQLite's `sqlite_version()` and PostgreSQL's `version()` functions serve as discriminators:

```go
migrator, err := goway.Configure().
    DataSource(database).
    Load() // dialect auto-detected from the connection
if err != nil {
    return err
}
```

If auto-detection fails, an error is returned. You can specify a dialect explicitly using the `.Dialect()` setter:

```go
migrator, err := goway.Configure().
    DataSource(database).
    Dialect(goway.Postgres()).
    Load()
if err != nil {
    return err
}
```

## Supported versions

goway targets the two most recent major versions of each database:

- **PostgreSQL**: versions 16 and 17
- **SQLite**: versions 3.43 and 3.44

Older versions may work but are not tested or supported.

## Drivers

goway does not bundle any database drivers. Your application must import and register the appropriate driver for the database you use.

### PostgreSQL with pgx

PostgreSQL connections use the [`jackc/pgx/v5`](https://github.com/jackc/pgx) driver:

```go
import _ "github.com/jackc/pgx/v5/stdlib"

// ...

db, err := sql.Open("pgx", "postgresql://user:password@localhost/dbname")
if err != nil {
    return err
}
```

### SQLite with modernc.org/sqlite

SQLite uses the pure Go [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) driver, which requires no C dependencies:

```go
import _ "modernc.org/sqlite"

// ...

db, err := sql.Open("sqlite", "path/to/database.db")
if err != nil {
    return err
}
```

## Feature comparison

The two dialects differ in several capabilities:

| Feature | PostgreSQL | SQLite |
|---------|-----------|--------|
| Addressable schemas | Yes | No |
| Per-transaction search path | Yes | No |
| Transactional DDL | Yes | Yes |
| Dollar-quoted string bodies | Yes | No |
| Trigger block splitting | No | Yes |
| Advisory migration lock | Yes | No |

### Schemas

PostgreSQL supports addressable schemas, allowing you to run migrations against multiple named schemas in the same database. SQLite has no concept of schemas and addresses all tables within a single attached database.

When using PostgreSQL, you can specify target schemas with the `.Schemas()` configuration method. goway creates these schemas if they do not exist (unless disabled with `.CreateSchemas(false)`) and runs all migrations against them.

### Search path

On PostgreSQL, the default schema for unqualified table and object references can be controlled per transaction or per session. Each migration transaction sets the search path to include the target schema, so references like `CREATE TABLE users` (without a schema qualifier) apply to the correct schema.

SQLite has no equivalent concept, so all table references must be unqualified (the database has only one implicit schema).

### Identifier quoting

Both dialects quote identifiers using double quotes: `"table_name"`, `"column_name"`. This is the standard SQL behavior for both PostgreSQL and SQLite.

### Statement splitting

Migrations are split into individual executable statements. Both dialects understand standard SQL statement terminators and block syntax:

- **PostgreSQL** understands parentheses, `BEGIN`/`END`, `CASE`/`END`, and quoted strings.
- **SQLite** also understands trigger blocks (`BEGIN ... END` inside `CREATE TRIGGER`), backtick identifiers, and `[bracket]` identifiers.

This ensures that complex statements like triggers and stored procedures are correctly parsed as single units, even if they contain semicolons.

### Non-transactional migrations

Some operations cannot run inside a transaction. Prefix your migration with a comment to exempt it:

```sql
-- goway:noTransaction
CREATE INDEX CONCURRENTLY idx_name ON table_name (column);
```

(The Flyway-compatible form `-- flyway:executeInTransaction=false` is also recognized.)

On PostgreSQL, this allows statements like `CREATE INDEX CONCURRENTLY` that require a transaction-free environment. On SQLite, certain operations like `VACUUM` and `PRAGMA` statements require the same treatment.

Non-transactional migrations run on a dedicated connection outside the per-migration transaction. If a non-transactional migration fails, a failed history row is recorded; use the [Repair](/goway/reference/commands/) command to clean it up.

## Single-writer concurrency on SQLite

SQLite enforces single-writer semantics at the database level. Only one connection may write at a time; other connections are blocked until the write completes. This means that concurrent migration attempts will serialize and only the first will make progress. For applications with a single initialization point (typical in Go applications that run migrations on startup), this is not a practical concern.

## Next steps

See the [Configuration reference](/goway/reference/configuration/) for all available options, and [Writing Migrations](/goway/guides/writing-migrations/) for migration naming and syntax.
