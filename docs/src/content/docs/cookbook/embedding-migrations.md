---
title: Embedding Migrations
description: Compile your migration scripts into your binary with go:embed.
---

When you compile your application, you can embed your migration scripts directly into the binary using Go's `go:embed` directive. This produces a single, self-contained executable that carries everything needed to initialize and migrate the database—no separate migration files to ship, no file paths to configure on disk.

## Why Embed Migrations?

Embedding migrations is particularly useful when deploying applications as a single binary:

- **Single artifact**: Deploy one executable; no need to manage migration files in version control or copy them to production.
- **Atomicity**: The migrations and application code always stay in sync.
- **Portability**: The binary works the same way on any platform or container, without depending on filesystem paths at runtime.
- **Easier testing**: Use the embedded migrations in unit or integration tests without mocking the file system.

## How It Works

Use Go's [`go:embed` directive](https://pkg.go.dev/embed) to embed a directory of SQL scripts into an `embed.FS`, then pass that filesystem to the migrator via the `.FS()` method:

```go
import (
    "embed"
    "github.com/cgardev/goway"
)

//go:embed migrations/*.sql
var migrations embed.FS

migrator, err := goway.Configure().
    DataSource(db).
    FS(migrations, "migrations").
    Load()
```

The `.FS()` method accepts an `embed.FS` and one or more directory paths within it. If no paths are specified, it defaults to `"."` (the root of the filesystem).

You can call `.FS()` multiple times to register migrations from different embedded directories, just as you can chain multiple `.Locations()` calls for filesystem paths. Migrations are discovered and sorted across all sources.

## Example: SQLite with Embedded Migrations

Here is a complete, runnable example that embeds SQLite migrations and applies them in-process:

### Directory Layout

```
myapp/
├── main.go
├── go.mod
└── migrations/
    ├── V1__init.sql
    ├── V2__add_users_table.sql
    └── V3__add_index.sql
```

### Code

```go
package main

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/cgardev/goway"

    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    // Create or open a database (for this example, a temporary file).
    dbPath := filepath.Join(os.TempDir(), "myapp.db")
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return err
    }
    defer db.Close()

    // Configure the migrator with embedded migrations.
    migrator, err := goway.Configure().
        DataSource(db).
        Dialect(goway.SQLite()).
        FS(migrations, "migrations").
        Load()
    if err != nil {
        return err
    }

    // Run migrations.
    ctx := context.Background()
    result, err := migrator.Migrate(ctx)
    if err != nil {
        return err
    }

    fmt.Printf("Applied %d migration(s); schema is now at version %s.\n",
        result.MigrationsExecuted, result.TargetSchemaVersion)

    return nil
}
```

### Example Migrations

**migrations/V1__init.sql:**
```sql
CREATE TABLE products (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);
```

**migrations/V2__add_users_table.sql:**
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT UNIQUE NOT NULL
);
```

**migrations/V3__add_index.sql:**
```sql
CREATE INDEX idx_users_email ON users(email);
```

## Combining Embedded and Filesystem Locations

You can mix embedded and filesystem sources:

```go
migrator, err := goway.Configure().
    DataSource(db).
    Locations("filesystem:db/migrations").  // scan the filesystem
    FS(embedded, "migrations").             // also scan the embedded FS
    Load()
```

Migrations are resolved from both locations and sorted together, so you can keep some migrations on disk while others are embedded.

:::note
The example module in the repository (`example/main.go`) demonstrates this exact pattern with SQLite. Run `go run ./example` to see it in action.
:::

## See Also

- [Commands](/goway/reference/commands/) — `migrate`, `info`, `validate`, and other operations
- [Configuration Reference](/goway/reference/configuration/) — all available configuration options
- [Writing Migrations](/goway/guides/writing-migrations/) — naming conventions and script syntax
