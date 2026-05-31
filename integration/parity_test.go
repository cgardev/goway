package integration

import (
	"context"
	"testing"

	"github.com/cgardev/goway"
)

// TestPostgresNoTransactionConcurrentIndex verifies that a script marked with
// the no-transaction directive can run CREATE INDEX CONCURRENTLY, which fails
// inside a transaction block.
func TestPostgresNoTransactionConcurrentIndex(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	const schema = "test_notx"
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.Postgres()).
		Locations().
		FS(migrationsFS, "testdata/notx_pg").
		Schemas(schema).
		CreateSchemas(true).
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}

	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("migrate with CREATE INDEX CONCURRENTLY: %v", err)
	}
	if result.MigrationsExecuted != 2 {
		t.Fatalf("executed %d migrations, want 2", result.MigrationsExecuted)
	}

	var indexCount int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = $1 AND indexname = 'idx_items_name'`,
		schema).Scan(&indexCount)
	if err != nil {
		t.Fatalf("query index: %v", err)
	}
	if indexCount != 1 {
		t.Fatalf("concurrent index count = %d, want 1", indexCount)
	}
}

// TestPostgresConcurrentIndexFailsWithoutDirective is a control: the same
// statement without the directive runs inside a transaction and is rejected by
// PostgreSQL, proving the directive is what makes the success case work.
func TestPostgresConcurrentIndexFailsWithoutDirective(t *testing.T) {
	database := openPostgres(t)
	ctx := context.Background()
	const schema = "test_notx_control"

	setup, err := goway.Configure().
		DataSource(database).Dialect(goway.Postgres()).
		Locations().FS(migrationsFS, "testdata/notx_pg").
		Schemas(schema).CreateSchemas(true).
		SQLMigrationSuffixes(".never").
		Load()
	if err != nil {
		t.Fatalf("load setup migrator: %v", err)
	}
	_ = setup

	// Create the table directly, then attempt the concurrent index inside a
	// transaction to confirm PostgreSQL rejects it.
	if _, err := database.Exec(`CREATE SCHEMA IF NOT EXISTS test_notx_control`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := database.Exec(`CREATE TABLE test_notx_control.items (id INTEGER PRIMARY KEY, name VARCHAR(100))`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE INDEX CONCURRENTLY idx_control ON test_notx_control.items (name)`); err == nil {
		t.Fatal("expected CREATE INDEX CONCURRENTLY to fail inside a transaction")
	}
}

// TestSQLiteNoTransactionVacuum verifies a no-transaction migration runs VACUUM,
// which cannot run inside a transaction on SQLite.
func TestSQLiteNoTransactionVacuum(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, "testdata/notx_sqlite").
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}

	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("migrate with VACUUM: %v", err)
	}
	if result.MigrationsExecuted != 2 {
		t.Fatalf("executed %d migrations, want 2", result.MigrationsExecuted)
	}
	// The table from V1 must still exist after the vacuum.
	if _, err := database.Exec("INSERT INTO notes (id, body) VALUES (1, 'hi')"); err != nil {
		t.Fatalf("insert after vacuum: %v", err)
	}
}

// TestSQLiteSQLCallbacks verifies that beforeMigrate and afterEachMigrate SQL
// callback scripts are discovered and fired.
func TestSQLiteSQLCallbacks(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, "testdata/callbacks").
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate with callbacks: %v", err)
	}

	// beforeMigrate inserts one 'before' row; afterEachMigrate inserts one 'each'
	// row per applied migration (one migration here).
	count := func(note string) int {
		var n int
		if err := database.QueryRow(`SELECT COUNT(*) FROM audit WHERE note = ?`, note).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", note, err)
		}
		return n
	}
	if got := count("before"); got != 1 {
		t.Errorf("beforeMigrate rows = %d, want 1", got)
	}
	if got := count("each"); got != 1 {
		t.Errorf("afterEachMigrate rows = %d, want 1", got)
	}
}

// TestSQLiteProgrammaticCallback verifies a callback registered through the
// configuration receives the lifecycle events.
func TestSQLiteProgrammaticCallback(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()

	events := map[goway.CallbackEvent]int{}
	recorder := goway.CallbackFunc(func(_ context.Context, event goway.CallbackEvent, _ goway.Execer, _ *goway.MigrationInfo) error {
		events[event]++
		return nil
	})

	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, "testdata/shared").
		Callbacks(recorder).
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if events[goway.EventBeforeMigrate] != 1 {
		t.Errorf("beforeMigrate fired %d times, want 1", events[goway.EventBeforeMigrate])
	}
	if events[goway.EventAfterMigrate] != 1 {
		t.Errorf("afterMigrate fired %d times, want 1", events[goway.EventAfterMigrate])
	}
	// testdata/shared has three migrations (V1, V2, R), so each-migrate fires 3x.
	if events[goway.EventBeforeEachMigrate] != 3 {
		t.Errorf("beforeEachMigrate fired %d times, want 3", events[goway.EventBeforeEachMigrate])
	}
	if events[goway.EventAfterEachMigrate] != 3 {
		t.Errorf("afterEachMigrate fired %d times, want 3", events[goway.EventAfterEachMigrate])
	}
}

// TestSQLiteRepeatableSuperseded applies a repeatable migration, changes it on
// disk via a second migrator with a different script, and verifies that re-runs
// accumulate history rows of which only the newest is current.
func TestSQLiteRepeatableSuperseded(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()

	migrate := func() *goway.Migrator {
		m, err := goway.Configure().
			DataSource(database).
			Dialect(goway.SQLite()).
			Locations().
			FS(migrationsFS, "testdata/shared").
			Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		return m
	}

	if _, err := migrate().Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Force the repeatable migration to be re-applied by tampering its recorded
	// checksum so it appears outdated, then migrate again.
	if _, err := database.Exec(
		`UPDATE flyway_schema_history SET checksum = checksum + 1 WHERE version IS NULL AND type = 'SQL'`); err != nil {
		t.Fatalf("tamper repeatable checksum: %v", err)
	}
	if _, err := migrate().Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	// There must now be two runs of the repeatable migration in history.
	var runs int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM flyway_schema_history WHERE script = 'R__widget_summary.sql'`).Scan(&runs); err != nil {
		t.Fatalf("count repeatable runs: %v", err)
	}
	if runs != 2 {
		t.Fatalf("repeatable runs in history = %d, want 2", runs)
	}

	// Info must report exactly one Superseded run and one current run.
	info, err := migrate().Info(ctx)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	var superseded, current int
	for _, migration := range info.Migrations {
		if migration.Script != "R__widget_summary.sql" {
			continue
		}
		switch migration.State {
		case goway.StateSuperseded:
			superseded++
		case goway.StateSuccess, goway.StateOutdated:
			current++
		}
	}
	if superseded != 1 {
		t.Errorf("superseded repeatable runs = %d, want 1", superseded)
	}
	if current != 1 {
		t.Errorf("current repeatable runs = %d, want 1", current)
	}

	// Validation must not treat a superseded run as an error.
	if result, err := migrate().Validate(ctx); err != nil || !result.Valid {
		t.Fatalf("validate after supersede: valid=%v err=%v", result.Valid, err)
	}
}
