package goway

import (
	"fmt"
	"math/big"
	"strings"
)

// Version represents a parsed, comparable migration version such as "1", "1.1"
// or "2.0.3". A version is a sequence of non negative integer parts that are
// compared element by element. A missing trailing element is treated as zero,
// so "1.0" and "1" are considered equal.
type Version struct {
	// parts holds the numeric components in declaration order. Arbitrary
	// precision integers are used so that very large numeric components, such as
	// date based versions, are compared correctly.
	parts []*big.Int

	// display retains the textual form used for presentation and for storage in
	// the schema history table, with underscores normalized to dots.
	display string
}

// parseVersion parses a raw version token into a Version. Underscores are
// normalized to dots before splitting, matching the Flyway convention that
// allows file systems that dislike dots in names to use underscores instead.
func parseVersion(raw string) (*Version, error) {
	if raw == "" {
		return nil, fmt.Errorf("%w: empty version", ErrInvalidMigrationName)
	}

	normalized := strings.ReplaceAll(raw, "_", ".")
	tokens := strings.Split(normalized, ".")
	parts := make([]*big.Int, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			return nil, fmt.Errorf("%w: version %q has an empty component", ErrInvalidMigrationName, raw)
		}
		value, ok := new(big.Int).SetString(token, 10)
		if !ok || value.Sign() < 0 {
			return nil, fmt.Errorf("%w: version %q component %q is not a non negative integer", ErrInvalidMigrationName, raw, token)
		}
		parts = append(parts, value)
	}

	// Drop trailing zero components so the numeric form is canonical and so
	// that equivalent versions such as 1 and 1.0 share the same parts.
	for len(parts) > 1 && parts[len(parts)-1].Sign() == 0 {
		parts = parts[:len(parts)-1]
	}

	return &Version{parts: parts, display: normalized}, nil
}

// Compare returns a negative number, zero, or a positive number when the
// receiver is respectively lower than, equal to, or greater than the other
// version. Components beyond the length of either version are treated as zero.
func (v *Version) Compare(other *Version) int {
	length := len(v.parts)
	if len(other.parts) > length {
		length = len(other.parts)
	}

	for index := 0; index < length; index++ {
		left := zeroBigInt
		if index < len(v.parts) {
			left = v.parts[index]
		}
		right := zeroBigInt
		if index < len(other.parts) {
			right = other.parts[index]
		}
		if result := left.Cmp(right); result != 0 {
			return result
		}
	}
	return 0
}

// Equal reports whether two versions are numerically equal. Two nil receivers
// are considered equal, and a nil receiver never equals a non nil one.
func (v *Version) Equal(other *Version) bool {
	if v == nil || other == nil {
		return v == other
	}
	return v.Compare(other) == 0
}

// String returns the textual representation used for display and storage.
func (v *Version) String() string {
	if v == nil {
		return ""
	}
	return v.display
}

// zeroBigInt is a shared immutable zero used while comparing versions of
// differing length. It must never be mutated.
var zeroBigInt = big.NewInt(0)
