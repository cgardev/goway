package goway

import (
	"context"
	"testing"
)

func TestParseCallbackName(t *testing.T) {
	suffixes := []string{".sql"}
	recognized := map[string]CallbackEvent{
		"beforeMigrate.sql":          EventBeforeMigrate,
		"afterMigrate.sql":           EventAfterMigrate,
		"beforeEachMigrate.sql":      EventBeforeEachMigrate,
		"afterEachMigrate__seed.sql": EventAfterEachMigrate,
		"AFTERMIGRATE.sql":           EventAfterMigrate,
	}
	for name, want := range recognized {
		got, ok := parseCallbackName(name, "__", suffixes)
		if !ok || got != want {
			t.Errorf("parseCallbackName(%q) = (%q, %v), want (%q, true)", name, got, ok, want)
		}
	}

	rejected := []string{
		"V1__create.sql",      // versioned migration
		"R__view.sql",         // repeatable migration
		"random.sql",          // unknown stem
		"beforeMigrate.txt",   // wrong suffix
		"afterMigrateNow.sql", // stem is not exactly an event name
	}
	for _, name := range rejected {
		if _, ok := parseCallbackName(name, "__", suffixes); ok {
			t.Errorf("parseCallbackName(%q) was recognized, want rejected", name)
		}
	}
}

// TestCallbackFuncImplementsCallback verifies the function adapter satisfies the
// Callback interface and forwards its arguments.
func TestCallbackFuncImplementsCallback(t *testing.T) {
	var seen CallbackEvent
	var callback Callback = CallbackFunc(func(_ context.Context, event CallbackEvent, _ Execer, _ *MigrationInfo) error {
		seen = event
		return nil
	})
	if err := callback.Handle(context.Background(), EventBeforeMigrate, nil, nil); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if seen != EventBeforeMigrate {
		t.Errorf("callback saw event %q, want %q", seen, EventBeforeMigrate)
	}
}
