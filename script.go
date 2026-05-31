package goway

import "strings"

// noTransactionMarkers are the comment directives that request a migration to
// run outside a transaction. The goway form is preferred; the flyway form is
// accepted for familiarity.
var noTransactionMarkers = []string{
	"goway:notransaction",
	"flyway:executeintransaction=false",
}

// scriptRequestsNoTransaction reports whether a SQL script opts out of the
// per-migration transaction through a comment directive. Only comment lines are
// inspected, so a marker that happens to appear inside a statement or string
// literal does not trigger the behavior. Detection is case insensitive and runs
// on the raw script before placeholder replacement.
func scriptRequestsNoTransaction(script string) bool {
	for _, raw := range strings.Split(script, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "--") {
			continue
		}
		lower := strings.ToLower(line)
		for _, marker := range noTransactionMarkers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}
