---
title: Non-Transactional Migrations
description: Run statements like CREATE INDEX CONCURRENTLY or VACUUM that cannot run in a transaction.
---

Some SQL statements cannot execute inside a transaction block. PostgreSQL's `CREATE INDEX CONCURRENTLY` allows index creation without locking the table for writes, but requires running outside a transaction. SQLite's `VACUUM` command also demands a transaction-free context. goway handles these with the `goway:noTransaction` directive.

## Marking a Migration

Add a comment line to the top of your migration file containing `goway:noTransaction` (case-insensitive):

```sql
-- goway:noTransaction

CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

The `-- flyway:executeInTransaction=false` form is also accepted for compatibility with Flyway:

```sql
-- flyway:executeInTransaction=false

VACUUM;
```

Detection is case-insensitive and looks only at comment lines, so a marker inside a string literal or statement has no effect. The directive must appear before any SQL statements.

## Execution Model

When goway encounters a non-transactional migration, it:

1. Reads and parses the script normally
2. Acquires a dedicated database connection (not pooled from the normal transaction)
3. Runs all statements on that connection **outside a transaction block**
4. Records the result in the schema history table

This means statements execute with auto-commit enabled, without the safety of rollback.

## Failure Handling

If a non-transactional migration fails, goway records a failed history row. Unlike transactional migrations (which roll back automatically on error), the non-transactional statement may have partially succeeded. Fix the script and then run [`Repair()`](/goway/reference/commands/) to clear the failed entry before retrying:

```go
// After fixing the migration file:
result, err := migrator.Repair(ctx)
if err != nil {
    log.Fatal(err)
}
// RemovedFailed migrations can now be retried
result, err = migrator.Migrate(ctx)
```

:::caution
Non-transactional migrations have no automatic rollback. Ensure your script is correct before executing it, and test thoroughly on a replica or staging database.
:::

## PostgreSQL: Concurrent Indexes

Create indexes without blocking writes:

```sql
-- goway:noTransaction

-- Create the index in the background without blocking INSERT/UPDATE/DELETE on the table
CREATE INDEX CONCURRENTLY idx_users_created_at ON users(created_at);

-- Optionally, run validation or maintenance in the same migration
ANALYZE users;
```

## SQLite: Vacuum and Optimization

Reclaim disk space or rebuild the entire database:

```sql
-- goway:noTransaction

-- Rebuild the database file and reclaim unused space
VACUUM;

-- Optionally optimize the query planner statistics
ANALYZE;
```

:::note
SQLite also supports incremental vacuum (`PRAGMA incremental_vacuum`) inside a transaction, if you want a less disruptive alternative to full vacuum.
:::

## Search Path and Schema Context

On PostgreSQL, goway sets the `search_path` session variable per migration using `SET` rather than a transaction-based approach, ensuring your unqualified objects are placed in the correct schema even outside a transaction.

On SQLite, schema context has no equivalent; goway operates on the default database.

## Placeholders

Placeholder replacement happens at execution time and does not affect the checksum calculation. Non-transactional migrations can use placeholders just like any other migration:

```sql
-- goway:noTransaction

CREATE INDEX CONCURRENTLY idx_on_${flyway:defaultSchema}_users_email 
  ON users(email);
```

See [Placeholders](/goway/reference/placeholders/) for details.

## When to Use

- **PostgreSQL**: `CREATE INDEX CONCURRENTLY`, `REINDEX CONCURRENTLY`, `CLUSTER`, or any statement that explicitly forbids transactions
- **SQLite**: `VACUUM`, `PRAGMA optimize`, or other pragma statements that require auto-commit
- **Avoid**: Regular table creation, data migration, or DML statements—these should use transactional migrations for safety
