---
title: Migration Naming
description: The exact rules for migration file names, versions, and ordering.
---

Goway migrations must follow a strict naming convention. File names encode the migration type (versioned or repeatable), version, and description. Understanding this system is essential for organizing your migrations and ensuring reliable execution.

## File Name Components

Every migration file name consists of these parts:

1. **Prefix** — Identifies the migration type:
   - `V` for versioned migrations (default)
   - `R` for repeatable migrations
2. **Version** — For versioned migrations only; absent for repeatables
3. **Separator** — Divides version from description (default `__`)
4. **Description** — Human-readable text; underscores become spaces
5. **Suffix** — File extension; `.sql` by default

### Example File Names

- `V1__init.sql` — versioned migration 1, description "init"
- `V2.1__add_index.sql` — versioned migration 2.1, description "add index"
- `R__populate_constants.sql` — repeatable migration, description "populate constants"
- `V1_0__baseline.sql` — versioned migration 1.0 (underscores normalized to dots)

## Configuring the Naming Convention

Use fluent setters on the configuration object to adjust the naming rules:

```go
config := goway.Configure().
    DataSource(db).
    SQLMigrationPrefix("M").           // change versioned prefix
    RepeatableSQLMigrationPrefix("RM"). // change repeatable prefix
    SQLMigrationSeparator("--").        // change separator
    SQLMigrationSuffixes(".sql", ".ddl") // add or replace suffixes
```

The defaults match Flyway:

| Component | Default Value |
|-----------|---------------|
| Versioned prefix | `V` |
| Repeatable prefix | `R` |
| Separator | `__` |
| Suffixes | `.sql` |

## Suffix Matching

The suffix is matched **case-insensitively** against the file name, and the **first configured suffix wins**. This allows flexibility with file systems and editors:

- `V1__init.SQL` matches `.sql` (case-insensitive)
- `V1__init.sql` and `V1__init.ddl` both match if suffixes are `[".sql", ".ddl"]`
- If suffixes are `[".ddl", ".sql"]`, a file named `V1__init.sql.ddl` matches `.ddl` first

## Versioned Migrations

A versioned migration has the form:

```
V<version>__<description>.<suffix>
```

The version must be present and non-empty. Versions consist of one or more dot-separated non-negative integers. Underscores in the raw file name are normalized to dots during parsing.

Valid examples:
- `V1__init.sql`
- `V1.0.1__schema_changes.sql`
- `V20260531__bootstrap.sql` (date-based version)
- `V1_0__init.sql` (underscores become dots: 1.0)

Invalid examples:
- `V__init.sql` (missing version)
- `V1.2.X__changes.sql` (non-numeric component)
- `V1__init` (missing suffix)

## Repeatable Migrations

A repeatable migration has the form:

```
R__<description>.<suffix>
```

The version part must be absent. Repeatable migrations run after all versioned migrations and are re-applied whenever their content (and thus checksum) changes.

Valid examples:
- `R__populate_constants.sql`
- `R__refresh_materialized_views.sql`

Invalid examples:
- `R1__description.sql` (repeatable must not declare a version)
- `R__some_script` (missing suffix)

## Version Parsing and Comparison

Versions are parsed and compared numerically, enabling arbitrary-precision version numbers and intuitive ordering.

### Parsing Rules

1. Underscores in the raw file name are normalized to dots: `1_2_3` becomes `1.2.3`
2. The normalized string is split on dots to extract integer parts
3. Each part is parsed as a non-negative integer of arbitrary precision
4. Trailing zeros are dropped so that `1`, `1.0`, and `1.0.0` are equivalent

### Comparison Rules

Versions are compared element by element from left to right. Components beyond the length of either version are treated as zero, so `1` equals `1.0`.

#### Comparison Examples

| Version A | Version B | Comparison | Notes |
|-----------|-----------|------------|-------|
| 1 | 2 | A < B | Single-part versions |
| 1.0 | 1 | A = B | Trailing zeros dropped |
| 1.1 | 1.2 | A < B | Second component compared |
| 2.0.1 | 2.1 | A < B | 2.0.1 vs 2.1.0 (zero padding) |
| 10 | 9 | A > B | Numeric, not lexicographic |
| 20260531 | 20260601 | A < B | Date-based versions work |

## Migration Ordering

Goway resolves and applies pending migrations in this order:

1. **Versioned migrations** — sorted by version in ascending order
2. **Repeatable migrations** — sorted by description in ascending (lexicographic) order

For example, given these files:

```
V1__init.sql
V2.1__add_index.sql
R__populate_constants.sql
V2__baseline.sql
R__refresh_views.sql
```

The execution order would be:

1. `V1__init.sql` (version 1)
2. `V2__baseline.sql` (version 2)
3. `V2.1__add_index.sql` (version 2.1)
4. `R__populate_constants.sql` (repeatable, description "populate constants")
5. `R__refresh_views.sql` (repeatable, description "refresh views")

## Description Handling

The description is everything between the separator and the suffix. All underscores in the description are converted to spaces before being stored in the schema history table.

- File name: `V1__create_user_table.sql`
- Stored description: `create user table`

Descriptions are case-sensitive for ordering repeatables (via lexicographic comparison) but are purely informational; they do not affect execution logic.

## Duplicate Detection

Goway rejects migrations with duplicate versions or duplicate repeatable descriptions, returning an error at load time.

- **Duplicate versioned migration**: Two files targeting the same version (e.g., both `V1__init.sql` and `V1__baseline.sql`) return `ErrDuplicateVersion`.
- **Duplicate repeatable migration**: Two files with the same repeatable description (e.g., both `R__init_constants.sql` and `R__init_constants.sql`) return `ErrDuplicateRepeatable`.

Version comparison ignores trailing zeros, so `V1.sql` and `V1.0.sql` are detected as duplicates.

## Migration Checksums

When a migration file is loaded, its content is checksummed with CRC32 (IEEE algorithm) for verification and change detection. The checksum:

- Is computed **line by line**, excluding line terminators
- **Skips the BOM** on the first line if present
- Is **not affected** by placeholder substitution (computed on raw content)
- Is compared against the stored checksum in the schema history on subsequent runs
- Triggers a "state changed" error if it does not match for an already-applied migration

This ensures Goway can reliably detect whether a migration has been modified after execution.
