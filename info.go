package goway

import (
	"fmt"
	"sort"
)

// migrationInfoEntry links a resolved migration with its applied history record
// and the derived state of the pair.
type migrationInfoEntry struct {
	resolved            *resolvedMigration
	applied             *appliedMigration
	version             *Version
	description         string
	script              string
	state               MigrationState
	outOfOrder          bool
	checksumMismatch    bool
	descriptionMismatch bool
}

// migrationInfoService holds the computed state of every migration, both
// versioned and repeatable, and exposes the queries used by the commands.
type migrationInfoService struct {
	entries []*migrationInfoEntry
	current *Version
}

// computeInfos correlates the resolved migrations with the applied history and
// derives the state of each migration. The configuration supplies the target
// version and the out of order policy.
func computeInfos(resolved []*resolvedMigration, applied []appliedMigration, configuration *Configuration) *migrationInfoService {
	resolvedVersioned := make(map[string]*resolvedMigration)
	resolvedRepeatable := make([]*resolvedMigration, 0)
	var maxResolved *Version
	for _, migration := range resolved {
		if migration.repeatable {
			resolvedRepeatable = append(resolvedRepeatable, migration)
			continue
		}
		resolvedVersioned[normalizedVersionKey(migration.version)] = migration
		if maxResolved == nil || migration.version.Compare(maxResolved) > 0 {
			maxResolved = migration.version
		}
	}

	appliedVersioned := make(map[string]*appliedMigration)
	repeatableApplied := make(map[string]*appliedMigration)
	var baseline *appliedMigration
	var current *Version
	noteCurrent := func(version *Version) {
		if version == nil {
			return
		}
		if current == nil || version.Compare(current) > 0 {
			current = version
		}
	}
	for index := range applied {
		record := &applied[index]
		switch {
		case record.migrationType == string(MigrationTypeSchema):
			continue
		case record.migrationType == string(MigrationTypeBaseline):
			baseline = record
			if record.success {
				noteCurrent(record.version)
			}
		case record.version != nil:
			appliedVersioned[normalizedVersionKey(record.version)] = record
			if record.success {
				noteCurrent(record.version)
			}
		default:
			repeatableApplied[record.description] = record
		}
	}

	var baselineVersion *Version
	if baseline != nil {
		baselineVersion = baseline.version
	}

	entries := make([]*migrationInfoEntry, 0, len(resolved)+len(applied))

	// Versioned entries are the union of resolved and applied versions.
	versionKeys := make([]string, 0)
	versionSet := make(map[string]*Version)
	for key, migration := range resolvedVersioned {
		if _, seen := versionSet[key]; !seen {
			versionKeys = append(versionKeys, key)
		}
		versionSet[key] = migration.version
	}
	for key, record := range appliedVersioned {
		if _, seen := versionSet[key]; !seen {
			versionKeys = append(versionKeys, key)
			versionSet[key] = record.version
		}
	}

	versionedEntries := make([]*migrationInfoEntry, 0, len(versionKeys))
	for _, key := range versionKeys {
		resolvedMigration := resolvedVersioned[key]
		appliedMigration := appliedVersioned[key]
		entry := &migrationInfoEntry{
			resolved: resolvedMigration,
			applied:  appliedMigration,
			version:  versionSet[key],
		}
		populateVersionedEntry(entry, configuration, current, baselineVersion, maxResolved)
		versionedEntries = append(versionedEntries, entry)
	}

	if baseline != nil {
		versionedEntries = append(versionedEntries, &migrationInfoEntry{
			applied:     baseline,
			version:     baseline.version,
			description: baseline.description,
			script:      baseline.script,
			state:       StateBaseline,
		})
	}

	sort.SliceStable(versionedEntries, func(i, j int) bool {
		return compareEntryVersions(versionedEntries[i].version, versionedEntries[j].version) < 0
	})
	entries = append(entries, versionedEntries...)

	// Repeatable entries are the union of resolved and applied descriptions.
	repeatableKeys := make([]string, 0)
	repeatableSet := make(map[string]bool)
	resolvedRepeatableByDescription := make(map[string]*resolvedMigration)
	for _, migration := range resolvedRepeatable {
		resolvedRepeatableByDescription[migration.description] = migration
		if !repeatableSet[migration.description] {
			repeatableKeys = append(repeatableKeys, migration.description)
			repeatableSet[migration.description] = true
		}
	}
	for description := range repeatableApplied {
		if !repeatableSet[description] {
			repeatableKeys = append(repeatableKeys, description)
			repeatableSet[description] = true
		}
	}
	sort.Strings(repeatableKeys)

	for _, description := range repeatableKeys {
		resolvedMigration := resolvedRepeatableByDescription[description]
		appliedMigration := repeatableApplied[description]
		entry := &migrationInfoEntry{
			resolved:    resolvedMigration,
			applied:     appliedMigration,
			description: description,
		}
		populateRepeatableEntry(entry)
		entries = append(entries, entry)
	}

	return &migrationInfoService{entries: entries, current: current}
}

// populateVersionedEntry derives the state of a versioned migration.
func populateVersionedEntry(entry *migrationInfoEntry, configuration *Configuration, current, baselineVersion, maxResolved *Version) {
	resolved := entry.resolved
	applied := entry.applied

	switch {
	case applied != nil && resolved != nil:
		entry.description = resolved.description
		entry.script = resolved.script
		if !applied.success {
			entry.state = StateFailed
			break
		}
		entry.state = StateSuccess
		if applied.checksum == nil || *applied.checksum != resolved.checksum {
			entry.checksumMismatch = true
		}
		if applied.description != resolved.description {
			entry.descriptionMismatch = true
		}

	case applied != nil && resolved == nil:
		entry.description = applied.description
		entry.script = applied.script
		future := maxResolved != nil && applied.version.Compare(maxResolved) > 0
		switch {
		case future && applied.success:
			entry.state = StateFuture
		case future && !applied.success:
			entry.state = StateFutureFailed
		case applied.success:
			entry.state = StateMissing
		default:
			entry.state = StateMissingFailed
		}

	case applied == nil && resolved != nil:
		entry.description = resolved.description
		entry.script = resolved.script
		switch {
		case configuration.target != nil && resolved.version.Compare(configuration.target) > 0:
			entry.state = StateAboveTarget
		case baselineVersion != nil && resolved.version.Compare(baselineVersion) <= 0:
			entry.state = StateIgnored
		case current != nil && resolved.version.Compare(current) < 0:
			entry.outOfOrder = true
			if configuration.outOfOrder {
				entry.state = StatePending
			} else {
				entry.state = StateIgnored
			}
		default:
			entry.state = StatePending
		}
	}
}

// populateRepeatableEntry derives the state of a repeatable migration.
func populateRepeatableEntry(entry *migrationInfoEntry) {
	resolved := entry.resolved
	applied := entry.applied
	switch {
	case applied != nil && resolved != nil:
		entry.script = resolved.script
		if applied.checksum == nil || *applied.checksum != resolved.checksum {
			entry.state = StateOutdated
		} else {
			entry.state = StateSuccess
		}
	case applied != nil && resolved == nil:
		entry.script = applied.script
		entry.state = StateMissing
	case applied == nil && resolved != nil:
		entry.script = resolved.script
		entry.state = StatePending
	}
}

// compareEntryVersions orders two possibly nil versions, placing a nil version
// last.
func compareEntryVersions(left, right *Version) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		return left.Compare(right)
	}
}

// pending returns the resolved migrations that should be applied, in execution
// order: versioned migrations by ascending version, then repeatable migrations
// by ascending description.
func (s *migrationInfoService) pending() []*migrationInfoEntry {
	var pending []*migrationInfoEntry
	for _, entry := range s.entries {
		if entry.resolved == nil {
			continue
		}
		if entry.resolved.repeatable {
			continue
		}
		if entry.state == StatePending {
			pending = append(pending, entry)
		}
	}
	for _, entry := range s.entries {
		if entry.resolved == nil || !entry.resolved.repeatable {
			continue
		}
		if entry.state == StatePending || entry.state == StateOutdated {
			pending = append(pending, entry)
		}
	}
	return pending
}

// infos returns the public, read only view of every migration.
func (s *migrationInfoService) infos() []MigrationInfo {
	result := make([]MigrationInfo, 0, len(s.entries))
	for _, entry := range s.entries {
		info := MigrationInfo{
			Version:     entry.version.String(),
			Description: entry.description,
			Script:      entry.script,
			State:       entry.state,
		}
		if entry.resolved != nil {
			info.Type = string(entry.resolved.migrationType)
			checksum := entry.resolved.checksum
			info.Checksum = &checksum
		}
		if entry.applied != nil {
			if info.Type == "" {
				info.Type = entry.applied.migrationType
			}
			info.InstalledRank = entry.applied.installedRank
			info.InstalledBy = entry.applied.installedBy
			info.ExecutionTime = entry.applied.executionTime
			installedOn := entry.applied.installedOn
			info.InstalledOn = &installedOn
		}
		result = append(result, info)
	}
	return result
}

// validate inspects every entry and returns the problems found. Pending and out
// of order migrations are only reported when ignorePending is false, which lets
// the migrate command validate without failing on the migrations it is about to
// apply.
func (s *migrationInfoService) validate(ignorePending bool) []ValidationError {
	var problems []ValidationError
	add := func(entry *migrationInfoEntry, message string) {
		problems = append(problems, ValidationError{
			Version:     entry.version.String(),
			Description: entry.description,
			Script:      entry.script,
			Message:     message,
		})
	}

	for _, entry := range s.entries {
		switch entry.state {
		case StateFailed, StateFutureFailed, StateMissingFailed:
			add(entry, "migration is marked as failed and must be resolved or repaired")
		case StateMissing:
			add(entry, "applied migration is no longer resolved locally")
		case StateFuture:
			add(entry, "applied migration has a version higher than any resolved migration")
		case StateSuccess:
			if entry.checksumMismatch {
				add(entry, fmt.Sprintf("checksum mismatch: applied %s differs from resolved %s",
					appliedChecksumText(entry), resolvedChecksumText(entry)))
			}
			if entry.descriptionMismatch {
				add(entry, "description mismatch between applied migration and resolved script")
			}
		case StatePending, StateOutdated:
			if !ignorePending {
				add(entry, "resolved migration has not been applied to the database")
			}
		case StateIgnored:
			if entry.outOfOrder && !ignorePending {
				add(entry, "out of order migration has not been applied; enable out of order execution to apply it")
			}
		}
	}
	return problems
}

func appliedChecksumText(entry *migrationInfoEntry) string {
	if entry.applied == nil || entry.applied.checksum == nil {
		return "<none>"
	}
	return fmt.Sprintf("%d", *entry.applied.checksum)
}

func resolvedChecksumText(entry *migrationInfoEntry) string {
	if entry.resolved == nil {
		return "<none>"
	}
	return fmt.Sprintf("%d", entry.resolved.checksum)
}
