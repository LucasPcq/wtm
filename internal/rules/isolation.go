package rules

import (
	"fmt"
	"strings"

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

func IsolationOptionLabelMany(isolation domain.Isolation) string {
	if IsVerbatim(isolation) {
		return domain.IsolationOptionVerbatimMany
	}
	return domain.IsolationOptionIsolatedMany
}

// IsolationSummary is how a recap reads the answer back.
func IsolationSummary(isolation domain.Isolation) string {
	if IsVerbatim(isolation) {
		return domain.IsolationSummaryVerbatim
	}
	return domain.IsolationSummaryIsolated
}

type IsolationAdoptionPendingParams struct {
	IsMain   bool
	Recorded domain.Isolation
	Config   domain.RunConfig
}

// IsolationAdoptionPending says whether a worktree still runs on its source's
// run values because it was created before the choice existed. The main
// checkout has nothing to adopt, and neither has a project declaring nothing to
// isolate.
func IsolationAdoptionPending(params IsolationAdoptionPendingParams) bool {
	return !params.IsMain && params.Recorded == "" && IsolationApplies(params.Config)
}

// DefaultComposeProjectName is the name `docker compose` gives a project whose
// environment names none: its directory's, lowercased, keeping only the
// characters a project name may hold.
func DefaultComposeProjectName(dir string) string {
	var name strings.Builder
	for _, r := range strings.ToLower(dir) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			name.WriteRune(r)
		}
	}
	return strings.TrimLeft(name.String(), "_-")
}

// IsolationAdoptOptionLabel is how the migration offers adopting isolation:
// what the worktree gains, and the data it leaves behind.
func IsolationAdoptOptionLabel(plan domain.IsolationAdoptionPlan) string {
	if plan.ComposeProject == "" {
		return domain.IsolationAdoptPortsLabel
	}
	return fmt.Sprintf(domain.IsolationAdoptComposeFmt, plan.ComposeProject, plan.CurrentComposeProject)
}

func IsolationNotAdoptedWarning(branch string) string {
	return fmt.Sprintf(domain.EnvIsolationNotAdoptedFmt, branch, branch)
}

type IsolationNotSwitchedParams struct {
	Branch    string
	Isolation domain.Isolation
	Cause     string
}

func IsolationNotSwitchedWarning(params IsolationNotSwitchedParams) string {
	return fmt.Sprintf(domain.EnvIsolationNotSwitchedFmt, params.Branch, params.Isolation, params.Cause, params.Branch, params.Isolation)
}

type IsolationIgnoredParams struct {
	Branch    string
	Requested domain.Isolation
	Current   domain.Isolation
}

// IsolationIgnoredWarning is --isolation met by a worktree that already
// exists: the flag only ever answers a creation. Empty when it was not given,
// or asked for what the worktree already is.
func IsolationIgnoredWarning(params IsolationIgnoredParams) string {
	if params.Requested == "" || params.Requested == params.Current {
		return ""
	}
	return fmt.Sprintf(domain.IsolationIgnoredFmt, domain.FlagIsolation, params.Requested, params.Branch, params.Current, params.Branch, domain.FlagIsolation, params.Requested)
}

type CreationFlagsIgnoredParams struct {
	Branch string
	// Given are the creation flags the run was passed, by name.
	Given []string
}

// CreationFlagsIgnoredWarnings names each flag that only shapes a worktree being
// created, on a run whose target was already there: said, rather than dropped.
func CreationFlagsIgnoredWarnings(params CreationFlagsIgnoredParams) []string {
	warnings := make([]string, 0, len(params.Given))
	for _, flag := range params.Given {
		warnings = append(warnings, fmt.Sprintf(domain.CreationFlagIgnoredFmt, flag, params.Branch))
	}
	return warnings
}

type IsolationRecapShownParams struct {
	Applies  bool
	Override domain.Isolation
}

// IsolationRecapShown is when a create-like recap names the isolation: the
// step was posed, or a flag answered it — a flag never loses its line.
func IsolationRecapShown(params IsolationRecapShownParams) bool {
	return params.Applies || params.Override != ""
}
