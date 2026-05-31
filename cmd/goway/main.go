// Command goway is a command line front end for the goway migration library.
// It connects to a PostgreSQL or SQLite database and runs one of the migration
// commands: migrate, info, validate, baseline, repair or clean.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cgardev/goway"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// options holds the values parsed from the command line flags.
type options struct {
	url                 string
	locations           string
	schemas             string
	table               string
	createSchemas       bool
	baselineVersion     string
	baselineDescription string
	baselineOnMigrate   bool
	outOfOrder          bool
	cleanDisabled       bool
	placeholders        string
	target              string
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("goway", flag.ContinueOnError)
	var opts options
	flags.StringVar(&opts.url, "url", envOrDefault("GOWAY_URL", ""), "database URL, for example postgres://user:pass@host:5432/db or sqlite:./app.db")
	flags.StringVar(&opts.locations, "locations", "filesystem:db/migration", "comma separated list of migration locations")
	flags.StringVar(&opts.schemas, "schemas", "", "comma separated list of schemas, the first being the default schema")
	flags.StringVar(&opts.table, "table", "flyway_schema_history", "name of the schema history table")
	flags.BoolVar(&opts.createSchemas, "create-schemas", true, "create missing schemas automatically")
	flags.StringVar(&opts.baselineVersion, "baseline-version", "1", "version to record when baselining")
	flags.StringVar(&opts.baselineDescription, "baseline-description", "<< Flyway Baseline >>", "description to record when baselining")
	flags.BoolVar(&opts.baselineOnMigrate, "baseline-on-migrate", false, "baseline automatically on the first migrate")
	flags.BoolVar(&opts.outOfOrder, "out-of-order", false, "allow migrations to be applied out of order")
	flags.BoolVar(&opts.cleanDisabled, "clean-disabled", true, "disable the clean command")
	flags.StringVar(&opts.placeholders, "placeholders", "", "comma separated key=value placeholder pairs")
	flags.StringVar(&opts.target, "target", "", "highest version to apply")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: goway [flags] <command>")
		fmt.Fprintln(os.Stderr, "Commands: migrate, info, validate, baseline, repair, clean")
		fmt.Fprintln(os.Stderr, "Flags:")
		flags.PrintDefaults()
	}

	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return fmt.Errorf("exactly one command is required")
	}
	command := flags.Arg(0)

	if opts.url == "" {
		return fmt.Errorf("a database URL is required, set -url or GOWAY_URL")
	}

	database, dialect, err := openDatabase(opts.url)
	if err != nil {
		return err
	}
	defer database.Close()

	migrator, err := buildMigrator(database, dialect, opts)
	if err != nil {
		return err
	}

	ctx := context.Background()
	switch command {
	case "migrate":
		return runMigrate(ctx, migrator)
	case "info":
		return runInfo(ctx, migrator)
	case "validate":
		return runValidate(ctx, migrator)
	case "baseline":
		return runBaseline(ctx, migrator)
	case "repair":
		return runRepair(ctx, migrator)
	case "clean":
		return runClean(ctx, migrator)
	default:
		flags.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

// openDatabase opens a connection pool and selects the matching dialect based on
// the URL scheme.
func openDatabase(url string) (*sql.DB, goway.Dialect, error) {
	switch {
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		database, err := sql.Open("pgx", url)
		return database, goway.Postgres(), err
	case strings.HasPrefix(url, "sqlite:"):
		path := strings.TrimPrefix(url, "sqlite:")
		database, err := sql.Open("sqlite", path)
		return database, goway.SQLite(), err
	case strings.HasPrefix(url, "file:"):
		database, err := sql.Open("sqlite", url)
		return database, goway.SQLite(), err
	default:
		return nil, nil, fmt.Errorf("unsupported database URL %q; use a postgres:// or sqlite: URL", url)
	}
}

// buildMigrator translates the parsed options into a configured migrator.
func buildMigrator(database *sql.DB, dialect goway.Dialect, opts options) (*goway.Migrator, error) {
	configuration := goway.Configure().
		DataSource(database).
		Dialect(dialect).
		Locations(splitList(opts.locations)...).
		Table(opts.table).
		CreateSchemas(opts.createSchemas).
		BaselineVersion(opts.baselineVersion).
		BaselineDescription(opts.baselineDescription).
		BaselineOnMigrate(opts.baselineOnMigrate).
		OutOfOrder(opts.outOfOrder).
		CleanDisabled(opts.cleanDisabled)

	if schemas := splitList(opts.schemas); len(schemas) > 0 {
		configuration.Schemas(schemas...)
	}
	if opts.target != "" {
		configuration.Target(opts.target)
	}
	if placeholders := parsePlaceholders(opts.placeholders); len(placeholders) > 0 {
		configuration.Placeholders(placeholders)
	}

	return configuration.Load()
}

func runMigrate(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Migrate(ctx)
	if err != nil {
		return err
	}
	if result.MigrationsExecuted == 0 {
		fmt.Printf("Schema is up to date. No migration necessary. (version %s)\n", orNone(result.TargetSchemaVersion))
		return nil
	}
	fmt.Printf("Successfully applied %d migration(s) to version %s:\n", result.MigrationsExecuted, orNone(result.TargetSchemaVersion))
	for _, migration := range result.Migrations {
		fmt.Printf("  %-10s %s\n", orNone(migration.Version), migration.Description)
	}
	return nil
}

func runInfo(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Info(ctx)
	if err != nil {
		return err
	}
	writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(writer, "Version\tDescription\tType\tState\tInstalled On")
	for _, migration := range result.Migrations {
		installedOn := ""
		if migration.InstalledOn != nil && !migration.InstalledOn.IsZero() {
			installedOn = migration.InstalledOn.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			orNone(migration.Version), migration.Description, migration.Type, migration.State, installedOn)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	fmt.Printf("\nCurrent version: %s\n", orNone(result.Current))
	return nil
}

func runValidate(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Validate(ctx)
	if result != nil && result.Valid {
		fmt.Printf("Validation successful: %d migration(s) validated.\n", result.ValidationCount)
		return nil
	}
	if result != nil {
		fmt.Fprintln(os.Stderr, "Validation failed:")
		for _, problem := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", orNone(problem.Version), problem.Message)
		}
	}
	return err
}

func runBaseline(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Baseline(ctx)
	if err != nil {
		return err
	}
	if result.Created {
		fmt.Printf("Successfully baselined schema at version %s.\n", result.BaselineVersion)
	} else {
		fmt.Printf("Schema is already baselined at version %s.\n", result.BaselineVersion)
	}
	return nil
}

func runRepair(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Repair(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Repair complete: removed %d failed entry(ies), realigned %d checksum(s).\n",
		result.RemovedFailed, result.AlignedChecksums)
	return nil
}

func runClean(ctx context.Context, migrator *goway.Migrator) error {
	result, err := migrator.Clean(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Successfully cleaned schema(s): %s.\n", strings.Join(result.SchemasCleaned, ", "))
	return nil
}

// splitList splits a comma separated value and trims each element, dropping
// empty entries.
func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parsePlaceholders parses comma separated key=value pairs into a map.
func parsePlaceholders(value string) map[string]string {
	placeholders := map[string]string{}
	for _, pair := range splitList(value) {
		key, val, found := strings.Cut(pair, "=")
		if found {
			placeholders[strings.TrimSpace(key)] = strings.TrimSpace(val)
		}
	}
	return placeholders
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func orNone(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}
