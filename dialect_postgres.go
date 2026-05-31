package goway

import (
	"context"
	"fmt"
)

// postgresDialect implements Dialect for PostgreSQL. Identifiers are quoted with
// double quotes, bind parameters use the ordinal form, and data definition
// statements participate in transactions.
type postgresDialect struct{}

func (postgresDialect) Name() string { return dialectPostgres }

func (postgresDialect) quoteIdentifier(name string) string { return quoteWith(name, '"') }

func (d postgresDialect) qualify(schema, table string) string {
	if schema == "" {
		return d.quoteIdentifier(table)
	}
	return d.quoteIdentifier(schema) + "." + d.quoteIdentifier(table)
}

func (d postgresDialect) createSchemaHistoryDDL(schema, table string) []string {
	reference := d.qualify(schema, table)
	return []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "installed_rank" INT NOT NULL,
    "version" VARCHAR(50),
    "description" VARCHAR(200) NOT NULL,
    "type" VARCHAR(20) NOT NULL,
    "script" VARCHAR(1000) NOT NULL,
    "checksum" INTEGER,
    "installed_by" VARCHAR(100) NOT NULL,
    "installed_on" TIMESTAMP NOT NULL DEFAULT now(),
    "execution_time" INTEGER NOT NULL,
    "success" BOOLEAN NOT NULL,
    CONSTRAINT %s PRIMARY KEY ("installed_rank")
)`, reference, d.quoteIdentifier(table+"_pk")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s ("success")`, d.quoteIdentifier(table+"_s_idx"), reference),
	}
}

func (d postgresDialect) insertSchemaHistoryDML(schema, table string) string {
	return fmt.Sprintf(`INSERT INTO %s `+
		`("installed_rank","version","description","type","script","checksum","installed_by","installed_on","execution_time","success") `+
		`VALUES ($1,$2,$3,$4,$5,$6,$7,now(),$8,$9)`, d.qualify(schema, table))
}

func (d postgresDialect) selectSchemaHistoryDML(schema, table string) string {
	return fmt.Sprintf(`SELECT "installed_rank","version","description","type","script","checksum",`+
		`"installed_by","installed_on","execution_time","success" FROM %s ORDER BY "installed_rank"`,
		d.qualify(schema, table))
}

func (postgresDialect) tableExistsQuery(schema, table string) (string, []any) {
	return `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2`,
		[]any{schema, table}
}

func (postgresDialect) currentUserQuery() (string, []any) {
	return "SELECT current_user", nil
}

func (postgresDialect) currentSchemaQuery() (string, []any) {
	return "SELECT current_schema()", nil
}

func (postgresDialect) supportsSchemas() bool { return true }

func (d postgresDialect) createSchemaDDL(schema string) string {
	return "CREATE SCHEMA IF NOT EXISTS " + d.quoteIdentifier(schema)
}

func (postgresDialect) schemaExistsQuery(schema string) (string, []any) {
	return `SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = $1`, []any{schema}
}

func (d postgresDialect) deleteFailedDML(schema, table string) string {
	return fmt.Sprintf(`DELETE FROM %s WHERE "success" = false`, d.qualify(schema, table))
}

func (d postgresDialect) updateChecksumDML(schema, table string) string {
	return fmt.Sprintf(`UPDATE %s SET "checksum" = $1 WHERE "installed_rank" = $2`, d.qualify(schema, table))
}

func (postgresDialect) splitStatements(sql string) ([]string, error) {
	return splitSQLStatements(sql, dialectPostgres)
}

func (d postgresDialect) setSearchPathSQL(schema string) string {
	if schema == "" {
		return ""
	}
	return "SET LOCAL search_path TO " + d.quoteIdentifier(schema)
}

func (d postgresDialect) sessionSearchPathSQL(schema string) string {
	if schema == "" {
		return ""
	}
	return "SET search_path TO " + d.quoteIdentifier(schema)
}

// cleanStatements drops the schema and recreates it, which removes every object
// it contains. This is simpler and more robust than enumerating each object.
func (d postgresDialect) cleanStatements(_ context.Context, _ querier, schema string) ([]string, error) {
	if schema == "" {
		return nil, fmt.Errorf("goway: a schema is required to clean a PostgreSQL database")
	}
	quoted := d.quoteIdentifier(schema)
	return []string{
		"DROP SCHEMA IF EXISTS " + quoted + " CASCADE",
		"CREATE SCHEMA " + quoted,
	}, nil
}
