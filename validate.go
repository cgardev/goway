package goway

import (
	"context"
	"fmt"
	"strings"
)

// Validate compares the applied migrations against the resolved scripts and
// reports any discrepancy, such as a checksum mismatch, a locally missing
// migration, a failed migration, or a resolved migration that has not been
// applied. When validation fails, the returned error wraps ErrValidationFailed
// and the result still carries the full list of problems.
func (f *Migrator) Validate(ctx context.Context) (*ValidateResult, error) {
	schema, err := f.resolveDefaultSchema(ctx)
	if err != nil {
		return nil, err
	}
	applied, err := f.loadApplied(ctx, f.history(schema))
	if err != nil {
		return nil, err
	}

	service := computeInfos(f.resolved, applied, f.configuration)
	problems := service.validate(false)
	result := &ValidateResult{
		Valid:           len(problems) == 0,
		Errors:          problems,
		ValidationCount: len(service.entries),
	}
	if !result.Valid {
		return result, validationError(problems)
	}
	return result, nil
}

// validationError builds a single error that summarizes every validation
// problem, while wrapping ErrValidationFailed so callers can match on it.
func validationError(problems []ValidationError) error {
	parts := make([]string, 0, len(problems))
	for _, problem := range problems {
		identifier := problem.Script
		if problem.Version != "" {
			identifier = "version " + problem.Version
		}
		parts = append(parts, fmt.Sprintf("%s: %s", identifier, problem.Message))
	}
	return fmt.Errorf("%w: %s", ErrValidationFailed, strings.Join(parts, "; "))
}
