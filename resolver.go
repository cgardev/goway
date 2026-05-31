package goway

import (
	"fmt"
	"sort"
)

// resolveMigrations scans every configured source, parses each candidate file
// name, computes its checksum, and returns the resolved migrations sorted in
// the order they would be applied: versioned migrations by ascending version,
// followed by repeatable migrations by ascending description. Duplicate
// versions or repeatable descriptions are rejected.
func resolveMigrations(configuration *Configuration) ([]*resolvedMigration, error) {
	var scanned []scannedFile
	for _, source := range configuration.sources() {
		files, err := source.scan()
		if err != nil {
			return nil, err
		}
		scanned = append(scanned, files...)
	}

	var versioned []*resolvedMigration
	var repeatable []*resolvedMigration
	seenVersions := make(map[string]string)
	seenRepeatable := make(map[string]string)

	for _, file := range scanned {
		parsed := parseResourceName(
			file.name,
			configuration.sqlMigrationPrefix,
			configuration.repeatableSQLMigrationPrefix,
			configuration.sqlMigrationSeparator,
			configuration.sqlMigrationSuffixes,
		)
		if !parsed.valid {
			continue
		}

		content, err := file.read()
		if err != nil {
			return nil, fmt.Errorf("goway: reading migration %s: %w", file.location, err)
		}
		checksum := calculateChecksum(content)
		reader := file.read

		migration := &resolvedMigration{
			description:   parsed.description,
			script:        file.name,
			checksum:      checksum,
			repeatable:    parsed.repeatable,
			migrationType: MigrationTypeSQL,
			read:          reader,
		}

		if parsed.repeatable {
			if previous, exists := seenRepeatable[parsed.description]; exists {
				return nil, fmt.Errorf("%w: %q and %q both describe %q",
					ErrDuplicateRepeatable, previous, file.name, parsed.description)
			}
			seenRepeatable[parsed.description] = file.name
			repeatable = append(repeatable, migration)
			continue
		}

		version, err := parseVersion(parsed.rawVersion)
		if err != nil {
			return nil, fmt.Errorf("goway: migration %s: %w", file.name, err)
		}
		migration.version = version

		key := normalizedVersionKey(version)
		if previous, exists := seenVersions[key]; exists {
			return nil, fmt.Errorf("%w: %q and %q both target version %s",
				ErrDuplicateVersion, previous, file.name, version)
		}
		seenVersions[key] = file.name
		versioned = append(versioned, migration)
	}

	sort.SliceStable(versioned, func(i, j int) bool {
		return versioned[i].version.Compare(versioned[j].version) < 0
	})
	sort.SliceStable(repeatable, func(i, j int) bool {
		return repeatable[i].description < repeatable[j].description
	})

	resolved := make([]*resolvedMigration, 0, len(versioned)+len(repeatable))
	resolved = append(resolved, versioned...)
	resolved = append(resolved, repeatable...)
	return resolved, nil
}

// normalizedVersionKey produces a canonical key for a version that ignores
// trailing zero components, so that "1" and "1.0" are recognized as duplicates.
func normalizedVersionKey(version *Version) string {
	highest := -1
	for index, part := range version.parts {
		if part.Sign() != 0 {
			highest = index
		}
	}
	key := ""
	for index := 0; index <= highest; index++ {
		if index > 0 {
			key += "."
		}
		key += version.parts[index].String()
	}
	if key == "" {
		key = "0"
	}
	return key
}
