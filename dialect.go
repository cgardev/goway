package goway

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dialect abstracts the differences between the supported databases. The
// interface is sealed: its methods are unexported, so callers obtain a dialect
// only through the Postgres and SQLite constructors. This mirrors the way the
// reference query builder isolates database specific rendering behind a single
// abstraction.
type Dialect interface {
	// Name returns the canonical dialect name.
	Name() string

	// quoteIdentifier quotes a single identifier so it can be embedded safely in
	// generated statements.
	quoteIdentifier(name string) string

	// qualify returns the quoted, optionally schema qualified, table reference.
	qualify(schema, table string) string

	// createSchemaHistoryDDL returns the statements that create the schema
	// history table.
	createSchemaHistoryDDL(schema, table string) []string

	// insertSchemaHistoryDML returns the parameterized insert statement for a
	// single history row. The parameters, in order, are installed rank, version,
	// description, type, script, checksum, installed by, execution time and
	// success. The installed timestamp is supplied by the database.
	insertSchemaHistoryDML(schema, table string) string

	// selectSchemaHistoryDML returns the query that reads every history row
	// ordered by installed rank.
	selectSchemaHistoryDML(schema, table string) string

	// tableExistsQuery returns a query and its arguments that yield a count
	// greater than zero when the history table exists.
	tableExistsQuery(schema, table string) (string, []any)

	// currentUserQuery returns a query that yields the current database user, or
	// an empty query when the concept does not apply.
	currentUserQuery() (string, []any)

	// currentSchemaQuery returns a query that yields the default schema, or an
	// empty query when the concept does not apply.
	currentSchemaQuery() (string, []any)

	// supportsSchemas reports whether the database has addressable schemas.
	supportsSchemas() bool

	// createSchemaDDL returns the statement that creates a schema, or an empty
	// string when schemas are not supported.
	createSchemaDDL(schema string) string

	// schemaExistsQuery returns a query and its arguments that yield a count
	// greater than zero when the schema exists, or an empty query when schemas
	// are not supported.
	schemaExistsQuery(schema string) (string, []any)

	// deleteFailedDML returns the statement that removes failed rows from the
	// history table.
	deleteFailedDML(schema, table string) string

	// updateChecksumDML returns the parameterized statement that updates the
	// checksum of a history row identified by its installed rank. The parameters
	// are the new checksum followed by the installed rank.
	updateChecksumDML(schema, table string) string

	// splitStatements divides a raw script into individual executable
	// statements.
	splitStatements(sql string) ([]string, error)

	// setSearchPathSQL returns a statement that makes the given schema the
	// default for unqualified object names within a transaction, or an empty
	// string when the dialect has no such concept.
	setSearchPathSQL(schema string) string

	// sessionSearchPathSQL returns a statement that makes the given schema the
	// default for the whole session rather than a single transaction, for use by
	// migrations that run without a transaction. It returns an empty string when
	// the dialect has no such concept.
	sessionSearchPathSQL(schema string) string

	// cleanStatements returns the statements that drop every object in the given
	// schema, querying the database when the set of objects must be discovered
	// dynamically.
	cleanStatements(ctx context.Context, db querier, schema string) ([]string, error)
}

// Postgres returns the dialect for PostgreSQL.
func Postgres() Dialect { return postgresDialect{} }

// SQLite returns the dialect for SQLite.
func SQLite() Dialect { return sqliteDialect{} }

const (
	dialectPostgres = "postgresql"
	dialectSQLite   = "sqlite"
)

// detectDialect inspects a live connection to determine which dialect to use.
// SQLite exposes the sqlite_version function, which PostgreSQL does not, so the
// presence of that function is used as the discriminator.
func detectDialect(ctx context.Context, db *sql.DB) (Dialect, error) {
	var sqliteVersion string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&sqliteVersion); err == nil {
		return SQLite(), nil
	}

	var postgresVersion string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&postgresVersion); err == nil {
		if strings.Contains(strings.ToLower(postgresVersion), "postgresql") {
			return Postgres(), nil
		}
		return nil, fmt.Errorf("%w: reported version %q", ErrUnsupportedDialect, postgresVersion)
	}

	return nil, ErrNoDialect
}

// quoteWith wraps an identifier in the given quote character, doubling any
// embedded occurrence of that character so the result is a valid quoted
// identifier.
func quoteWith(identifier string, quote byte) string {
	escaped := strings.ReplaceAll(identifier, string(quote), string(quote)+string(quote))
	return string(quote) + escaped + string(quote)
}
