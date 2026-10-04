package rules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DistinctNamesParams struct {
	Names []string
	Blank string
}

// DistinctNames refuses a malformed list of positional names as a whole (exit
// 2), before anything is said about what they name.
func DistinctNames(params DistinctNamesParams) ([]string, error) {
	names := make([]string, 0, len(params.Names))
	seen := make(map[string]bool, len(params.Names))
	for _, raw := range params.Names {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, fmt.Errorf("%w: %s", domain.ErrUsage, params.Blank)
		}
		if seen[name] {
			return nil, fmt.Errorf("%w: "+domain.BranchGivenTwiceFmt, domain.ErrUsage, name)
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

type BatchFailureOfParams struct {
	Branch string
	Path   string
	Err    error
}

func BatchFailureOf(params BatchFailureOfParams) domain.BatchFailure {
	return domain.BatchFailure{
		Branch:     params.Branch,
		Path:       params.Path,
		Error:      params.Err.Error(),
		ExitCode:   ExitCode(params.Err),
		Privileged: errors.Is(params.Err, domain.ErrWorktreeRemoveFailed),
	}
}
