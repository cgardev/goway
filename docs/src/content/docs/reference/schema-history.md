---
title: Schema History
description: "The flyway_schema_history table: its columns, how rows are written, and how state is derived."
---

The schema history table tracks every migration applied to the database. goway creates and maintains this table automatically, storing all metadata needed to determine which migrations have run, in what order, and with what outcome.

## The Schema History Table

By default, goway writes to a table named `flyway_schema_history`. You can change this name with the [`Table()`](/goway/reference/configuration/) configuration method. The table always lives in the default schema.

### Columns

The schema history table has the following columns:

| Column | Type | Meaning |
|--------|------|---------|
| `installed_rank` | INT | The numeric rank in which the migration was applied, starting at 1. Primary key. |
| `version` | VARCHAR(50) | The migration version, or NULL for repeatable migrations and schema markers. |
| `description` | VARCHAR(200) | The human-readable description of the migration. |
| `type` | VARCHAR(20) | The migration type: `SQL`, `BASELINE`, `SCHEMA`, or `DELETED`. |
| `script` | VARCHAR(1000) | The script file name or synthetic marker. |
| `checksum` | INTEGER | The CRC32 checksum of the script content, or NULL for synthetic entries. |
| `installed_by` | VARCHAR(100) | The database user that applied the migration. |
| `installed_on` | TIMESTAMP | The timestamp when the migration was applied. Set by the database. |
| `execution_time` | INTEGER | The duration of the migration in milliseconds. |
| `success` | BOOLEAN | Whether the migration completed successfully. |

### Writing History

Each time a migration executes, a row is inserted into the history table **in the same transaction as the migration itself**. This ensures that if the migration succeeds, the history row is committed together; if the migration fails, both are rolled back.

The `installed_on` timestamp is set by the database server at insert time using the database's current time function:
- **PostgreSQL**: `now()`
- **SQLite**: `strftime('%Y-%m-%d %H:%M:%f', 'now')`

The `installed_by` column records the database user who ran the migration. This is determined by querying the database (PostgreSQL's `current_user`, or an empty string on SQLite, which has no concept of user). You can override this with the [`InstalledBy()`](/goway/reference/configuration/) configuration method.

:::note
Non-transactional migrations—those marked with `-- goway:noTransaction`—run outside a transaction and may fail without rolling back the history row. In this case, the row is marked with `success = false` and can be cleaned up with the [Repair](/goway/reference/commands/) command.
:::

## Migration States

The `MigrationState` describes the relationship between a resolved migration (found on disk) and a recorded history row. goway derives the state of each migration dynamically when you run [`Info`](/goway/reference/commands/), [`Validate`](/goway/reference/commands/), or [`Migrate`](/goway/reference/commands/).

### Applied Migrations

| State | Meaning |
|-------|---------|
| `Success` | A versioned or repeatable migration was applied successfully. |
| `Failed` | A migration was applied but failed and must be resolved. |
| `Outdated` | A repeatable migration was applied, but its script changed; it will be re-applied on the next migrate. |
| `Superseded` | An older run of a repeatable migration; a newer run of the same description is now current. |
| `Baseline` | The synthetic baseline entry created by the [`Baseline`](/goway/reference/commands/) command. |

### Unapplied Migrations

| State | Meaning |
|-------|---------|
| `Pending` | A resolved migration that has not been applied and will be applied on the next migrate. |
| `Above Target` | A pending migration with a version above the configured target version. |
| `Ignored` | A pending migration that will be skipped—for example, because it is below the baseline or (when out-of-order execution is disabled) because it was discovered after migrations of a higher version. |

### Missing or Future Migrations

| State | Meaning |
|-------|---------|
| `Missing` | A migration that was successfully applied in the past but is no longer resolved from the configured locations. |
| `Failed (Missing)` | A migration that failed in the past and is no longer resolved. |
| `Future` | An applied migration with a version higher than any currently resolved migration. |
| `Failed (Future)` | A failed applied migration with a version higher than any currently resolved migration. |
| `Out of Order` | A migration that was applied but whose version is earlier than other applied migrations (only shown when out-of-order execution is disabled). |

## Repair and Maintenance

The [`Repair`](/goway/reference/commands/) command helps recover from migration failures and script changes:

- **Removes failed rows**: Deletes all history rows where `success = false`. This allows you to retry the migration after fixing the problem.
- **Realigns checksums**: Updates the recorded `checksum` of any applied migration to match the current resolved script. Use this after intentionally modifying an already-applied script (typically a repeatable migration).

Repair does not modify successful migrations or change the order of migrations. Use the [`Clean`](/goway/reference/commands/) command if you need to remove all applied migrations entirely.

## Flyway Compatibility

The schema history table is designed for compatibility with [Flyway](https://flywaydb.org/). The table name, column names, type values (`SQL`, `BASELINE`, `SCHEMA`, `DELETED`), and placeholder names (`${flyway:user}`, `${flyway:table}`, etc.) follow Flyway conventions. This allows goway to read and write a history table that Flyway has already created, or for Flyway to continue using a history table that goway created.
