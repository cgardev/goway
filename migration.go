package goway

import "time"

// MigrationType classifies the kind of entry recorded in the schema history
// table. The string values match those written by Flyway so that a history
// table can be shared with the reference implementation.
type MigrationType string

const (
	// MigrationTypeSQL marks a migration backed by a SQL script.
	MigrationTypeSQL MigrationType = "SQL"

	// MigrationTypeBaseline marks the synthetic entry created by the baseline
	// command to establish a starting point.
	MigrationTypeBaseline MigrationType = "BASELINE"

	// MigrationTypeSchema marks the synthetic entry created when the migrator
	// creates one or more schemas on the caller's behalf.
	MigrationTypeSchema MigrationType = "SCHEMA"

	// MigrationTypeDeleted marks an entry that was removed from consideration by
	// the repair command.
	MigrationTypeDeleted MigrationType = "DELETED"
)

// MigrationState describes the relationship between a resolved migration script
// and the recorded history for a single migration.
type MigrationState string

const (
	// StatePending indicates a resolved migration that has not been applied.
	StatePending MigrationState = "Pending"

	// StateSuccess indicates a migration that was applied successfully.
	StateSuccess MigrationState = "Success"

	// StateFailed indicates a migration whose application failed.
	StateFailed MigrationState = "Failed"

	// StateOutOfOrder indicates a migration that was applied out of order.
	StateOutOfOrder MigrationState = "Out of Order"

	// StateOutdated indicates a repeatable migration whose script changed since
	// it was last applied and will be re-applied.
	StateOutdated MigrationState = "Outdated"

	// StateMissing indicates a successfully applied migration that can no longer
	// be resolved from the configured locations.
	StateMissing MigrationState = "Missing"

	// StateMissingFailed indicates a failed migration that can no longer be
	// resolved from the configured locations.
	StateMissingFailed MigrationState = "Failed (Missing)"

	// StateFuture indicates an applied migration with a version higher than any
	// resolved migration.
	StateFuture MigrationState = "Future"

	// StateFutureFailed indicates a failed applied migration with a version
	// higher than any resolved migration.
	StateFutureFailed MigrationState = "Failed (Future)"

	// StateAboveTarget indicates a pending migration above the configured target.
	StateAboveTarget MigrationState = "Above Target"

	// StateIgnored indicates a pending migration that will be skipped, for
	// example because it is below the baseline.
	StateIgnored MigrationState = "Ignored"

	// StateBaseline indicates the synthetic baseline entry.
	StateBaseline MigrationState = "Baseline"
)

// resolvedMigration is a migration script discovered in the configured
// locations, together with the information needed to apply it.
type resolvedMigration struct {
	// version is the parsed version, or nil for repeatable migrations.
	version *Version

	// description is the human readable description.
	description string

	// script is the file name used for logging and stored in the history table.
	script string

	// checksum is the CRC32 checksum of the script content.
	checksum int32

	// repeatable reports whether the migration is repeatable.
	repeatable bool

	// migrationType classifies the migration for the history table.
	migrationType MigrationType

	// read returns the raw script content.
	read func() ([]byte, error)
}

// appliedMigration is a single row read from the schema history table.
type appliedMigration struct {
	installedRank int
	version       *Version
	description   string
	migrationType string
	script        string
	checksum      *int32
	installedBy   string
	installedOn   time.Time
	executionTime int
	success       bool
}

// isVersioned reports whether the applied migration carries a version.
func (a appliedMigration) isVersioned() bool {
	return a.version != nil
}

// isSynthetic reports whether the applied entry is a baseline or schema marker
// rather than a real migration script.
func (a appliedMigration) isSynthetic() bool {
	return a.migrationType == string(MigrationTypeBaseline) ||
		a.migrationType == string(MigrationTypeSchema)
}

// MigrationInfo is the public, read only view of a single migration that
// combines what was resolved from disk with what was recorded in history.
type MigrationInfo struct {
	// Version is the migration version, or an empty string for repeatable
	// migrations and the schema marker.
	Version string

	// Description is the human readable description.
	Description string

	// Type classifies the migration, for example "SQL" or "BASELINE".
	Type string

	// Script is the script file name or synthetic marker name.
	Script string

	// Checksum is the resolved checksum when available.
	Checksum *int32

	// State describes the relationship between the script and the history.
	State MigrationState

	// InstalledRank is the order in which the migration was applied, or zero when
	// it has not been applied.
	InstalledRank int

	// InstalledOn is the application timestamp, or nil when not applied.
	InstalledOn *time.Time

	// InstalledBy is the database user that applied the migration.
	InstalledBy string

	// ExecutionTime is the duration of the application in milliseconds.
	ExecutionTime int
}
