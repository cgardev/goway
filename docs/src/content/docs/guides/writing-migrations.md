---
title: Writing Migrations
description: The naming convention, versioned vs repeatable migrations, multi-statement scripts, and checksums.
---

## File Naming Convention

Migration files follow a strict naming convention to be recognized and ordered correctly. The format is:

```
<prefix><version><separator><description><suffix>
```

**Versioned migrations** (run once):
```
V<version>__<description>.sql
```

**Repeatable migrations** (re-run when checksum changes):
```
R__<description>.sql
```

### Default Values

| Component | Default | Configurable |
|-----------|---------|--------------|
| Versioned prefix | `V` | `SQLMigrationPrefix()` |
| Repeatable prefix | `R` | `RepeatableSQLMigrationPrefix()` |
| Separator | `__` (double underscore) | `SQLMigrationSeparator()` |
| Suffix | `.sql` | `SQLMigrationSuffixes()` |

### Naming Examples

| Filename | Type | Version | Description |
|----------|------|---------|-------------|
| `V1__init.sql` | Versioned | `1` | init |
| `V1.1__add_users_table.sql` | Versioned | `1.1` | add users table |
| `V2_0_1__create_index.sql` | Versioned | `2.0.1` | create index |
| `R__refresh_materialized_view.sql` | Repeatable | (none) | refresh materialized view |
| `R__latest_countries.sql` | Repeatable | (none) | latest countries |

Underscores in the description are converted to spaces in the migration history table. Underscores in the version are converted to dots for parsing purposes, allowing file systems that dislike dots to use underscores instead (e.g., `V2_0_1` parses as version `2.0.1`).

## Version Numbering

Versions are sequences of non-negative integer components separated by dots or underscores. Examples:

- `1` — a single component
- `1.1` — two components
- `2.0.1` — three components
- `20260531` — a date-based version (arbitrary precision)

### Version Comparison and Equality

Versions are compared component-by-component, with missing trailing components treated as zero. This means:

- `1` equals `1.0` equals `1.0.0`
- `2` is less than `2.1` (the second component defaults to zero in the first version)
- `10` is greater than `2` (numeric comparison, not string-based)

Trailing zeros are automatically removed from the internal representation, so duplicate versions using different forms (e.g., `1` and `1.0`) are correctly detected and rejected.

## Versioned vs Repeatable Migrations

### Versioned Migrations

Versioned migrations run **once**, in ascending version order. Use them for:

- Creating tables
- Adding columns or indexes
- One-off data transformations
- Any schema change that should be applied exactly once

Once a versioned migration is executed, it is never run again, even if you modify the file.

### Repeatable Migrations

Repeatable migrations run **every time** their checksum changes, after all versioned migrations. Use them for:

- Views and materialized views
- Functions and stored procedures
- Refresh operations
- Idempotent utility scripts

Repeatable migrations are sorted by description (alphabetically) and are applied in that order. If you change the content of a repeatable migration, the system will detect the checksum change and re-apply it on the next migration run.

:::note
Both versioned and repeatable migrations must follow the naming convention and file suffix rules. Files that do not match are ignored (unless they are callback scripts).
:::

## Multi-Statement Scripts

A migration script can contain multiple SQL statements separated by semicolons. The parser understands SQL syntax and will not incorrectly split on semicolons that appear inside strings, comments, or other constructs.

### Syntax Awareness

The parser respects:

- **Single-quoted strings** — e.g., `'O''Brien'` (doubled quotes are escaped)
- **Double-quoted identifiers** — e.g., `"Column Name"`
- **Line comments** — `-- comment text` (to end of line)
- **Block comments** — `/* nested /* comments */ supported */`
- **PostgreSQL dollar-quoted strings** — e.g., `$$function body$$` or `$fn$body$fn$`
  - Dollar quotes can contain an optional identifier: `$function$...$function$`
  - The identifier must start with a letter or underscore
- **PostgreSQL escape strings** — e.g., `E'line\nbreak'`
- **SQLite backtick identifiers** — e.g., `` `Column` ``
- **SQLite bracket identifiers** — e.g., `[Column Name]`
- **SQLite trigger blocks** — `BEGIN...END` blocks in CREATE TRIGGER statements are treated as nested, so their inner semicolons do not split statements
- **SQLite CASE expressions** — `CASE...END` blocks are nested (do not split on their inner semicolons)

### Statement Extraction

After splitting, empty statements (containing only whitespace) are discarded. Each non-empty statement is trimmed and executed separately. Comments and whitespace within statements are preserved.

### Example Multi-Statement Script

```sql
-- Initialize the schema
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- Add an index
CREATE INDEX idx_users_name ON users(name);

-- Insert initial data
INSERT INTO users (name) VALUES
    ('Alice'),
    ('Bob'),
    ('Charlie');
```

This script contains three statements: CREATE TABLE, CREATE INDEX, and INSERT.

## Non-Transactional Migrations

By default, each SQL migration runs inside its own database transaction, which is committed if the migration succeeds or rolled back if it fails. Some operations cannot run inside a transaction—for example, PostgreSQL's `CREATE INDEX CONCURRENTLY` or SQLite's `VACUUM`.

To run a migration outside a transaction, add a comment directive at the start of the file:

```sql
-- goway:noTransaction
CREATE INDEX CONCURRENTLY idx_large_table ON large_table(column);
```

The alternative Flyway-compatible form is also accepted:

```sql
-- flyway:executeInTransaction=false
CREATE INDEX CONCURRENTLY idx_large_table ON large_table(column);
```

Non-transactional migrations are executed on a dedicated database connection. If a non-transactional migration fails, a history row is recorded with `success=false`. Use the [Repair](/goway/reference/commands/) command to clear failed migration records before retrying.

:::caution
Non-transactional migrations should be idempotent (safe to retry). Include `IF NOT EXISTS` or similar guards.
:::

For more details and examples, see the [Cookbook](/goway/cookbook/).

## Checksums

Each SQL migration file is assigned a checksum to detect unintended changes. The checksum is computed using the **CRC32 algorithm (IEEE)** over the raw file content.

### How Checksums Are Computed

1. The file content is split into lines (respecting CR, LF, and CRLF line terminators)
2. The UTF-8 byte order mark (BOM), if present on the first line, is stripped
3. The raw bytes of each line (excluding line terminators) are hashed using CRC32
4. The result is interpreted as a signed 32-bit integer

### Checksum Storage and Validation

When a migration is executed, its checksum is stored in the schema history table. On subsequent runs:

- For **versioned migrations**: the stored checksum is ignored; the migration is not re-executed
- For **repeatable migrations**: the current checksum is compared to the stored one; if they differ, the migration is re-executed and the checksum is updated
- During `validate`: checksums are verified; a mismatch indicates the file has been modified since execution

### Changing a Migration File

**Versioned migration:** Do not modify the content of a migration after it has been applied to production. If you need to make a change, create a new versioned migration (with a higher version number) to apply the fix.

**Repeatable migration:** Modifying the file will cause it to be re-executed on the next migration run. Ensure the changes are idempotent—safe to run multiple times.

:::tip
If you accidentally modify a versioned migration and need to recover, use the [Repair](/goway/reference/commands/) command to fix the checksum mismatch. However, the best practice is to create a new migration instead.
:::

---

**Next:** See [Configuration](/goway/guides/configuration/) to set up your migration source locations and database connection.
