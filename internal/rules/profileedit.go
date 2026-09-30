package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ProfilePatch is one non-interactive edit of a profile. A nil pointer is a
// field the caller left alone. Jobs replaces the list rather than merging into
// it: their order is the start order, which only a full list can express.
type ProfilePatch struct {
	Name    *string
	Jobs    []string
	Default *bool
}

// Empty reports a patch that would change nothing, which is how a runner tells
// "edit these fields" from "open the wizard".
func (p ProfilePatch) Empty() bool {
	return p.Name == nil && p.Jobs == nil && p.Default == nil
}

type ApplyProfilePatchParams struct {
	Current domain.ProfileConfig
	Patch   ProfilePatch
}

// ApplyProfilePatch returns Current with the patch applied. Whether the named
// jobs exist is ValidateRun's answer, not this one's.
func ApplyProfilePatch(params ApplyProfilePatchParams) (domain.ProfileConfig, error) {
	profile := params.Current
	patch := params.Patch

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return domain.ProfileConfig{}, fmt.Errorf("--%s cannot be empty", domain.FlagName)
		}
		profile.Name = name
	}
	if patch.Jobs != nil {
		jobs := trimmedNonEmpty(patch.Jobs)
		if len(jobs) == 0 {
			return domain.ProfileConfig{}, fmt.Errorf("--%s cannot be empty — a profile starts at least one job", domain.FlagJobs)
		}
		profile.Jobs = jobs
	}
	if patch.Default != nil {
		profile.Default = *patch.Default
	}

	return profile, nil
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
