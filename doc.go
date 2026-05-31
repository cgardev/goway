// Package goway provides version-based database schema migrations for Go,
// reimplementing the core behavior of the Java tool Flyway. It discovers
// versioned and repeatable SQL migration scripts, records what has been applied
// in a schema history table, and brings a database up to date by executing the
// pending scripts in order, each inside its own transaction.
//
// The package depends only on the standard library. Database access is performed
// through the database/sql package, so the calling application supplies its own
// driver and connection pool. Only PostgreSQL and SQLite are supported, the
// latter through the pure Go driver modernc.org/sqlite that avoids any binding
// to C.
//
// The entry point is Configure, which returns a Configuration that is populated
// with a fluent builder and finalized with Load:
//
//	migrator, err := goway.Configure().
//		DataSource(database).
//		Locations("filesystem:db/migration").
//		Schemas("public").
//		Load()
//	if err != nil {
//		return err
//	}
//	if _, err := migrator.Migrate(ctx); err != nil {
//		return err
//	}
//
// Migration scripts follow the same naming convention as Flyway. Versioned
// scripts are named V<version>__<description>.sql, for example
// V1__create_table.sql or V2.1__add_index.sql. Repeatable scripts are named
// R__<description>.sql and are re-executed whenever their checksum changes.
package goway
