package goway

import "context"

// Repair removes failed entries from the schema history and realigns the
// recorded checksums of applied migrations with the resolved scripts. It is the
// recommended remedy after a failed migration on a database without
// transactional data definition, or after a deliberate change to an already
// applied script.
func (f *Migrator) Repair(ctx context.Context) (*RepairResult, error) {
	schema, err := f.resolveDefaultSchema(ctx)
	if err != nil {
		return nil, err
	}
	db := f.configuration.db
	history := f.history(schema)

	exists, err := history.exists(ctx, db)
	if err != nil {
		return nil, err
	}
	result := &RepairResult{}
	if !exists {
		return result, nil
	}

	removed, err := history.deleteFailed(ctx, db)
	if err != nil {
		return nil, err
	}
	result.RemovedFailed = int(removed)

	applied, err := history.all(ctx, db)
	if err != nil {
		return nil, err
	}

	resolvedVersioned := make(map[string]*resolvedMigration)
	resolvedRepeatable := make(map[string]*resolvedMigration)
	for _, migration := range f.resolved {
		if migration.repeatable {
			resolvedRepeatable[migration.description] = migration
		} else {
			resolvedVersioned[normalizedVersionKey(migration.version)] = migration
		}
	}

	for _, record := range applied {
		if record.isSynthetic() {
			continue
		}
		var resolved *resolvedMigration
		if record.version != nil {
			resolved = resolvedVersioned[normalizedVersionKey(record.version)]
		} else {
			resolved = resolvedRepeatable[record.description]
		}
		if resolved == nil {
			continue
		}
		if record.checksum == nil || *record.checksum != resolved.checksum {
			if err := history.updateChecksum(ctx, db, record.installedRank, resolved.checksum); err != nil {
				return nil, err
			}
			result.AlignedChecksums++
		}
	}

	return result, nil
}
