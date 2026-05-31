package goway

import (
	"context"
	"fmt"
)

// sqliteDialect implements Dialect for SQLite. SQLite has no addressable
// schemas and no concept of database users, so the schema related methods are
// either no-ops or report a lack of support. Bind parameters use the question
// mark form.
type sqliteDialect struct{}

func (sqliteDialect) Name() string { return dialectSQLite }

func (sqliteDialect) quoteIdentifier(name string) string { return quoteWith(name, '"') }

// qualify ignores the schema because SQLite addresses tables by name within a
// single attached database.
func (d sqliteDialect) qualify(_, table string) string {
	return d.quoteIdentifier(table)
}

func (d sqliteDialect) createSchemaHistoryDDL(_, table string) []string {
	return []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "installed_rank" INT NOT NULL PRIMARY KEY,
    "version" VARCHAR(50),
    "description" VARCHAR(200) NOT NULL,
    "type" VARCHAR(20) NOT NULL,
    "script" VARCHAR(1000) NOT NULL,
    "checksum" INTEGER,
    "installed_by" VARCHAR(100) NOT NULL,
    "installed_on" TIMESTAMP NOT NULL DEFAULT (strftime('%%Y-%%m-%%d %%H:%%M:%%f', 'now')),
    "execution_time" INTEGER NOT NULL,
    "success" BOOLEAN NOT NULL
)`, d.quoteIdentifier(table)),
	}
}

func (d sqliteDialect) insertSchemaHistoryDML(_, table string) string {
	return fmt.Sprintf(`INSERT INTO %s `+
		`("installed_rank","version","description","type","script","checksum","installed_by","installed_on","execution_time","success") `+
		`VALUES (?,?,?,?,?,?,?,strftime('%%Y-%%m-%%d %%H:%%M:%%f','now'),?,?)`, d.quoteIdentifier(table))
}

func (d sqliteDialect) selectSchemaHistoryDML(_, table string) string {
	return fmt.Sprintf(`SELECT "installed_rank","version","description","type","script","checksum",`+
		`"installed_by","installed_on","execution_time","success" FROM %s ORDER BY "installed_rank"`,
		d.quoteIdentifier(table))
}

func (sqliteDialect) tableExistsQuery(_, table string) (string, []any) {
	return `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, []any{table}
}

func (sqliteDialect) currentUserQuery() (string, []any) { return "", nil }

func (sqliteDialect) currentSchemaQuery() (string, []any) { return "", nil }

func (sqliteDialect) supportsSchemas() bool { return false }

func (sqliteDialect) createSchemaDDL(string) string { return "" }

func (sqliteDialect) schemaExistsQuery(string) (string, []any) { return "", nil }

func (d sqliteDialect) deleteFailedDML(_, table string) string {
	return fmt.Sprintf(`DELETE FROM %s WHERE "success" = 0`, d.quoteIdentifier(table))
}

func (d sqliteDialect) updateChecksumDML(_, table string) string {
	return fmt.Sprintf(`UPDATE %s SET "checksum" = ? WHERE "installed_rank" = ?`, d.quoteIdentifier(table))
}

func (sqliteDialect) splitStatements(sql string) ([]string, error) {
	return splitSQLStatements(sql, dialectSQLite)
}

func (sqliteDialect) setSearchPathSQL(string) string { return "" }

// cleanStatements enumerates the user defined objects from the SQLite catalog
// and returns statements to drop each of them. Internal objects whose names
// begin with the reserved prefix are skipped.
func (d sqliteDialect) cleanStatements(ctx context.Context, db querier, _ string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT type, name FROM sqlite_master WHERE type IN ('table','view','index','trigger') AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers, views, indexes, tables []string
	for rows.Next() {
		var objectType, name string
		if err := rows.Scan(&objectType, &name); err != nil {
			return nil, err
		}
		quoted := d.quoteIdentifier(name)
		switch objectType {
		case "trigger":
			triggers = append(triggers, "DROP TRIGGER IF EXISTS "+quoted)
		case "view":
			views = append(views, "DROP VIEW IF EXISTS "+quoted)
		case "index":
			indexes = append(indexes, "DROP INDEX IF EXISTS "+quoted)
		case "table":
			tables = append(tables, "DROP TABLE IF EXISTS "+quoted)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Foreign key enforcement is disabled for the duration of the clean so that
	// parent tables can be dropped regardless of declaration order; the caller
	// runs these statements on a single dedicated connection. Dependent objects
	// are still dropped before the tables they reference.
	statements := make([]string, 0, len(triggers)+len(views)+len(indexes)+len(tables)+2)
	statements = append(statements, "PRAGMA foreign_keys = OFF")
	statements = append(statements, triggers...)
	statements = append(statements, views...)
	statements = append(statements, indexes...)
	statements = append(statements, tables...)
	statements = append(statements, "PRAGMA foreign_keys = ON")
	return statements, nil
}
