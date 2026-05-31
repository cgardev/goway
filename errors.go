package goway

import "errors"

// The following sentinel errors classify the failures that the package can
// report. They are returned wrapped with additional context, so callers should
// compare against them with errors.Is rather than by direct equality.
var (
	// ErrNoDataSource indicates that Load was called without configuring a
	// database connection through DataSource.
	ErrNoDataSource = errors.New("goway: no data source configured")

	// ErrNoDialect indicates that the database dialect could not be detected
	// automatically and was not provided explicitly through Dialect.
	ErrNoDialect = errors.New("goway: could not determine database dialect")

	// ErrUnsupportedDialect indicates that the configured or detected database
	// is not one of the supported dialects (PostgreSQL and SQLite).
	ErrUnsupportedDialect = errors.New("goway: unsupported database dialect")

	// ErrValidationFailed indicates that validation found at least one problem,
	// such as a checksum mismatch or a locally missing applied migration.
	ErrValidationFailed = errors.New("goway: validation failed")

	// ErrCleanDisabled indicates that Clean was invoked while the clean command
	// is disabled, which is the default for safety.
	ErrCleanDisabled = errors.New("goway: clean is disabled")

	// ErrFailedMigration indicates that the schema history contains a migration
	// that previously failed and must be resolved before migrating further.
	ErrFailedMigration = errors.New("goway: detected a previously failed migration")

	// ErrDuplicateVersion indicates that more than one resolved migration shares
	// the same version.
	ErrDuplicateVersion = errors.New("goway: found more than one migration with the same version")

	// ErrDuplicateRepeatable indicates that more than one resolved repeatable
	// migration shares the same description.
	ErrDuplicateRepeatable = errors.New("goway: found more than one repeatable migration with the same description")

	// ErrInvalidMigrationName indicates that a script name does not satisfy the
	// configured naming convention.
	ErrInvalidMigrationName = errors.New("goway: invalid migration name")

	// ErrOutOfOrder indicates that a pending migration has a version lower than
	// an already applied one while out of order execution is disabled.
	ErrOutOfOrder = errors.New("goway: detected an out of order migration")
)
