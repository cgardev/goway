package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cgardev/goway"
)

// postgresMigrator builds a migrator that manages the given schema and creates
// it if missing. Each test uses a distinct schema so they remain isolated while
// sharing a single container.
func postgresMigrator(t *testing.T, database *sql.DB, schema string, paths ...string) *goway.Migrator {
	t.Helper()
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.Postgres()).
		Locations().
		FS(migrationsFS, paths...).
		Schemas(schema).
		CreateSchemas(true).
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}
	return migrator
}

func TestPostgresMigrateLifecycle(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	migrator := postgresMigrator(t, database, "test_lifecycle", "testdata/shared")

	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.MigrationsExecuted != 3 {
		t.Fatalf("executed %d migrations, want 3", result.MigrationsExecuted)
	}
	if result.TargetSchemaVersion != "2" {
		t.Fatalf("target version = %q, want 2", result.TargetSchemaVersion)
	}

	if _, err := database.Exec(`INSERT INTO test_lifecycle.widgets (id, name, price) VALUES (1, 'gear', 5)`); err != nil {
		t.Fatalf("insert into migrated table: %v", err)
	}

	second, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.MigrationsExecuted != 0 {
		t.Fatalf("second migrate executed %d migrations, want 0", second.MigrationsExecuted)
	}
}

func TestPostgresCreatesAndUsesSchema(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	const schema = "test_schema_iso"
	migrator := postgresMigrator(t, database, schema, "testdata/shared")

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The migrated table and the schema history table must both live in the
	// configured schema, not in public.
	if got := tableCount(t, database, schema, "widgets"); got != 1 {
		t.Errorf("widgets table in schema %s = %d, want 1", schema, got)
	}
	if got := tableCount(t, database, schema, "flyway_schema_history"); got != 1 {
		t.Errorf("history table in schema %s = %d, want 1", schema, got)
	}
	if got := tableCount(t, database, "public", "widgets"); got != 0 {
		t.Errorf("widgets leaked into public schema (count %d)", got)
	}
}

func TestPostgresDollarQuotedFunction(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	const schema = "test_pg_func"
	migrator := postgresMigrator(t, database, schema, "testdata/pg")

	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("migrate with dollar quoted function: %v", err)
	}
	if result.MigrationsExecuted != 2 {
		t.Fatalf("executed %d migrations, want 2", result.MigrationsExecuted)
	}

	var functionCount int
	err = database.QueryRow(`SELECT COUNT(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace `+
		`WHERE n.nspname = $1 AND p.proname = 'event_count'`, schema).Scan(&functionCount)
	if err != nil {
		t.Fatalf("query function: %v", err)
	}
	if functionCount != 1 {
		t.Fatalf("event_count function count = %d, want 1", functionCount)
	}

	var eventRows int
	if err := database.QueryRow(`SELECT COUNT(*) FROM test_pg_func.events`).Scan(&eventRows); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventRows != 1 {
		t.Fatalf("events row count = %d, want 1", eventRows)
	}
}

func TestPostgresRepairRealignsChecksum(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	const schema = "test_repair"
	migrator := postgresMigrator(t, database, schema, "testdata/shared")

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := database.Exec(`UPDATE test_repair.flyway_schema_history SET checksum = 42 WHERE version = '1'`); err != nil {
		t.Fatalf("tamper checksum: %v", err)
	}
	if _, err := migrator.Validate(ctx); err == nil {
		t.Fatal("expected validation to fail after tampering")
	}

	repair, err := migrator.Repair(ctx)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if repair.AlignedChecksums < 1 {
		t.Fatalf("repair realigned %d checksums, want at least 1", repair.AlignedChecksums)
	}

	result, err := migrator.Validate(ctx)
	if err != nil || !result.Valid {
		t.Fatalf("validate after repair: valid=%v err=%v", result.Valid, err)
	}
}

// tableCount returns the number of tables with the given name in the given
// schema.
func tableCount(t *testing.T, database *sql.DB, schema, table string) int {
	t.Helper()
	var count int
	err := database.QueryRow(
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2`,
		schema, table).Scan(&count)
	if err != nil {
		t.Fatalf("count tables: %v", err)
	}
	return count
}
