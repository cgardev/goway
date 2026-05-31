package goway

import (
	"fmt"
	"strings"
)

// replacePlaceholders substitutes every occurrence of a placeholder in the
// script with its configured value. A placeholder is delimited by the prefix
// and suffix, for example "${" and "}". When replacement is enabled and a
// placeholder has no configured value, an error is returned, matching the strict
// behavior of the reference implementation. Replacement is disabled, and the
// script is returned unchanged, when there are no placeholders configured.
//
// Placeholder keys are matched case insensitively; callers are expected to store
// keys in lower case, which the Configuration setters do.
func replacePlaceholders(script string, placeholders map[string]string, prefix, suffix string) (string, error) {
	if len(placeholders) == 0 || prefix == "" || suffix == "" {
		return script, nil
	}

	var builder strings.Builder
	builder.Grow(len(script))
	index := 0
	for index < len(script) {
		if strings.HasPrefix(script[index:], prefix) {
			remainder := script[index+len(prefix):]
			end := strings.Index(remainder, suffix)
			if end >= 0 {
				key := remainder[:end]
				if value, ok := placeholders[strings.ToLower(key)]; ok {
					builder.WriteString(value)
					index += len(prefix) + end + len(suffix)
					continue
				}
				return "", fmt.Errorf("goway: no value provided for placeholder %s%s%s", prefix, key, suffix)
			}
		}
		builder.WriteByte(script[index])
		index++
	}
	return builder.String(), nil
}
