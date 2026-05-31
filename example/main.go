// Command example demonstrates running migrations against an in-process SQLite
// database using migrations embedded into the binary with go:embed.
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

// migrations holds the SQL scripts that are compiled into the binary. The
// migrator reads them through the fs.FS based source.
//
//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	if err := demonstrate(); err != nil {
		log.Fatal(err)
	}
}

func demonstrate() error {
	directory, err := os.MkdirTemp("", "goway-example")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)

	databasePath := filepath.Join(directory, "example.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return err
	}
	defer database.Close()

	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		FS(migrations, "migrations").
		Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, err := migrator.Migrate(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %d migration(s); schema is now at version %s.\n\n",
		result.MigrationsExecuted, result.TargetSchemaVersion)

	information, err := migrator.Info(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Migration state:")
	for _, migration := range information.Migrations {
		version := migration.Version
		if version == "" {
			version = "(repeatable)"
		}
		fmt.Printf("  %-12s %-24s %s\n", version, migration.Description, migration.State)
	}
	return nil
}
