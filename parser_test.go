package goway

import (
	"strings"
	"testing"
)

func mustSplit(t *testing.T, dialect, script string) []string {
	t.Helper()
	statements, err := splitSQLStatements(script, dialect)
	if err != nil {
		t.Fatalf("splitSQLStatements error: %v", err)
	}
	return statements
}

func TestSplitSimpleStatements(t *testing.T) {
	got := mustSplit(t, dialectPostgres, "SELECT 1; SELECT 2;")
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitIgnoresSemicolonInString(t *testing.T) {
	got := mustSplit(t, dialectPostgres, "INSERT INTO t VALUES ('a;b'); SELECT 1;")
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitIgnoresSemicolonInLineComment(t *testing.T) {
	got := mustSplit(t, dialectPostgres, "SELECT 1; -- a comment; with semicolon\nSELECT 2;")
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitNestedBlockComment(t *testing.T) {
	got := mustSplit(t, dialectPostgres, "/* outer /* inner */ still */ SELECT 1;")
	if len(got) != 1 {
		t.Fatalf("got %d statements (%q), want 1", len(got), got)
	}
}

func TestSplitDollarQuotedFunction(t *testing.T) {
	script := "CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql; SELECT 1;"
	got := mustSplit(t, dialectPostgres, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitDollarQuotedWithTag(t *testing.T) {
	script := "SELECT $tag$ a; b; c $tag$; SELECT 2;"
	got := mustSplit(t, dialectPostgres, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitEscapeStringProtectsSemicolon(t *testing.T) {
	// In an escape string, a backslash escapes the following quote, so the inner
	// semicolon must not terminate the statement.
	script := "SELECT E'a\\';b'; SELECT 2;"
	got := mustSplit(t, dialectPostgres, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
	if !strings.Contains(got[0], "E'a\\';b'") {
		t.Errorf("first statement = %q, expected to contain the escape string", got[0])
	}
}

func TestSplitDollarSignThatIsNotAQuote(t *testing.T) {
	// A bare dollar followed by a digit is not a dollar quote tag.
	got := mustSplit(t, dialectPostgres, "SELECT $1; SELECT $2;")
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitSQLiteTriggerBody(t *testing.T) {
	script := "CREATE TRIGGER t AFTER INSERT ON x BEGIN UPDATE y SET a = 1; DELETE FROM z; END; SELECT 1;"
	got := mustSplit(t, dialectSQLite, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitSQLiteTriggerWithCase(t *testing.T) {
	script := "CREATE TRIGGER t AFTER INSERT ON x BEGIN " +
		"UPDATE y SET a = CASE WHEN 1 THEN 2 ELSE 3 END; END; SELECT 1;"
	got := mustSplit(t, dialectSQLite, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitSQLiteBacktickIdentifierWithKeyword(t *testing.T) {
	// A backtick quoted identifier that happens to be a block keyword must not
	// affect statement boundaries.
	script := "CREATE TABLE `BEGIN` (id INTEGER); SELECT 1;"
	got := mustSplit(t, dialectSQLite, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}

func TestSplitSQLiteBracketIdentifierWithKeyword(t *testing.T) {
	script := "CREATE TABLE [BEGIN] (id INTEGER); SELECT 1;"
	got := mustSplit(t, dialectSQLite, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements (%q), want 2", len(got), got)
	}
}
