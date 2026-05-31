package goway

import "testing"

func i32(value int32) *int32 { return &value }

// findInfo returns the first migration info whose script and installed rank
// match, or fails the test.
func findInfo(t *testing.T, infos []MigrationInfo, script string, rank int) MigrationInfo {
	t.Helper()
	for _, info := range infos {
		if info.Script == script && info.InstalledRank == rank {
			return info
		}
	}
	t.Fatalf("no migration info for script %q rank %d in %+v", script, rank, infos)
	return MigrationInfo{}
}

func TestComputeInfosRepeatableSupersededWhenChecksumMatches(t *testing.T) {
	configuration := Configure()
	resolved := []*resolvedMigration{{
		description:   "view",
		script:        "R__view.sql",
		checksum:      100,
		repeatable:    true,
		migrationType: MigrationTypeSQL,
	}}
	applied := []appliedMigration{
		{installedRank: 1, description: "view", script: "R__view.sql", migrationType: "SQL", checksum: i32(50), success: true},
		{installedRank: 2, description: "view", script: "R__view.sql", migrationType: "SQL", checksum: i32(100), success: true},
	}

	service := computeInfos(resolved, applied, configuration)
	infos := service.infos()

	if got := findInfo(t, infos, "R__view.sql", 1).State; got != StateSuperseded {
		t.Errorf("older run state = %q, want %q", got, StateSuperseded)
	}
	if got := findInfo(t, infos, "R__view.sql", 2).State; got != StateSuccess {
		t.Errorf("latest run state = %q, want %q", got, StateSuccess)
	}

	// A superseded run must never be a validation error, and the matching latest
	// run keeps validation clean.
	if problems := service.validate(false); len(problems) != 0 {
		t.Errorf("validate reported problems for a superseded repeatable: %+v", problems)
	}

	// Nothing is pending: the latest run matches the resolved checksum.
	if pending := service.pending(); len(pending) != 0 {
		t.Errorf("expected no pending migrations, got %d", len(pending))
	}
}

func TestComputeInfosRepeatableLatestOutdated(t *testing.T) {
	configuration := Configure()
	resolved := []*resolvedMigration{{
		description:   "view",
		script:        "R__view.sql",
		checksum:      777,
		repeatable:    true,
		migrationType: MigrationTypeSQL,
	}}
	applied := []appliedMigration{
		{installedRank: 1, description: "view", script: "R__view.sql", migrationType: "SQL", checksum: i32(50), success: true},
		{installedRank: 2, description: "view", script: "R__view.sql", migrationType: "SQL", checksum: i32(100), success: true},
	}

	service := computeInfos(resolved, applied, configuration)
	infos := service.infos()

	if got := findInfo(t, infos, "R__view.sql", 1).State; got != StateSuperseded {
		t.Errorf("older run state = %q, want %q", got, StateSuperseded)
	}
	if got := findInfo(t, infos, "R__view.sql", 2).State; got != StateOutdated {
		t.Errorf("latest run state = %q, want %q", got, StateOutdated)
	}

	// The outdated latest run is pending re-execution.
	pending := service.pending()
	if len(pending) != 1 {
		t.Fatalf("expected exactly one pending migration, got %d", len(pending))
	}
}
