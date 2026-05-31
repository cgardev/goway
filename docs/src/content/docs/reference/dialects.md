---
title: Dialects
description: The Dialect abstraction, Postgres() and SQLite(), detection, and per-dialect behavior.
---

A `Dialect` abstracts the differences between supported databases, allowing goway to work with multiple systems while handling database-specific SQL syntax, naming conventions, and features.

The `Dialect` interface is sealed — it has no exported methods — so you obtain a dialect only through the constructor functions `Postgres()` and `SQLite()`. This ensures type safety and prevents accidental misuse.

## Creating a Dialect

```go
import "github.com/cgardev/goway"

// Explicitly select PostgreSQL
dialect := goway.Postgres()

// Explicitly select SQLite
dialect := goway.SQLite()
```

In most cases, you do not need to specify a dialect explicitly. When you call `Configure().Load()`, goway detects the dialect automatically by attempting to query `sqlite_version()` (unique to SQLite) and then `version()` (PostgreSQL). If neither query succeeds, `Load` returns `ErrNoDialect`; if a `version()` succeeds but does not identify PostgreSQL, it returns `ErrUnsupportedDialect`.

You can override auto-detection with `Configure().Dialect(dialect)` if needed.

## Retrieving the Active Dialect

Once a `Migrator` is loaded, you can query which dialect is in use:

```go
migrator, err := goway.Configure().
    DataSource(db).
    Load(ctx)
if err != nil {
    return err
}

name := migrator.Dialect().Name() // "postgresql" or "sqlite"
```

## Supported Databases

Goway targets the **two most recent major versions** of each database:

- **PostgreSQL**: Current and previous major version (e.g., 16 and 15)
- **SQLite**: Current and previous major version (e.g., 3.46 and 3.45)

:::note
Older versions may work, but compatibility is not guaranteed.
:::

## Capabilities Matrix

| Feature | PostgreSQL | SQLite |
|---------|-----------|--------|
| Addressable schemas | Yes | No |
| User identification | Yes | No |
| Advisory locks | Yes | No |
| Dollar-quoted string literals | Yes | No |
| BEGIN/CASE/END trigger blocks | No | Yes |
| Backtick and bracket identifiers | No | Yes |
| Multi-writer concurrency | Yes | Single writer |

## PostgreSQL Dialect

### Driver

Use the pgx driver via `jackc/pgx/v5`:

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

db, err := sql.Open("pgx", "postgresql://user:password@localhost/dbname")
```

### Features

**Schemas**

PostgreSQL has addressable schemas. When you configure a default schema with [`DefaultSchema(name)`](/goway/reference/configuration/), goway creates it if it does not exist (controlled by [`CreateSchemas(bool)`](/goway/reference/configuration/)). The history table is placed in the default schema.

**Search Path**

For each transaction, goway executes `SET LOCAL search_path TO <schema>`, which makes the specified schema the default for unqualified object names within that transaction only. Non-transactional migrations (those marked with `-- goway:noTransaction`) use `SET search_path TO <schema>` for the whole session.

**Advisory Lock**

Goway uses PostgreSQL's session advisory lock mechanism to prevent concurrent migrations. A single lock is held for the duration of the migration run.

**Dollar-Quoted Strings**

PostgreSQL's dollar-quoting syntax allows you to write functions and other bodies without escaping embedded quotes:

```sql
-- V2__create_function.sql
CREATE FUNCTION add(a INT, b INT) RETURNS INT AS $$
BEGIN
    RETURN a + b;
END;
$$ LANGUAGE plpgsql;
```

The goway statement splitter understands dollar-quoted blocks and does not split within them.

## SQLite Dialect

### Driver

Use the modernc.org/sqlite pure-Go driver:

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

db, err := sql.Open("sqlite", "file:mydb.sqlite?cache=shared")
```

### Features

**No Schemas**

SQLite does not support addressable schemas. All tables exist in the main database. Configuration calls like `DefaultSchema()` and `Schemas()` have no effect. The history table is created in the main database.

**Single Writer**

SQLite enforces a single writer at a time. Concurrent migration runs from multiple processes will serialize and may timeout if locks are held too long.

**Trigger Block Parsing**

The statement splitter understands SQLite trigger block syntax and does not split on `BEGIN`, `CASE`, or `END` keywords when they appear inside trigger definitions:

```sql
-- V3__create_trigger.sql
CREATE TRIGGER update_timestamp
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;
```

**Quoted Identifiers**

SQLite accepts double quotes (standard SQL), backticks (MySQL compatibility), and square brackets (T-SQL compatibility):

```sql
-- All three are valid in SQLite:
CREATE TABLE "users" (id INT);
CREATE TABLE `users` (id INT);
CREATE TABLE [users] (id INT);
```

Goway uses double quotes for generated SQL and accepts all three forms in migrations.

## Error Handling

If goway cannot detect a dialect, it returns `ErrNoDialect`:

```go
if errors.Is(err, goway.ErrNoDialect) {
    // Neither SQLite nor PostgreSQL detected; connection or query failed.
}
```

If auto-detection succeeds but identifies an unsupported database, it returns `ErrUnsupportedDialect`:

```go
if errors.Is(err, goway.ErrUnsupportedDialect) {
    // A database responded but was not PostgreSQL or SQLite.
}
```

Use `errors.Is()` rather than direct equality, since these errors are wrapped with context.

## Next Steps

- [Configuration](/goway/reference/configuration/) — Learn how to set default schemas, locations, placeholders, and other options.
- [Commands](/goway/reference/commands/) — Discover what each command does and what it returns.
- [Writing Migrations](/goway/guides/writing-migrations/) — See how to name and write migration scripts.
