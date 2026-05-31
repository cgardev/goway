---
title: Placeholders
description: Placeholder substitution in migration scripts, delimiters, defaults, and built-ins.
---

Placeholders allow you to inject dynamic values into your migration scripts at execution time. goway supports the same placeholder syntax as Flyway, making it easy to maintain compatibility or migrate existing Flyway projects to Go.

## Default delimiters

Placeholders are delimited by `${` and `}` by default. Any placeholder found in a migration script or callback that does not have a configured value produces an error.

```sql
-- Example migration using a placeholder
CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

GRANT SELECT ON users TO ${APP_DB_USER};
```

## Configuring placeholder values

Pass a map of key-value pairs to the `Placeholders` method during configuration:

```go
migrator, err := goway.Configure().
  DataSource(db).
  Placeholders(map[string]string{
    "APP_DB_USER": "app_reader",
    "SCHEMA_NAME": "public",
  }).
  Load()
if err != nil {
  log.Fatal(err)
}
```

Placeholder keys are matched case-insensitively. The configuration automatically stores keys in lowercase, so `${APP_DB_USER}` and `${app_db_user}` in your scripts will both match the configured key.

## Custom delimiters

You can change the opening and closing delimiters with `PlaceholderPrefix` and `PlaceholderSuffix`:

```go
migrator, err := goway.Configure().
  DataSource(db).
  PlaceholderPrefix("@").
  PlaceholderSuffix("@").
  Placeholders(map[string]string{
    "APP_USER": "app_service",
  }).
  Load()
if err != nil {
  log.Fatal(err)
}
```

Migration scripts would then use `@APP_USER@` instead of `${APP_USER}`.

## Built-in placeholders

goway provides four built-in placeholders that are always available, regardless of what you configure:

- `${flyway:defaultSchema}` — The default schema (the schema containing the schema history table)
- `${flyway:user}` — The name of the user that applied the migration (set by `InstalledBy`, defaults to the database connection user)
- `${flyway:table}` — The name of the schema history table
- `${flyway:filename}` — The name of the current migration script file

:::tip
The `flyway:` prefix is kept for drop-in compatibility with Flyway. If you are migrating from Flyway, your existing placeholder scripts will work without modification.
:::

Example using built-ins:

```sql
-- V2__add_audit_log.sql
CREATE TABLE audit_log (
  event_id BIGSERIAL PRIMARY KEY,
  event_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  applied_by TEXT,
  migration_script TEXT
);

INSERT INTO ${flyway:defaultSchema}.audit_log (applied_by, migration_script)
VALUES ('${flyway:user}', '${flyway:filename}');
```

## Timing and checksums

Placeholder substitution happens at execution time, after migrations are loaded and resolved but before SQL statements are executed. Crucially, **placeholder substitution does not affect the migration checksum**. The checksum is computed on the raw file content before any placeholders are replaced.

This means:
- You can change placeholder values between runs without invalidating the migration history
- A migration with the same script content will have the same checksum regardless of placeholder substitution
- The schema history table always records the filename, not the substituted result

## Error handling

An unresolved placeholder—one that appears in a script but has no value configured—produces an error before any SQL is executed:

```
goway: no value provided for placeholder ${UNDEFINED_VALUE}
```

Check that all placeholders in your scripts are listed in the configuration map, and that the keys match (case does not matter).

## Placeholders in callbacks

SQL callback scripts (found in your migration locations and named like `beforeMigrate.sql` or `afterEachMigrate__description.sql`) also support placeholder substitution. Built-in placeholders are available in callbacks, and any custom placeholders you configure work as well.

:::note
Callback scripts use the same placeholder delimiters as regular migrations. If you customize `PlaceholderPrefix` or `PlaceholderSuffix`, both migrations and callbacks will use the new delimiters.
:::

## Comparison to Flyway

goway uses the same placeholder semantics as Flyway:

- Default delimiters: `${` and `}`
- Built-in namespace: `flyway:` (e.g., `${flyway:defaultSchema}`)
- Case-insensitive key matching
- Substitution at execution time, not affecting checksums
- Unresolved placeholders produce errors
- Callbacks support placeholders

If you are porting Flyway migrations to goway, you can reuse your placeholder configuration and scripts with no changes.
