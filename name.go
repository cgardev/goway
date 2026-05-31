package goway

import "strings"

// resourceName holds the components extracted from a migration script file name
// according to the configured naming convention.
type resourceName struct {
	// prefix is the leading marker that classifies the script, for example "V"
	// for versioned migrations or "R" for repeatable migrations.
	prefix string

	// rawVersion is the textual version that appears before the separator. It is
	// empty for repeatable migrations.
	rawVersion string

	// description is the human readable description with underscores converted to
	// spaces, matching the value stored in the schema history table.
	description string

	// suffix is the matched file suffix, for example ".sql".
	suffix string

	// repeatable reports whether the script is a repeatable migration.
	repeatable bool

	// valid reports whether the file name satisfies the naming convention.
	valid bool

	// reason describes why an invalid name was rejected, for diagnostics.
	reason string
}

// parseResourceName splits a migration script file name into its components.
// The suffix is stripped first, then the prefix, and finally the remainder is
// split at the first occurrence of the separator into version and description.
func parseResourceName(fileName, versionedPrefix, repeatablePrefix, separator string, suffixes []string) resourceName {
	suffix := matchSuffix(fileName, suffixes)
	if suffix == "" {
		return resourceName{valid: false, reason: "no recognized suffix"}
	}
	stem := fileName[:len(fileName)-len(suffix)]

	var prefix string
	var repeatable bool
	switch {
	case versionedPrefix != "" && strings.HasPrefix(stem, versionedPrefix):
		prefix = versionedPrefix
	case repeatablePrefix != "" && strings.HasPrefix(stem, repeatablePrefix):
		prefix = repeatablePrefix
		repeatable = true
	default:
		return resourceName{valid: false, reason: "no recognized prefix"}
	}

	rest := stem[len(prefix):]
	separatorIndex := strings.Index(rest, separator)
	if separatorIndex < 0 {
		return resourceName{valid: false, reason: "missing separator " + separator}
	}

	rawVersion := rest[:separatorIndex]
	rawDescription := rest[separatorIndex+len(separator):]

	if repeatable {
		if rawVersion != "" {
			return resourceName{valid: false, reason: "repeatable migration must not declare a version"}
		}
	} else if rawVersion == "" {
		return resourceName{valid: false, reason: "versioned migration must declare a version"}
	}

	return resourceName{
		prefix:      prefix,
		rawVersion:  rawVersion,
		description: strings.ReplaceAll(rawDescription, "_", " "),
		suffix:      suffix,
		repeatable:  repeatable,
		valid:       true,
	}
}

// matchSuffix returns the first configured suffix that matches the end of the
// file name, comparing case insensitively, matching the order based resolution
// of the reference implementation. An empty string is returned when no suffix
// matches.
func matchSuffix(fileName string, suffixes []string) string {
	lowerName := strings.ToLower(fileName)
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(lowerName, strings.ToLower(suffix)) {
			return suffix
		}
	}
	return ""
}
