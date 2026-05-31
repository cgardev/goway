---
title: Configuration
description: Every setting on the fluent configuration builder and its default value.
---

Every aspect of goway's behavior is controlled through the fluent `Configuration` builder. You create one with `Configure()`, chain setter calls to customize it, then call `.Load()` (or `.LoadContext(ctx)`) to produce a ready-to-use `Migrator`.

## Data Source and Dialect

The database connection and its type are the foundation of every migration run.

### DataSource

```go
config := goway.Configure().
    DataSource(db).
    Load()
```

Required. Pass your `*sql.DB` connection pool. There is no default.

### Dialect

```go
config := goway.Configure().
    DataSource(db).
    Dialect(goway.Postgres()).
    Load()
```

Optional. goway auto-detects the dialect from the connection if you do not set it. You can set it explicitly to bypass detection or to choose a specific dialect:

- `goway.Postgres()` — PostgreSQL 15 and later
- `goway.SQLite()` — SQLite 3.37 and later

If you do not set a dialect, `.Load()` queries the database to determine it.

## Locations and File Systems

Tell goway where to find migration scripts.

### Locations

```go
config := goway.Configure().
    DataSource(db).
    Locations("db/migration", "db/util").
    Load()
```

Default: `["db/migration"]`

Replaces the list of file system directories to scan. Locations are relative to the current working directory (when running a CLI) or the program's entry point (when embedded). You may include `filesystem:` or `classpath:` prefixes; they are optional.

### FS (Embedded Migrations)

```go
//go:embed db/migration/*.sql
var migrations embed.FS

config := goway.Configure().
    DataSource(db).
    FS(migrations, "db/migration").
    Load()
```

Optional. Register an `fs.FS` (such as one created with `go:embed`) alongside the directory paths inside it that contain migrations. You can call `.FS()` multiple times to add more file systems. If you pass an empty path list, it defaults to `"."`.

:::tip
Embedding migrations into your binary guarantees they are always available, even if the filesystem changes. This is recommended for production deployments.
:::

## Schemas and History Table

Manage which schemas goway touches and where it records its history.

### Schemas

```go
config := goway.Configure().
    DataSource(db).
    Schemas("public", "audit", "internal").
    Load()
```

Optional. List the schemas managed by the migrator. The first schema also becomes the default schema. On PostgreSQL, the first schema is passed to `search_path` during migration execution. On SQLite, this setting has no effect (SQLite has no schemas).

### DefaultSchema

```go
config := goway.Configure().
    DataSource(db).
    Schemas("public", "audit").
    DefaultSchema("audit").
    Load()
```

Optional. Override which schema holds the schema history table. By default, the first schema from `.Schemas()` is used. The history table is always created in this schema.

### CreateSchemas

```go
config := goway.Configure().
    DataSource(db).
    CreateSchemas(false).
    Load()
```

Default: `true`

Controls whether goway automatically creates missing schemas during migration. Set to `false` if you want to manage schema creation yourself.

### Table

```go
config := goway.Configure().
    DataSource(db).
    Table("migration_history").
    Load()
```

Default: `flyway_schema_history`

The name of the schema history table. The table is created automatically in the default schema if it does not exist. The table structure matches Flyway's for drop-in compatibility.

## Baseline

Configure baseline behavior for existing databases.

### BaselineVersion

```go
config := goway.Configure().
    DataSource(db).
    BaselineVersion("2024.1").
    Load()
```

Default: `"1"`

The version recorded when you run the `baseline` command, and the lowest version below which migrations are ignored. Versions use dot or underscore separators (e.g., `"2024.1"`, `"1.0.0"`, `"3"`).

### BaselineDescription

```go
config := goway.Configure().
    DataSource(db).
    BaselineDescription("Initial production schema").
    Load()
```

Default: `"<< Flyway Baseline >>"`

The description recorded by the baseline command.

### BaselineOnMigrate

```go
config := goway.Configure().
    DataSource(db).
    BaselineOnMigrate(true).
    Load()
```

Default: `false`

When enabled, if the database schema exists but the history table does not, goway automatically baseline the schema on the first migrate. This is useful for bringing existing databases under migration control without a separate baseline step.

## Placeholders

Substitute values into your migration scripts at runtime.

### Placeholders

```go
config := goway.Configure().
    DataSource(db).
    Placeholders(map[string]string{
        "owner": "app_user",
        "env":   "production",
    }).
    Load()
```

Default: empty map

Pass a map of key-value pairs to be substituted into scripts. Keys are matched case-insensitively. Placeholders do not affect checksums—only the raw file content is used for checksums, so changing a placeholder value and re-running a migration will not re-execute it.

The built-in placeholders are always available:

- `${flyway:defaultSchema}` — the default schema name
- `${flyway:user}` — the database user
- `${flyway:table}` — the history table name
- `${flyway:filename}` — the current migration script filename

### PlaceholderPrefix and PlaceholderSuffix

```go
config := goway.Configure().
    DataSource(db).
    PlaceholderPrefix("[[").
    PlaceholderSuffix("]]").
    Placeholders(map[string]string{
        "owner": "app_user",
    }).
    Load()
```

Defaults: `"${"` and `"}"`

Customize the delimiters used to mark placeholders in scripts. Use this if your SQL already uses the default `${}` syntax for PostgreSQL dollar quoting or other constructs.

## Migration Naming

Override the prefixes, separator, and file suffixes that goway uses to identify and parse migration scripts.

### SQLMigrationPrefix and RepeatableSQLMigrationPrefix

```go
config := goway.Configure().
    DataSource(db).
    SQLMigrationPrefix("M").
    RepeatableSQLMigrationPrefix("X").
    Load()
```

Defaults: `"V"` and `"R"`

The prefixes that mark versioned and repeatable migrations respectively. By default, versioned migrations start with `V` (e.g., `V1__init.sql`, `V2__add_index.sql`) and repeatable ones start with `R` (e.g., `R__create_views.sql`).

### SQLMigrationSeparator

```go
config := goway.Configure().
    DataSource(db).
    SQLMigrationSeparator("--").
    Load()
```

Default: `"__"`

The token separating the version from the description in a versioned migration filename. Changing this to `--` allows you to use names like `V1--init.sql`.

### SQLMigrationSuffixes

```go
config := goway.Configure().
    DataSource(db).
    SQLMigrationSuffixes(".sql", ".ddl").
    Load()
```

Default: `[".sql"]`

The file suffixes recognized as migration scripts. By default, only `.sql` files are scanned. You can add others like `.ddl`, `.plsql`, etc., if your scripts use different extensions.

## Safety Controls

These settings protect you from unintended consequences.

### ValidateOnMigrate

```go
config := goway.Configure().
    DataSource(db).
    ValidateOnMigrate(false).
    Load()
```

Default: `true`

When enabled, `migrate` runs validation before applying any migrations. Validation checks for missing, duplicate, and out-of-order migrations. If validation fails, no migrations are applied. Disable only if you have a specific reason to skip this check.

### CleanDisabled

```go
config := goway.Configure().
    DataSource(db).
    CleanDisabled(false).
    Load()
```

Default: `true`

When enabled (the default), the `clean` command is disabled to prevent accidental deletion of all objects in your schema. Set to `false` to allow clean operations. This is recommended only in development.

:::caution
The `clean` command drops all objects in every managed schema. Use it only when you fully understand the consequences.
:::

### OutOfOrder

```go
config := goway.Configure().
    DataSource(db).
    OutOfOrder(true).
    Load()
```

Default: `false`

When enabled, allows migrations with a version lower than the current schema version to be applied. By default, this is not permitted to help catch migration ordering mistakes. Enable only when you deliberately want to insert migrations into the past.

### Target

```go
config := goway.Configure().
    DataSource(db).
    Target("2024.1").
    Load()
```

Optional. Limit the highest version that `migrate` will apply. Useful for testing migrations without applying the latest versions, or for gradual deployments.

## Other Settings

### InstalledBy

```go
config := goway.Configure().
    DataSource(db).
    InstalledBy("deployment-bot").
    Load()
```

Optional. Override the user name recorded in the history table for applied migrations. By default, the database user is used.

### Callbacks

```go
config := goway.Configure().
    DataSource(db).
    Callbacks(myCallback).
    Load()
```

Optional. Register programmatic callbacks invoked during migration runs. In addition, goway discovers and executes SQL callback scripts from your locations: `beforeMigrate.sql`, `afterMigrate.sql`, `beforeEachMigrate.sql`, and `afterEachMigrate__<description>.sql`.

## Loading the Configuration

Once you have configured all settings, call `.Load()` or `.LoadContext(ctx)` to finalize and validate the configuration:

```go
db, _ := sql.Open("pgx", "postgresql://...")

migrator, err := goway.Configure().
    DataSource(db).
    Locations("db/migration").
    Load()
if err != nil {
    log.Fatal(err)
}

result, err := migrator.Migrate(context.Background())
```

`.Load()` returns a `*Migrator` ready for use, or an error if the configuration is invalid (e.g., missing data source, invalid version format, unreachable migration locations).

For more detail on every configuration option, see the [full API reference](/goway/reference/configuration/).
