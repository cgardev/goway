package goway

import (
	"context"
	"fmt"
)

// Baseline records a baseline entry in the schema history so that existing
// databases can be brought under management without re-running the migrations
// that produced their current state. Migrations at or below the baseline version
// are subsequently ignored.
func (f *Migrator) Baseline(ctx context.Context) (*BaselineResult, error) {
	schema, err := f.resolveDefaultSchema(ctx)
	if err != nil {
		return nil, err
	}
	db := f.configuration.db
	history := f.history(schema)

	if _, err := f.ensureSchemas(ctx, db); err != nil {
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
	}

	applied, err := history.all(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, record := range applied {
		switch record.migrationType {
		case string(MigrationTypeSchema):
			continue
		case string(MigrationTypeBaseline):
			return &BaselineResult{
				BaselineVersion:     f.configuration.baselineVersion.String(),
				BaselineDescription: f.configuration.baselineDescription,
				Created:             false,
			}, nil
		default:
			return nil, fmt.Errorf("goway: cannot baseline a schema history that already contains applied migrations")
		}
	}

	installedBy, err := resolveInstalledBy(ctx, db, f.dialect, f.configuration.installedBy)
	if err != nil {
		return nil, err
	}
	if err := f.insertBaseline(ctx, db, history, installedBy); err != nil {
		return nil, err
	}

	return &BaselineResult{
		BaselineVersion:     f.configuration.baselineVersion.String(),
		BaselineDescription: f.configuration.baselineDescription,
		Created:             true,
	}, nil
}

// insertBaseline writes the baseline row into the history table.
func (f *Migrator) insertBaseline(ctx context.Context, db querier, history *schemaHistory, installedBy string) error {
	rank, err := history.nextInstalledRank(ctx, db)
	if err != nil {
		return err
	}
	return history.insert(ctx, db, appliedMigration{
		installedRank: rank,
		version:       f.configuration.baselineVersion,
		description:   f.configuration.baselineDescription,
		migrationType: string(MigrationTypeBaseline),
		script:        f.configuration.baselineDescription,
		installedBy:   installedBy,
		success:       true,
	})
}
