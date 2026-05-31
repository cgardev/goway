package goway

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// querier is the subset of the database/sql API shared by *sql.DB and *sql.Tx,
// allowing the history operations to run either on a pooled connection or inside
// a transaction.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// schemaHistory provides access to the schema history table for a single schema
// and table name.
type schemaHistory struct {
	dialect Dialect
	schema  string
	table   string
}

// newSchemaHistory builds a schema history accessor.
func newSchemaHistory(dialect Dialect, schema, table string) *schemaHistory {
	return &schemaHistory{dialect: dialect, schema: schema, table: table}
}

// exists reports whether the schema history table is present.
func (h *schemaHistory) exists(ctx context.Context, db querier) (bool, error) {
	query, args := h.dialect.tableExistsQuery(h.schema, h.table)
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// create creates the schema history table and its supporting objects.
func (h *schemaHistory) create(ctx context.Context, db querier) error {
	for _, statement := range h.dialect.createSchemaHistoryDDL(h.schema, h.table) {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("goway: creating schema history table: %w", err)
		}
	}
	return nil
}

// all reads every applied migration ordered by installed rank.
func (h *schemaHistory) all(ctx context.Context, db querier) ([]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, h.dialect.selectSchemaHistoryDML(h.schema, h.table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []appliedMigration
	for rows.Next() {
		var (
			record      appliedMigration
			version     sql.NullString
			checksum    sql.NullInt64
			installedBy sql.NullString
			installedOn scannedTime
			success     scannedBool
		)
		if err := rows.Scan(
			&record.installedRank,
			&version,
			&record.description,
			&record.migrationType,
			&record.script,
			&checksum,
			&installedBy,
			&installedOn,
			&record.executionTime,
			&success,
		); err != nil {
			return nil, err
		}

		if version.Valid && version.String != "" {
			parsed, parseErr := parseVersion(version.String)
			if parseErr != nil {
				return nil, fmt.Errorf("goway: reading recorded version %q: %w", version.String, parseErr)
			}
			record.version = parsed
		}
		if checksum.Valid {
			value := int32(checksum.Int64)
			record.checksum = &value
		}
		if installedBy.Valid {
			record.installedBy = installedBy.String
		}
		record.installedOn = installedOn.value
		record.success = bool(success)
		applied = append(applied, record)
	}
	return applied, rows.Err()
}

// scannedBool reads a boolean column regardless of whether the driver returns a
// native boolean, an integer, or a textual representation. PostgreSQL returns a
// native boolean while SQLite returns an integer.
type scannedBool bool

func (b *scannedBool) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		*b = false
	case bool:
		*b = scannedBool(value)
	case int64:
		*b = value != 0
	case float64:
		*b = value != 0
	case []byte:
		*b = parseTextualBool(string(value))
	case string:
		*b = parseTextualBool(value)
	default:
		return fmt.Errorf("goway: cannot read boolean from %T", src)
	}
	return nil
}

func parseTextualBool(value string) scannedBool {
	return scannedBool(value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "t"))
}

// scannedTime reads a timestamp column regardless of whether the driver returns
// a native time or a textual representation. PostgreSQL returns a native time
// while SQLite returns text formatted by the insert statement.
type scannedTime struct {
	value time.Time
}

func (t *scannedTime) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		t.value = time.Time{}
	case time.Time:
		t.value = value
	case []byte:
		return t.parse(string(value))
	case string:
		return t.parse(value)
	case int64:
		t.value = time.Unix(value, 0).UTC()
	default:
		return fmt.Errorf("goway: cannot read timestamp from %T", src)
	}
	return nil
}

func (t *scannedTime) parse(value string) error {
	if value == "" {
		t.value = time.Time{}
		return nil
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			t.value = parsed
			return nil
		}
	}
	return fmt.Errorf("goway: cannot parse timestamp %q", value)
}

// nextInstalledRank returns the rank that should be assigned to the next
// recorded migration.
func (h *schemaHistory) nextInstalledRank(ctx context.Context, db querier) (int, error) {
	query := fmt.Sprintf(`SELECT COALESCE(MAX("installed_rank"), 0) FROM %s`,
		h.dialect.qualify(h.schema, h.table))
	var maximum int
	if err := db.QueryRowContext(ctx, query).Scan(&maximum); err != nil {
		return 0, err
	}
	return maximum + 1, nil
}

// insert records a single migration in the history table.
func (h *schemaHistory) insert(ctx context.Context, db querier, record appliedMigration) error {
	var version any
	if record.version != nil {
		version = record.version.String()
	}
	var checksum any
	if record.checksum != nil {
		checksum = *record.checksum
	}
	_, err := db.ExecContext(ctx, h.dialect.insertSchemaHistoryDML(h.schema, h.table),
		record.installedRank,
		version,
		record.description,
		record.migrationType,
		record.script,
		checksum,
		record.installedBy,
		record.executionTime,
		record.success,
	)
	if err != nil {
		return fmt.Errorf("goway: recording migration %q: %w", record.script, err)
	}
	return nil
}

// deleteFailed removes every failed row from the history table.
func (h *schemaHistory) deleteFailed(ctx context.Context, db querier) (int64, error) {
	result, err := db.ExecContext(ctx, h.dialect.deleteFailedDML(h.schema, h.table))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// updateChecksum aligns the recorded checksum of a row with the resolved value.
func (h *schemaHistory) updateChecksum(ctx context.Context, db querier, installedRank int, checksum int32) error {
	_, err := db.ExecContext(ctx, h.dialect.updateChecksumDML(h.schema, h.table), checksum, installedRank)
	return err
}

// resolveInstalledBy determines the user to record for a migration. An explicit
// configuration value wins; otherwise the database is queried, and an empty
// string is used when the dialect has no concept of a user.
func resolveInstalledBy(ctx context.Context, db querier, dialect Dialect, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	query, args := dialect.currentUserQuery()
	if query == "" {
		return "", nil
	}
	var user string
	if err := db.QueryRowContext(ctx, query, args...).Scan(&user); err != nil {
		return "", err
	}
	return user, nil
}
