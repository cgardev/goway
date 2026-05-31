package goway

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"strings"
	"time"
)

// Migrate brings the database up to date by applying every pending migration in
// order. Missing schemas and the schema history table are created when needed.
// Each migration runs inside its own transaction, so a failed migration leaves
// no partial state and can be retried after the script is corrected.
func (f *Migrator) Migrate(ctx context.Context) (*MigrateResult, error) {
	schema, err := f.resolveDefaultSchema(ctx)
	if err != nil {
		return nil, err
	}
	db := f.configuration.db
	history := f.history(schema)

	// Only one migrator may run at a time. On PostgreSQL this is enforced with a
	// session advisory lock; SQLite is single writer by nature.
	release, err := f.acquireLock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	installedBy, err := resolveInstalledBy(ctx, db, f.dialect, f.configuration.installedBy)
	if err != nil {
		return nil, err
	}

	createdSchemas, err := f.ensureSchemas(ctx, db)
	if err != nil {
		return nil, err
	}

	existed, err := history.exists(ctx, db)
	if err != nil {
		return nil, err
	}
	if !existed {
		if err := history.create(ctx, db); err != nil {
			return nil, err
		}
		if len(createdSchemas) > 0 {
			if err := f.recordSchemaCreation(ctx, db, history, createdSchemas, installedBy); err != nil {
				return nil, err
			}
		}
		if f.configuration.baselineOnMigrate {
			if err := f.insertBaseline(ctx, db, history, installedBy); err != nil {
				return nil, err
			}
		}
	}

	applied, err := history.all(ctx, db)
	if err != nil {
		return nil, err
	}
	service := computeInfos(f.resolved, applied, f.configuration)
	result := &MigrateResult{InitialSchemaVersion: service.current.String()}

	if f.configuration.validateOnMigrate {
		if problems := service.validate(true); len(problems) > 0 {
			return nil, validationError(problems)
		}
	}

	if err := f.fireCallbacks(ctx, db, EventBeforeMigrate, nil, schema); err != nil {
		return nil, err
	}

	for _, entry := range service.pending() {
		info, err := f.applyMigration(ctx, db, history, entry, schema, installedBy)
		if err != nil {
			result.TargetSchemaVersion = f.currentVersionString(ctx, history)
			return result, err
		}
		result.Migrations = append(result.Migrations, info)
		result.MigrationsExecuted++
	}

	if err := f.fireCallbacks(ctx, db, EventAfterMigrate, nil, schema); err != nil {
		result.TargetSchemaVersion = f.currentVersionString(ctx, history)
		return result, err
	}

	result.TargetSchemaVersion = f.currentVersionString(ctx, history)
	return result, nil
}

// applyMigration prepares a migration's statements and applies them either
// within a transaction or, when the script opted out, directly on a dedicated
// connection.
func (f *Migrator) applyMigration(ctx context.Context, db *sql.DB, history *schemaHistory, entry *migrationInfoEntry, schema, installedBy string) (MigrationInfo, error) {
	migration := entry.resolved

	content, err := migration.read()
	if err != nil {
		return MigrationInfo{}, fmt.Errorf("goway: reading migration %s: %w", migration.script, err)
	}
	script, err := replacePlaceholders(string(content),
		f.effectivePlaceholders(schema, installedBy, migration.script),
		f.configuration.placeholderPrefix, f.configuration.placeholderSuffix)
	if err != nil {
		return MigrationInfo{}, err
	}
	statements, err := f.dialect.splitStatements(script)
	if err != nil {
		return MigrationInfo{}, err
	}

	info := MigrationInfo{
		Version:     migration.version.String(),
		Description: migration.description,
		Type:        string(migration.migrationType),
		Script:      migration.script,
	}

	if migration.noTransaction {
		return f.applyWithoutTransaction(ctx, db, history, migration, statements, schema, installedBy, info, entry.outOfOrder)
	}
	return f.applyWithinTransaction(ctx, db, history, migration, statements, schema, installedBy, info, entry.outOfOrder)
}

// applyWithinTransaction runs the migration and records its history row inside a
// single transaction, so a failure rolls back both, leaving no partial state.
func (f *Migrator) applyWithinTransaction(ctx context.Context, db *sql.DB, history *schemaHistory, migration *resolvedMigration, statements []string, schema, installedBy string, info MigrationInfo, outOfOrder bool) (MigrationInfo, error) {
	transaction, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MigrationInfo{}, err
	}

	// Direct unqualified objects into the default schema for the duration of the
	// transaction so a configured schema is honored by SQL that does not qualify
	// its object names.
	if searchPath := f.dialect.setSearchPathSQL(schema); searchPath != "" {
		if _, err := transaction.ExecContext(ctx, searchPath); err != nil {
			_ = transaction.Rollback()
			return MigrationInfo{}, fmt.Errorf("goway: setting search path for %s: %w", migration.script, err)
		}
	}

	start := time.Now()
	if err := f.fireCallbacks(ctx, transaction, EventBeforeEachMigrate, &info, schema); err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			_ = transaction.Rollback()
			return MigrationInfo{}, fmt.Errorf("goway: applying migration %s: %w", migration.script, err)
		}
	}
	if err := f.fireCallbacks(ctx, transaction, EventAfterEachMigrate, &info, schema); err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}
	executionTime := int(time.Since(start).Milliseconds())

	rank, err := history.nextInstalledRank(ctx, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}
	if err := history.insert(ctx, transaction, f.buildRecord(migration, rank, installedBy, executionTime, true)); err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}
	if err := transaction.Commit(); err != nil {
		return MigrationInfo{}, fmt.Errorf("goway: committing migration %s: %w", migration.script, err)
	}
	return f.finishInfo(info, migration, rank, installedBy, executionTime, outOfOrder), nil
}

// applyWithoutTransaction runs a migration that opted out of the transaction,
// for statements such as PostgreSQL's CREATE INDEX CONCURRENTLY or SQLite's
// VACUUM that cannot run inside a transaction block. It uses a dedicated
// connection and, on failure, records a failed history row since there is no
// transaction to roll back.
func (f *Migrator) applyWithoutTransaction(ctx context.Context, db *sql.DB, history *schemaHistory, migration *resolvedMigration, statements []string, schema, installedBy string, info MigrationInfo, outOfOrder bool) (MigrationInfo, error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return MigrationInfo{}, err
	}
	defer connection.Close()

	if searchPath := f.dialect.sessionSearchPathSQL(schema); searchPath != "" {
		if _, err := connection.ExecContext(ctx, searchPath); err != nil {
			return MigrationInfo{}, fmt.Errorf("goway: setting search path for %s: %w", migration.script, err)
		}
	}

	start := time.Now()
	runErr := f.runStatements(ctx, connection, migration, statements, &info, schema)
	executionTime := int(time.Since(start).Milliseconds())

	rank, err := history.nextInstalledRank(ctx, connection)
	if err != nil {
		if runErr != nil {
			return MigrationInfo{}, runErr
		}
		return MigrationInfo{}, err
	}

	if runErr != nil {
		// Best effort: record the failure so it is visible and can be repaired.
		_ = history.insert(ctx, connection, f.buildRecord(migration, rank, installedBy, executionTime, false))
		return MigrationInfo{}, runErr
	}
	if err := history.insert(ctx, connection, f.buildRecord(migration, rank, installedBy, executionTime, true)); err != nil {
		return MigrationInfo{}, err
	}
	return f.finishInfo(info, migration, rank, installedBy, executionTime, outOfOrder), nil
}

// runStatements fires the per-migration callbacks around the migration's own
// statements on the given executor.
func (f *Migrator) runStatements(ctx context.Context, exec Execer, migration *resolvedMigration, statements []string, info *MigrationInfo, schema string) error {
	if err := f.fireCallbacks(ctx, exec, EventBeforeEachMigrate, info, schema); err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := exec.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("goway: applying migration %s: %w", migration.script, err)
		}
	}
	return f.fireCallbacks(ctx, exec, EventAfterEachMigrate, info, schema)
}

// buildRecord assembles the schema history row for an applied migration.
func (f *Migrator) buildRecord(migration *resolvedMigration, rank int, installedBy string, executionTime int, success bool) appliedMigration {
	checksum := migration.checksum
	return appliedMigration{
		installedRank: rank,
		version:       migration.version,
		description:   migration.description,
		migrationType: string(migration.migrationType),
		script:        migration.script,
		checksum:      &checksum,
		installedBy:   installedBy,
		executionTime: executionTime,
		success:       success,
	}
}

// finishInfo completes the public migration info for a successfully applied
// migration.
func (f *Migrator) finishInfo(info MigrationInfo, migration *resolvedMigration, rank int, installedBy string, executionTime int, outOfOrder bool) MigrationInfo {
	state := StateSuccess
	if outOfOrder {
		state = StateOutOfOrder
	}
	checksum := migration.checksum
	installedOn := nowUTC()
	info.Checksum = &checksum
	info.State = state
	info.InstalledRank = rank
	info.InstalledOn = &installedOn
	info.InstalledBy = installedBy
	info.ExecutionTime = executionTime
	return info
}

// fireCallbacks runs the SQL callback scripts and the programmatic callbacks
// registered for the given event, in that order.
func (f *Migrator) fireCallbacks(ctx context.Context, exec Execer, event CallbackEvent, migration *MigrationInfo, schema string) error {
	for _, callback := range f.sqlCallbacks {
		if callback.event != event {
			continue
		}
		if err := f.runSQLCallback(ctx, exec, callback, schema); err != nil {
			return err
		}
	}
	for _, callback := range f.configuration.callbacks {
		if err := callback.Handle(ctx, event, exec, migration); err != nil {
			return fmt.Errorf("goway: callback for %s: %w", event, err)
		}
	}
	return nil
}

// runSQLCallback executes the statements of a single SQL callback script.
func (f *Migrator) runSQLCallback(ctx context.Context, exec Execer, callback sqlCallback, schema string) error {
	content, err := callback.read()
	if err != nil {
		return fmt.Errorf("goway: reading callback %s: %w", callback.script, err)
	}
	script, err := replacePlaceholders(string(content),
		f.effectivePlaceholders(schema, "", callback.script),
		f.configuration.placeholderPrefix, f.configuration.placeholderSuffix)
	if err != nil {
		return err
	}
	statements, err := f.dialect.splitStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := exec.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("goway: callback %s: %w", callback.script, err)
		}
	}
	return nil
}

// recordSchemaCreation writes the synthetic entry that documents schemas created
// on the caller's behalf.
func (f *Migrator) recordSchemaCreation(ctx context.Context, db querier, history *schemaHistory, schemas []string, installedBy string) error {
	rank, err := history.nextInstalledRank(ctx, db)
	if err != nil {
		return err
	}
	return history.insert(ctx, db, appliedMigration{
		installedRank: rank,
		description:   "<< Flyway Schema Creation >>",
		migrationType: string(MigrationTypeSchema),
		script:        strings.Join(schemas, ","),
		installedBy:   installedBy,
		success:       true,
	})
}

// currentVersionString reloads the history and returns the current version, or
// an empty string when it cannot be determined.
func (f *Migrator) currentVersionString(ctx context.Context, history *schemaHistory) string {
	applied, err := history.all(ctx, f.configuration.db)
	if err != nil {
		return ""
	}
	return computeInfos(f.resolved, applied, f.configuration).current.String()
}

// effectivePlaceholders merges the configured placeholders with the built in
// flyway scoped placeholders that the reference implementation always provides.
// Keys are lower cased so lookups are case insensitive; configured values take
// precedence over the defaults.
func (f *Migrator) effectivePlaceholders(schema, installedBy, script string) map[string]string {
	merged := make(map[string]string, len(f.configuration.placeholders)+4)
	merged["flyway:defaultschema"] = schema
	merged["flyway:user"] = installedBy
	merged["flyway:table"] = f.configuration.table
	merged["flyway:filename"] = script
	for key, value := range f.configuration.placeholders {
		merged[key] = value
	}
	return merged
}

// acquireLock obtains an exclusive lock so that only one migrator proceeds at a
// time, returning a function that releases it. On PostgreSQL a session advisory
// lock is held on a dedicated connection; other dialects need no lock.
func (f *Migrator) acquireLock(ctx context.Context) (func(), error) {
	if f.dialect.Name() != dialectPostgres {
		return func() {}, nil
	}
	connection, err := f.configuration.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	key := advisoryLockKey(f.configuration.table)
	if _, err := connection.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return func() {
		_, _ = connection.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		_ = connection.Close()
	}, nil
}

// advisoryLockKey derives a stable PostgreSQL advisory lock key from the schema
// history table name.
func advisoryLockKey(table string) int64 {
	return int64(int32(crc32.ChecksumIEEE([]byte("goway:" + table))))
}
