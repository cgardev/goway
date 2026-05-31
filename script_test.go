package goway

import "testing"

func TestScriptRequestsNoTransaction(t *testing.T) {
	requests := []string{
		"-- goway:noTransaction\nCREATE INDEX CONCURRENTLY idx ON t (c);",
		"--goway:noTransaction\nVACUUM;",
		"-- GOWAY:NOTRANSACTION\nSELECT 1;",
		"-- flyway:executeInTransaction=false\nSELECT 1;",
		"-- a normal comment\n-- goway:noTransaction\nSELECT 1;",
	}
	for _, script := range requests {
		if !scriptRequestsNoTransaction(script) {
			t.Errorf("expected no-transaction directive to be detected in:\n%s", script)
		}
	}

	plain := []string{
		"CREATE TABLE t (id INTEGER);",
		"-- just a comment\nSELECT 1;",
		"SELECT 'goway:noTransaction';", // appears in a statement, not a comment
	}
	for _, script := range plain {
		if scriptRequestsNoTransaction(script) {
			t.Errorf("did not expect a directive to be detected in:\n%s", script)
		}
	}
}
