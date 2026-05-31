package goway

import (
	"context"
	"time"
)

// Migrator is the entry point for running migration commands against a database.
// It is created by Load and is safe to reuse for multiple commands. Each command
// re-reads the schema history so that concurrent changes are observed.
type Migrator struct {
	configuration *Configuration
	dialect       Dialect
	resolved      []*resolvedMigration
}

// Dialect returns the dialect that was configured or detected.
func (f *Migrator) Dialect() Dialect { return f.dialect }

// ValidationError describes a single problem found during validation.
type ValidationError struct {
	// Version is the migration version, empty for repeatable migrations.
	Version string
	// Description is the migration description.
	Description string
	// Script is the script file name or synthetic marker.
	Script string
	// Message explains the problem.
	Message string
}

// MigrateResult summarizes the outcome of a migrate command.
type MigrateResult struct {
	// InitialSchemaVersion is the current version before migrating.
	InitialSchemaVersion string
	// TargetSchemaVersion is the current version after migrating.
	TargetSchemaVersion string
	// MigrationsExecuted is the number of migrations applied.
	MigrationsExecuted int
	// Migrations lists the migrations applied during this run.
	Migrations []MigrationInfo
}

// InfoResult is the outcome of an info command.
type InfoResult struct {
	// Migrations lists every migration with its derived state.
	Migrations []MigrationInfo
	// Current is the current schema version.
	Current string
}

// ValidateResult is the outcome of a validate command.
type ValidateResult struct {
	// Valid reports whether validation found no problems.
	Valid bool
	// Errors lists the problems found.
	Errors []ValidationError
	// ValidationCount is the number of migrations validated.
	ValidationCount int
}

// BaselineResult is the outcome of a baseline command.
type BaselineResult struct {
	// BaselineVersion is the version recorded as the baseline.
	BaselineVersion string
	// BaselineDescription is the description recorded as the baseline.
	BaselineDescription string
	// Created reports whether a baseline entry was created by this command.
	Created bool
}

// RepairResult is the outcome of a repair command.
type RepairResult struct {
	// RemovedFailed is the number of failed rows removed.
	RemovedFailed int
	// AlignedChecksums is the number of rows whose checksum was realigned.
	AlignedChecksums int
}

// CleanResult is the outcome of a clean command.
type CleanResult struct {
	// SchemasCleaned lists the schemas that were cleaned.
	SchemasCleaned []string
}

// db returns the configured connection pool.
func (f *Migrator) db() querier { return f.configuration.db }

// resolveDefaultSchema determines the schema that holds the history table.
func (f *Migrator) resolveDefaultSchema(ctx context.Context) (string, error) {
	if f.configuration.defaultSchema != "" {
		return f.configuration.defaultSchema, nil
	}
	if len(f.configuration.schemas) > 0 {
		return f.configuration.schemas[0], nil
	}
	if f.dialect.supportsSchemas() {
		query, args := f.dialect.currentSchemaQuery()
		if query != "" {
			var schema string
			if err := f.configuration.db.QueryRowContext(ctx, query, args...).Scan(&schema); err != nil {
				return "", err
			}
			return schema, nil
		}
	}
	return "", nil
}

// history builds a schema history accessor for the given default schema.
func (f *Migrator) history(schema string) *schemaHistory {
	return newSchemaHistory(f.dialect, schema, f.configuration.table)
}

// ensureSchemas creates any configured schema that does not yet exist and
// returns the names of the schemas it created.
func (f *Migrator) ensureSchemas(ctx context.Context, db querier) ([]string, error) {
	if !f.configuration.createSchemas || !f.dialect.supportsSchemas() {
		return nil, nil
	}
	var created []string
	for _, schema := range f.configuration.schemas {
		if schema == "" {
			continue
		}
		query, args := f.dialect.schemaExistsQuery(schema)
		if query == "" {
			continue
		}
		var count int
		if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			continue
		}
		statement := f.dialect.createSchemaDDL(schema)
		if statement == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return nil, err
		}
		created = append(created, schema)
	}
	return created, nil
}

// loadApplied reads the applied migrations, returning an empty slice when the
// history table does not yet exist.
func (f *Migrator) loadApplied(ctx context.Context, history *schemaHistory) ([]appliedMigration, error) {
	exists, err := history.exists(ctx, f.db())
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return history.all(ctx, f.db())
}

// Info computes the state of every migration without changing the database.
func (f *Migrator) Info(ctx context.Context) (*InfoResult, error) {
	schema, err := f.resolveDefaultSchema(ctx)
	if err != nil {
		return nil, err
	}
	applied, err := f.loadApplied(ctx, f.history(schema))
	if err != nil {
		return nil, err
	}
	service := computeInfos(f.resolved, applied, f.configuration)
	return &InfoResult{
		Migrations: service.infos(),
		Current:    service.current.String(),
	}, nil
}

// nowUTC is used only where a timestamp must be supplied by the application; the
// history table itself records the database clock.
func nowUTC() time.Time { return time.Now().UTC() }
