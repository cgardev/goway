---
title: Commands
description: Reference for migrate, info, validate, baseline, repair, and clean, with their result types.
---

All commands are methods on `*Migrator`, which is obtained by calling `Load()` on a configured `*Configuration`. Each command accepts a `context.Context` and returns a result struct and an error.

## Migrate

```go
func (f *Migrator) Migrate(ctx context.Context) (*MigrateResult, error)
```

Applies every pending migration in order, bringing the database up to date. Missing schemas and the schema history table are created when needed. Each migration runs inside its own transaction, so a failed migration leaves no partial state and can be retried after the script is corrected.

Before applying migrations, `Migrate` will:
- Create any configured schemas that do not yet exist (unless schema creation is disabled).
- Create the schema history table if it does not exist.
- Optionally insert a baseline entry if configured with `BaselineOnMigrate(true)` and the history table is newly created.
- Run `beforeMigrate` callbacks.

If `ValidateOnMigrate(true)` is set (the default), validation is performed before any migrations are applied. Validation failures cause `Migrate` to return without making changes.

Each migration runs within a transaction on PostgreSQL, which is released after the migration completes. SQLite is single-writer by default. PostgreSQL enforces a session advisory lock to prevent concurrent migrators.

The command fires `beforeEachMigrate` and `afterEachMigrate` callbacks around each migration, then runs `afterMigrate` callbacks once all migrations complete.

### Result

```go
type MigrateResult struct {
  InitialSchemaVersion string        // The schema version before migration
  TargetSchemaVersion  string        // The schema version after migration
  MigrationsExecuted   int           // Number of migrations applied
  Migrations           []MigrationInfo // Details of each applied migration
}
```

### Errors

- `errors.Is(err, ErrValidationFailed)`: validation found a problem before applying any migrations.
- `errors.Is(err, ErrFailedMigration)`: a previously failed migration is present; resolve it with `Repair` before proceeding.
- Context cancellation or database errors.
- A migration may partially fail, in which case the result is returned with the target version as of the failure point. The failed row is recorded in the history table and can be repaired.

## Info

```go
func (f *Migrator) Info(ctx context.Context) (*InfoResult, error)
```

Computes and returns the state of every migration—both resolved and applied—without making any changes to the database. This is useful for auditing, logging, and integration with deployment tools.

### Result

```go
type InfoResult struct {
  Migrations []MigrationInfo // Every migration, resolved and applied
  Current    string          // The current schema version
}
```

## Validate

```go
func (f *Migrator) Validate(ctx context.Context) (*ValidateResult, error)
```

Inspects the applied migrations against the resolved scripts and reports any discrepancies. Validation detects checksum mismatches, locally missing migrations, failed migrations, and unapplied migrations.

If validation passes, the error is `nil` and `Valid` is `true`. If validation fails, an error wrapping `ErrValidationFailed` is returned, but the result still carries the full list of problems.

### Result

```go
type ValidateResult struct {
  Valid           bool              // True if no problems were found
  Errors          []ValidationError // Details of each problem
  ValidationCount int               // Number of migrations inspected
}
```

### Errors

- `errors.Is(err, ErrValidationFailed)`: one or more validation problems were found.

## Baseline

```go
func (f *Migrator) Baseline(ctx context.Context) (*BaselineResult, error)
```

Records a baseline entry in the schema history, allowing existing databases to be brought under migration management without re-running the migrations that produced their current state. Migrations at or below the baseline version are subsequently ignored.

Baseline can only be called on an empty schema history (containing no applied migrations except synthetic entries like schema creation or a prior baseline). If the history already contains applied migrations, an error is returned.

### Result

```go
type BaselineResult struct {
  BaselineVersion     string // The version recorded as the baseline
  BaselineDescription string // The description recorded as the baseline
  Created             bool   // True if a new baseline entry was inserted; false if one already existed
}
```

## Repair

```go
func (f *Migrator) Repair(ctx context.Context) (*RepairResult, error)
```

Removes failed entries from the schema history and realigns recorded checksums with resolved scripts. This is the recommended remedy after a failed migration on a database without transactional DDL, or after deliberately editing an already-applied migration script.

Repair does not re-apply migrations; it only updates the history table. Use `Migrate` to apply pending or outdated migrations after repair.

### Result

```go
type RepairResult struct {
  RemovedFailed    int // Number of failed rows deleted from the history
  AlignedChecksums int // Number of rows whose checksum was updated
}
```

## Clean

```go
func (f *Migrator) Clean(ctx context.Context) (*CleanResult, error)
```

Drops every object in the managed schemas, returning the database to an empty state. This is a destructive and irreversible operation. Clean is disabled by default and must be explicitly enabled by calling `CleanDisabled(false)` on the configuration.

### Result

```go
type CleanResult struct {
  SchemasCleaned []string // The schemas that were cleaned
}
```

### Errors

- `errors.Is(err, ErrCleanDisabled)`: clean is disabled (the default).

## Dialect

```go
func (f *Migrator) Dialect() Dialect
```

Returns the database dialect that was configured or detected (e.g., PostgreSQL or SQLite). Useful for logging or adapting behavior based on dialect-specific capabilities.

## Result Types

### MigrationInfo

```go
type MigrationInfo struct {
  Version       string         // The version number (empty for repeatable migrations)
  Description   string         // The migration description
  Type          string         // The migration type: "SQL", "BASELINE", "SCHEMA", or "DELETED"
  Script        string         // The script file name or synthetic marker
  Checksum      *int32         // CRC32 checksum of the resolved script; nil if not yet applied
  State         MigrationState // Current state of the migration
  InstalledRank int            // Database-assigned order; zero if not installed
  InstalledOn   *time.Time     // Timestamp the migration was applied (database clock); nil if not installed
  InstalledBy   string         // User or system that applied the migration; empty if not installed
  ExecutionTime int            // Duration in milliseconds; zero if not installed
}
```

### MigrationState

A migration state is one of:

- `"Pending"`: resolved but not yet applied.
- `"Success"`: applied successfully.
- `"Failed"`: previously applied but reported an error; must be repaired before migrating further.
- `"Out of Order"`: applied successfully but with a version lower than an already-applied migration (requires `OutOfOrder(true)` to apply).
- `"Outdated"`: a repeatable migration that has been re-applied with a different checksum.
- `"Superseded"`: a repeatable migration that has been replaced by a newer run.
- `"Missing"`: applied but no longer resolved locally.
- `"Failed (Missing)"`: applied but marked failed and no longer resolved locally.
- `"Future"`: applied with a version higher than any currently resolved migration.
- `"Failed (Future)"`: applied and failed, with a version higher than any currently resolved migration.
- `"Above Target"`: resolved but its version exceeds the configured target.
- `"Ignored"`: not applied; either at or below a baseline version or an out-of-order migration with `OutOfOrder(false)`.
- `"Baseline"`: a baseline entry in the history.

### ValidationError

```go
type ValidationError struct {
  Version     string // The version of the migration, empty for repeatable migrations
  Description string // The migration description
  Script      string // The script file name
  Message     string // Explanation of the problem
}
```

Validation errors include checksum mismatches, missing applied migrations, failed migrations, and unapplied migrations (when called outside of `Migrate`).
