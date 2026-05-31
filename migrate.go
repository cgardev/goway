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

	for _, entry := range service.pending() {
		info, err := f.applyMigration(ctx, db, history, entry, schema, installedBy)
		if err != nil {
			result.TargetSchemaVersion = f.currentVersionString(ctx, history)
			return result, err
		}
		result.Migrations = append(result.Migrations, info)
		result.MigrationsExecuted++
	}

	result.TargetSchemaVersion = f.currentVersionString(ctx, history)
	return result, nil
}

// applyMigration executes a single migration inside its own transaction and
// records it in the schema history. The history row is written in the same
// transaction as the migration statements, so a failure rolls back both.
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
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			_ = transaction.Rollback()
			return MigrationInfo{}, fmt.Errorf("goway: applying migration %s: %w", migration.script, err)
		}
	}
	executionTime := int(time.Since(start).Milliseconds())

	rank, err := history.nextInstalledRank(ctx, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}

	checksum := migration.checksum
	record := appliedMigration{
		installedRank: rank,
		version:       migration.version,
		description:   migration.description,
		migrationType: string(migration.migrationType),
		script:        migration.script,
		checksum:      &checksum,
		installedBy:   installedBy,
		executionTime: executionTime,
		success:       true,
	}
	if err := history.insert(ctx, transaction, record); err != nil {
		_ = transaction.Rollback()
		return MigrationInfo{}, err
	}
	if err := transaction.Commit(); err != nil {
		return MigrationInfo{}, fmt.Errorf("goway: committing migration %s: %w", migration.script, err)
	}

	state := StateSuccess
	if entry.outOfOrder {
		state = StateOutOfOrder
	}
	installedOn := nowUTC()
	return MigrationInfo{
		Version:       migration.version.String(),
		Description:   migration.description,
		Type:          string(migration.migrationType),
		Script:        migration.script,
		Checksum:      &checksum,
		State:         state,
		InstalledRank: rank,
		InstalledOn:   &installedOn,
		InstalledBy:   installedBy,
		ExecutionTime: executionTime,
	}, nil
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
