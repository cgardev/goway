package goway

import "context"

// Clean drops every object in the managed schemas, returning the database to an
// empty state. It is disabled by default and must be enabled explicitly through
// CleanDisabled, since it is destructive and irreversible.
func (f *Migrator) Clean(ctx context.Context) (*CleanResult, error) {
	if f.configuration.cleanDisabled {
		return nil, ErrCleanDisabled
	}

	schemas := append([]string(nil), f.configuration.schemas...)
	if len(schemas) == 0 {
		schema, err := f.resolveDefaultSchema(ctx)
		if err != nil {
			return nil, err
		}
		schemas = []string{schema}
	}

	connection, err := f.configuration.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	result := &CleanResult{}
	for _, schema := range schemas {
		statements, err := f.dialect.cleanStatements(ctx, connection, schema)
		if err != nil {
			return nil, err
		}
		for _, statement := range statements {
			if _, err := connection.ExecContext(ctx, statement); err != nil {
				return nil, err
			}
		}
		result.SchemasCleaned = append(result.SchemasCleaned, schema)
	}
	return result, nil
}
