package goway

import "testing"

var defaultSuffixes = []string{".sql"}

func parseName(fileName string) resourceName {
	return parseResourceName(fileName, "V", "R", "__", defaultSuffixes)
}

func TestParseResourceNameVersioned(t *testing.T) {
	parsed := parseName("V1__create_table.sql")
	if !parsed.valid {
		t.Fatalf("expected valid name, got reason %q", parsed.reason)
	}
	if parsed.repeatable {
		t.Error("versioned migration reported as repeatable")
	}
	if parsed.rawVersion != "1" {
		t.Errorf("rawVersion = %q, want 1", parsed.rawVersion)
	}
	if parsed.description != "create table" {
		t.Errorf("description = %q, want %q", parsed.description, "create table")
	}
}

func TestParseResourceNameRepeatable(t *testing.T) {
	parsed := parseName("R__active_users_view.sql")
	if !parsed.valid {
		t.Fatalf("expected valid name, got reason %q", parsed.reason)
	}
	if !parsed.repeatable {
		t.Error("repeatable migration not reported as repeatable")
	}
	if parsed.rawVersion != "" {
		t.Errorf("rawVersion = %q, want empty", parsed.rawVersion)
	}
	if parsed.description != "active users view" {
		t.Errorf("description = %q, want %q", parsed.description, "active users view")
	}
}

func TestParseResourceNameDottedVersion(t *testing.T) {
	parsed := parseName("V2.1__add_index.sql")
	if !parsed.valid || parsed.rawVersion != "2.1" {
		t.Fatalf("rawVersion = %q valid = %v, want 2.1 valid", parsed.rawVersion, parsed.valid)
	}
}

func TestParseResourceNameInvalid(t *testing.T) {
	for _, name := range []string{
		"readme.txt",          // wrong suffix
		"V__missing.sql",      // versioned without a version
		"R1__has_version.sql", // repeatable with a version
		"V1_single.sql",       // separator is __ not _
		"X1__wrong.sql",       // unknown prefix
	} {
		if parsed := parseName(name); parsed.valid {
			t.Errorf("parseName(%q) reported valid, want invalid", name)
		}
	}
}

func TestParseResourceNameCaseInsensitiveSuffix(t *testing.T) {
	if parsed := parseName("V1__upper.SQL"); !parsed.valid {
		t.Errorf("upper case suffix .SQL was not accepted: %q", parsed.reason)
	}
}

func TestMatchSuffixReturnsFirstConfigured(t *testing.T) {
	// When several configured suffixes match, the first in configuration order
	// wins, matching Flyway's ResourceNameParser.stripSuffix behavior.
	if got := matchSuffix("V1__ax.sql", []string{".sql", "x.sql"}); got != ".sql" {
		t.Errorf("matchSuffix returned %q, want .sql (first match)", got)
	}
	if got := matchSuffix("V1__ax.sql", []string{"x.sql", ".sql"}); got != "x.sql" {
		t.Errorf("matchSuffix returned %q, want x.sql (first match)", got)
	}
}
