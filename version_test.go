package goway

import "testing"

func mustVersion(t *testing.T, raw string) *Version {
	t.Helper()
	version, err := parseVersion(raw)
	if err != nil {
		t.Fatalf("parseVersion(%q) returned error: %v", raw, err)
	}
	return version
}

func TestParseVersionRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "1.x", "-1", "1..2", "v1", "1.", ".1"} {
		if _, err := parseVersion(raw); err == nil {
			t.Errorf("parseVersion(%q) succeeded, want error", raw)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1", "1", 0},
		{"1.0", "1", 0},
		{"1.0.0", "1", 0},
		{"2", "1", 1},
		{"1.1", "1.2", -1},
		{"2.0.1", "2.1", -1},
		{"10", "9", 1},
		{"1.10", "1.9", 1},
	}
	for _, c := range cases {
		got := mustVersion(t, c.left).Compare(mustVersion(t, c.right))
		if sign(got) != c.want {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", c.left, c.right, got, c.want)
		}
	}
}

func TestVersionEqualWithTrailingZeros(t *testing.T) {
	if !mustVersion(t, "1.0").Equal(mustVersion(t, "1")) {
		t.Error("1.0 should equal 1")
	}
	if mustVersion(t, "1.1").Equal(mustVersion(t, "1")) {
		t.Error("1.1 should not equal 1")
	}
}

func TestVersionStringPreservesDisplay(t *testing.T) {
	if got := mustVersion(t, "2.1").String(); got != "2.1" {
		t.Errorf("String() = %q, want 2.1", got)
	}
	// Underscores are normalized to dots for display and storage.
	if got := mustVersion(t, "1_1").String(); got != "1.1" {
		t.Errorf("String() = %q, want 1.1", got)
	}
}

func TestParseVersionStripsTrailingZeroParts(t *testing.T) {
	// The canonical numeric form drops trailing zero components so that
	// equivalent versions share the same key, matching the reference behavior.
	if got := len(mustVersion(t, "1.2.0").parts); got != 2 {
		t.Errorf("parts length of 1.2.0 = %d, want 2", got)
	}
	if got := len(mustVersion(t, "1.0.0").parts); got != 1 {
		t.Errorf("parts length of 1.0.0 = %d, want 1", got)
	}
	if got := len(mustVersion(t, "0").parts); got != 1 {
		t.Errorf("parts length of 0 = %d, want 1", got)
	}
}

func TestNormalizedVersionKey(t *testing.T) {
	cases := map[string]string{
		"1":     "1",
		"1.0":   "1",
		"1.2.0": "1.2",
		"0":     "0",
		"2.10":  "2.10",
	}
	for raw, want := range cases {
		if got := normalizedVersionKey(mustVersion(t, raw)); got != want {
			t.Errorf("normalizedVersionKey(%q) = %q, want %q", raw, got, want)
		}
	}
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
