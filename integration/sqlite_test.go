package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/cgardev/goway"
)

func sqliteMigrator(t *testing.T, database *sql.DB, paths ...string) *goway.Migrator {
	t.Helper()
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, paths...).
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}
	return migrator
}

func TestSQLiteMigrateLifecycle(t *testing.T) {
	database := openSQLite(t)
	migrator := sqliteMigrator(t, database, "testdata/shared")
	ctx := context.Background()

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

	// The schema produced by the migrations must be usable.
	if _, err := database.Exec("INSERT INTO widgets (id, name, price) VALUES (1, 'gear', 5)"); err != nil {
		t.Fatalf("insert into migrated table: %v", err)
	}

	var historyCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM flyway_schema_history`).Scan(&historyCount); err != nil {
		t.Fatalf("count history rows: %v", err)
	}
	if historyCount != 3 {
		t.Fatalf("history rows = %d, want 3", historyCount)
	}
}

func TestSQLiteMigrateIsIdempotent(t *testing.T) {
	database := openSQLite(t)
	migrator := sqliteMigrator(t, database, "testdata/shared")
	ctx := context.Background()

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if result.MigrationsExecuted != 0 {
		t.Fatalf("second migrate executed %d migrations, want 0", result.MigrationsExecuted)
	}
}

func TestSQLiteInfoReflectsState(t *testing.T) {
	database := openSQLite(t)
	migrator := sqliteMigrator(t, database, "testdata/shared")
	ctx := context.Background()

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	information, err := migrator.Info(ctx)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if information.Current != "2" {
		t.Fatalf("current = %q, want 2", information.Current)
	}
	if len(information.Migrations) != 3 {
		t.Fatalf("info reported %d migrations, want 3", len(information.Migrations))
	}
	for _, migration := range information.Migrations {
		if migration.State != goway.StateSuccess {
			t.Errorf("migration %q has state %q, want Success", migration.Script, migration.State)
		}
	}
}

func TestSQLiteValidateDetectsChecksumMismatch(t *testing.T) {
	database := openSQLite(t)
	migrator := sqliteMigrator(t, database, "testdata/shared")
	ctx := context.Background()

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result, err := migrator.Validate(ctx); err != nil || !result.Valid {
		t.Fatalf("validate after migrate: valid=%v err=%v", result.Valid, err)
	}

	if _, err := database.Exec(`UPDATE flyway_schema_history SET checksum = 123456 WHERE version = '1'`); err != nil {
		t.Fatalf("tamper checksum: %v", err)
	}
	result, err := migrator.Validate(ctx)
	if err == nil {
		t.Fatal("expected validation error after tampering checksum")
	}
	if !errors.Is(err, goway.ErrValidationFailed) {
		t.Fatalf("error = %v, want ErrValidationFailed", err)
	}
	if result.Valid {
		t.Fatal("validation reported valid after tampering")
	}
}

func TestSQLiteTriggerBodyIsAppliedAsOneStatement(t *testing.T) {
	database := openSQLite(t)
	migrator := sqliteMigrator(t, database, "testdata/sqlite")
	ctx := context.Background()

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate with trigger: %v", err)
	}

	// Inserting a log row must fire the trigger, which writes to log_audit.
	if _, err := database.Exec("INSERT INTO logs (id, message) VALUES (1, 'hello')"); err != nil {
		t.Fatalf("insert log: %v", err)
	}
	var auditRows int
	if err := database.QueryRow("SELECT COUNT(*) FROM log_audit").Scan(&auditRows); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditRows == 0 {
		t.Fatal("trigger did not fire; trigger body was likely split incorrectly")
	}
}

func TestSQLitePlaceholderReplacement(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()
	// The placeholder script references ${table_name}; the configured key is in a
	// different case to verify case insensitive matching.
	migrator, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, "testdata/placeholder").
		Placeholders(map[string]string{"TABLE_NAME": "things"}).
		Load()
	if err != nil {
		t.Fatalf("load migrator: %v", err)
	}

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("migrate with placeholder: %v", err)
	}
	if _, err := database.Exec("INSERT INTO things (id) VALUES (1)"); err != nil {
		t.Fatalf("table from placeholder was not created: %v", err)
	}
}

func TestSQLiteCleanRemovesObjects(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()

	if _, err := sqliteMigrator(t, database, "testdata/shared").Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cleaner, err := goway.Configure().
		DataSource(database).
		Dialect(goway.SQLite()).
		Locations().
		FS(migrationsFS, "testdata/shared").
		CleanDisabled(false).
		Load()
	if err != nil {
		t.Fatalf("load cleaner: %v", err)
	}
	if _, err := cleaner.Clean(ctx); err != nil {
		t.Fatalf("clean: %v", err)
	}

	var name string
	err = database.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'widgets'`).Scan(&name)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("widgets table still present after clean (err=%v)", err)
	}
}

func TestSQLiteCleanDisabledByDefault(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()
	migrator := sqliteMigrator(t, database, "testdata/shared")
	if _, err := migrator.Clean(ctx); !errors.Is(err, goway.ErrCleanDisabled) {
		t.Fatalf("clean error = %v, want ErrCleanDisabled", err)
	}
}

func TestSQLiteBaselineSkipsMigrationsAtOrBelowBaseline(t *testing.T) {
	database := openSQLite(t)
	ctx := context.Background()

	// Simulate an existing database already at version 1.
	if _, err := database.Exec("CREATE TABLE widgets (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL)"); err != nil {
		t.Fatalf("seed existing schema: %v", err)
	}

	migrator := sqliteMigrator(t, database, "testdata/shared")
	baseline, err := migrator.Baseline(ctx)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if !baseline.Created {
		t.Fatal("baseline was not created")
	}

	result, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatalf("migrate after baseline: %v", err)
	}
	// V1 is at the baseline and is skipped; only V2 and the repeatable run.
	if result.MigrationsExecuted != 2 {
		t.Fatalf("executed %d migrations after baseline, want 2", result.MigrationsExecuted)
	}
}
