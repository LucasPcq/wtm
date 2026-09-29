package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EffectiveIsolation reads an unset value as isolated: every worktree was
// isolated before the choice existed, and main has no metadata to hold one.
func EffectiveIsolation(value domain.Isolation) domain.Isolation {
	if value == "" {
		return domain.IsolationIsolated
	}
	return value
}

func IsVerbatim(value domain.Isolation) bool {
	return EffectiveIsolation(value) == domain.IsolationVerbatim
}

// ParseIsolation validates a value typed on the command line; empty stays
// empty, which is "not given".
func ParseIsolation(value string) (domain.Isolation, error) {
	switch isolation := domain.Isolation(value); isolation {
	case "", domain.IsolationIsolated, domain.IsolationVerbatim:
		return isolation, nil
	default:
		return "", fmt.Errorf(domain.IsolationUnknownFmt, value, domain.IsolationIsolated, domain.IsolationVerbatim)
	}
}

func ValidateIsolation(cfg domain.RunConfig) []string {
	if _, err := ParseIsolation(string(cfg.Isolation)); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// IsolationApplies says whether run.toml declares anything a worktree could
// isolate — a port, a slice, a .env link, a compose identity. Without one the
// two answers do exactly the same thing, so the question is not worth asking.
func IsolationApplies(cfg domain.RunConfig) bool {
	if len(cfg.EnvPorts) > 0 || len(cfg.EnvValues) > 0 || len(ComposeProjectDirs(cfg)) > 0 {
		return true
	}
	for _, job := range cfg.Jobs {
		if len(job.Ports) > 0 || HasNamespace(job) {
			return true
		}
	}
	return false
}

// IsolationChoices lists both answers with the given one first, which is where
// every surface starts the cursor.
func IsolationChoices(first domain.Isolation) []domain.Isolation {
	if IsVerbatim(first) {
		return []domain.Isolation{domain.IsolationVerbatim, domain.IsolationIsolated}
	}
	return []domain.Isolation{domain.IsolationIsolated, domain.IsolationVerbatim}
}

// IsolationOptionLabel is how a picker offers the answer.
func IsolationOptionLabel(isolation domain.Isolation) string {
	if IsVerbatim(isolation) {
		return domain.IsolationOptionVerbatim
	}
	return domain.IsolationOptionIsolated
}

// IsolationSummary is how a recap reads the answer back.
func IsolationSummary(isolation domain.Isolation) string {
	if IsVerbatim(isolation) {
		return domain.IsolationSummaryVerbatim
	}
	return domain.IsolationSummaryIsolated
}

// FirstIsolation is the first answer given, in the order the caller trusts
// them: a flag before a default.
func FirstIsolation(values ...domain.Isolation) domain.Isolation {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
