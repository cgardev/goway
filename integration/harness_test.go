package integration

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// migrationsFS holds the test migration scripts embedded into the test binary.
//
//go:embed testdata/shared/*.sql testdata/pg/*.sql testdata/sqlite/*.sql testdata/placeholder/*.sql
var migrationsFS embed.FS

const defaultPostgresImage = "postgres:18-alpine"

// postgresDSN is the connection string for the shared PostgreSQL container, or
// an empty string when no container could be started.
var postgresDSN string

func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

func runTestMain(m *testing.M) int {
	ctx := context.Background()

	image := os.Getenv("GOWAY_PG_IMAGE")
	if image == "" {
		image = defaultPostgresImage
	}

	container, err := postgres.Run(ctx, image,
		postgres.WithDatabase("goway_test"),
		postgres.WithUsername("goway"),
		postgres.WithPassword("goway_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		// PostgreSQL tests skip themselves when no container is available; the
		// SQLite tests still run.
		fmt.Fprintf(os.Stderr, "PostgreSQL container unavailable, skipping its tests: %v\n", err)
		return m.Run()
	}
	defer func() {
		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate postgres container: %v\n", terminateErr)
		}
	}()

	if dsn, dsnErr := container.ConnectionString(ctx, "sslmode=disable"); dsnErr == nil {
		postgresDSN = dsn
	}

	return m.Run()
}

// openSQLite opens a fresh SQLite database in a temporary file with foreign key
// enforcement enabled.
func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "goway_test.db")
	database, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// openPostgres connects to the shared PostgreSQL container, skipping the test
// when none is available.
func openPostgres(t *testing.T) *sql.DB {
	t.Helper()
	if postgresDSN == "" {
		t.Skip("PostgreSQL container is not available (Docker required)")
	}
	database, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
