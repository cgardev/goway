package goway

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
)

// fsLocation pairs a file system with the directory paths inside it that hold
// migration scripts.
type fsLocation struct {
	fileSystem fs.FS
	paths      []string
}

// Configuration collects every setting that controls how migrations are
// discovered and applied. It is created with Configure, populated through the
// fluent setters, and finalized with Load. Each setter returns the same
// Configuration so calls can be chained.
type Configuration struct {
	db      *sql.DB
	dialect Dialect

	locations   []string
	fsLocations []fsLocation

	schemas       []string
	defaultSchema string
	createSchemas bool
	table         string

	baselineVersion     *Version
	baselineDescription string
	baselineOnMigrate   bool

	placeholders      map[string]string
	placeholderPrefix string
	placeholderSuffix string

	sqlMigrationPrefix           string
	repeatableSQLMigrationPrefix string
	sqlMigrationSeparator        string
	sqlMigrationSuffixes         []string

	validateOnMigrate bool
	cleanDisabled     bool
	outOfOrder        bool
	target            *Version
	installedBy       string

	configErr error
}

// Configure creates a Configuration populated with the same defaults as Flyway.
func Configure() *Configuration {
	baseline, _ := parseVersion("1")
	return &Configuration{
		locations:                    []string{"db/migration"},
		createSchemas:                true,
		table:                        "flyway_schema_history",
		baselineVersion:              baseline,
		baselineDescription:          "<< Flyway Baseline >>",
		placeholders:                 map[string]string{},
		placeholderPrefix:            "${",
		placeholderSuffix:            "}",
		sqlMigrationPrefix:           "V",
		repeatableSQLMigrationPrefix: "R",
		sqlMigrationSeparator:        "__",
		sqlMigrationSuffixes:         []string{".sql"},
		validateOnMigrate:            true,
		cleanDisabled:                true,
	}
}

// DataSource sets the database connection pool used for every operation.
func (c *Configuration) DataSource(db *sql.DB) *Configuration {
	c.db = db
	return c
}

// Dialect sets the database dialect explicitly, bypassing automatic detection.
func (c *Configuration) Dialect(dialect Dialect) *Configuration {
	c.dialect = dialect
	return c
}

// Locations replaces the list of file system locations scanned for migrations.
// A location may carry a "filesystem:" or "classpath:" prefix; a bare path is
// also accepted.
func (c *Configuration) Locations(locations ...string) *Configuration {
	c.locations = append([]string(nil), locations...)
	return c
}

// FS registers an additional file system, such as one produced by go:embed,
// together with the directory paths inside it that contain migrations.
func (c *Configuration) FS(fileSystem fs.FS, paths ...string) *Configuration {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	c.fsLocations = append(c.fsLocations, fsLocation{fileSystem: fileSystem, paths: paths})
	return c
}

// Schemas sets the schemas managed by the migrator. The first schema is the
// default schema in which the schema history table is created.
func (c *Configuration) Schemas(schemas ...string) *Configuration {
	c.schemas = append([]string(nil), schemas...)
	return c
}

// DefaultSchema overrides the schema that holds the schema history table.
func (c *Configuration) DefaultSchema(schema string) *Configuration {
	c.defaultSchema = schema
	return c
}

// CreateSchemas controls whether missing schemas are created automatically.
func (c *Configuration) CreateSchemas(create bool) *Configuration {
	c.createSchemas = create
	return c
}

// Table sets the name of the schema history table.
func (c *Configuration) Table(table string) *Configuration {
	c.table = table
	return c
}

// BaselineVersion sets the version recorded by the baseline command and below
// which migrations are ignored.
func (c *Configuration) BaselineVersion(version string) *Configuration {
	parsed, err := parseVersion(version)
	if err != nil {
		c.configErr = err
		return c
	}
	c.baselineVersion = parsed
	return c
}

// BaselineDescription sets the description recorded by the baseline command.
func (c *Configuration) BaselineDescription(description string) *Configuration {
	c.baselineDescription = description
	return c
}

// BaselineOnMigrate controls whether a non empty schema without a history table
// is baselined automatically on the first migrate.
func (c *Configuration) BaselineOnMigrate(enabled bool) *Configuration {
	c.baselineOnMigrate = enabled
	return c
}

// Placeholders sets the placeholder values substituted into scripts.
func (c *Configuration) Placeholders(placeholders map[string]string) *Configuration {
	c.placeholders = make(map[string]string, len(placeholders))
	for key, value := range placeholders {
		c.placeholders[strings.ToLower(key)] = value
	}
	return c
}

// PlaceholderPrefix sets the opening delimiter of a placeholder.
func (c *Configuration) PlaceholderPrefix(prefix string) *Configuration {
	c.placeholderPrefix = prefix
	return c
}

// PlaceholderSuffix sets the closing delimiter of a placeholder.
func (c *Configuration) PlaceholderSuffix(suffix string) *Configuration {
	c.placeholderSuffix = suffix
	return c
}

// SQLMigrationPrefix sets the prefix that marks versioned migration scripts.
func (c *Configuration) SQLMigrationPrefix(prefix string) *Configuration {
	c.sqlMigrationPrefix = prefix
	return c
}

// RepeatableSQLMigrationPrefix sets the prefix that marks repeatable scripts.
func (c *Configuration) RepeatableSQLMigrationPrefix(prefix string) *Configuration {
	c.repeatableSQLMigrationPrefix = prefix
	return c
}

// SQLMigrationSeparator sets the token between the version and the description.
func (c *Configuration) SQLMigrationSeparator(separator string) *Configuration {
	c.sqlMigrationSeparator = separator
	return c
}

// SQLMigrationSuffixes sets the recognized script file suffixes.
func (c *Configuration) SQLMigrationSuffixes(suffixes ...string) *Configuration {
	c.sqlMigrationSuffixes = append([]string(nil), suffixes...)
	return c
}

// ValidateOnMigrate controls whether migrate validates before applying.
func (c *Configuration) ValidateOnMigrate(enabled bool) *Configuration {
	c.validateOnMigrate = enabled
	return c
}

// CleanDisabled controls whether the clean command is permitted.
func (c *Configuration) CleanDisabled(disabled bool) *Configuration {
	c.cleanDisabled = disabled
	return c
}

// OutOfOrder controls whether migrations with a version lower than the current
// one may still be applied.
func (c *Configuration) OutOfOrder(enabled bool) *Configuration {
	c.outOfOrder = enabled
	return c
}

// Target sets the highest version that migrate will apply.
func (c *Configuration) Target(version string) *Configuration {
	parsed, err := parseVersion(version)
	if err != nil {
		c.configErr = err
		return c
	}
	c.target = parsed
	return c
}

// InstalledBy overrides the user recorded for applied migrations.
func (c *Configuration) InstalledBy(user string) *Configuration {
	c.installedBy = user
	return c
}

// sources builds the migration sources from the configured locations and file
// systems.
func (c *Configuration) sources() []migrationSource {
	sources := make([]migrationSource, 0, len(c.locations)+len(c.fsLocations))
	for _, location := range c.locations {
		sources = append(sources, parseLocation(location))
	}
	for _, entry := range c.fsLocations {
		for _, path := range entry.paths {
			sources = append(sources, &fsSource{fileSystem: entry.fileSystem, root: path})
		}
	}
	return sources
}

// Load validates the configuration, determines the dialect when one was not set
// explicitly, resolves the migrations from disk, and returns a ready Migrator
// instance.
func (c *Configuration) Load() (*Migrator, error) {
	return c.LoadContext(context.Background())
}

// LoadContext behaves like Load while honoring the supplied context during
// dialect detection.
func (c *Configuration) LoadContext(ctx context.Context) (*Migrator, error) {
	if c.configErr != nil {
		return nil, c.configErr
	}
	if c.db == nil {
		return nil, ErrNoDataSource
	}

	dialect := c.dialect
	if dialect == nil {
		detected, err := detectDialect(ctx, c.db)
		if err != nil {
			return nil, err
		}
		dialect = detected
	}

	resolved, err := resolveMigrations(c)
	if err != nil {
		return nil, err
	}

	return &Migrator{
		configuration: c,
		dialect:       dialect,
		resolved:      resolved,
	}, nil
}
