package goway

import (
	"context"
	"database/sql"
	"strings"
)

// CallbackEvent identifies a point in the migrate lifecycle at which callbacks
// are invoked. The values match the corresponding Flyway event names.
type CallbackEvent string

const (
	// EventBeforeMigrate fires once before any migration is applied.
	EventBeforeMigrate CallbackEvent = "beforeMigrate"

	// EventAfterMigrate fires once after all migrations have been applied.
	EventAfterMigrate CallbackEvent = "afterMigrate"

	// EventBeforeEachMigrate fires before each individual migration, inside the
	// migration's transaction when one is used.
	EventBeforeEachMigrate CallbackEvent = "beforeEachMigrate"

	// EventAfterEachMigrate fires after each individual migration, inside the
	// migration's transaction when one is used.
	EventAfterEachMigrate CallbackEvent = "afterEachMigrate"
)

// Execer is the minimal interface needed to run a statement. It is satisfied by
// *sql.DB, *sql.Tx and *sql.Conn, so a callback can execute SQL on whichever
// handle is active for the current event.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Callback receives lifecycle events during a migrate run. For the per-migration
// events the migration argument describes the migration being processed; it is
// nil for the run level events.
type Callback interface {
	Handle(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error
}

// CallbackFunc adapts an ordinary function to the Callback interface.
type CallbackFunc func(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error

// Handle calls the underlying function.
func (f CallbackFunc) Handle(ctx context.Context, event CallbackEvent, exec Execer, migration *MigrationInfo) error {
	return f(ctx, event, exec, migration)
}

// callbackEventsByName maps the lower cased event name to its canonical value.
var callbackEventsByName = map[string]CallbackEvent{
	"beforemigrate":     EventBeforeMigrate,
	"aftermigrate":      EventAfterMigrate,
	"beforeeachmigrate": EventBeforeEachMigrate,
	"aftereachmigrate":  EventAfterEachMigrate,
}

// sqlCallback is a callback backed by a SQL script discovered in the configured
// locations.
type sqlCallback struct {
	event  CallbackEvent
	script string
	read   func() ([]byte, error)
}

// parseCallbackName reports whether a script file name denotes a callback and,
// if so, which event it handles. The name without its suffix must equal an event
// name, optionally followed by the separator and a description, for example
// "afterEachMigrate__seed.sql". Matching is case insensitive.
func parseCallbackName(fileName, separator string, suffixes []string) (CallbackEvent, bool) {
	suffix := matchSuffix(fileName, suffixes)
	if suffix == "" {
		return "", false
	}
	stem := fileName[:len(fileName)-len(suffix)]
	name := stem
	if index := strings.Index(stem, separator); index >= 0 {
		name = stem[:index]
	}
	event, ok := callbackEventsByName[strings.ToLower(name)]
	return event, ok
}
