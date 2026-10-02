package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func ExecCommandLine(args []string) (string, error) {
	line := strings.TrimSpace(strings.Join(args, " "))
	if line == "" {
		return "", domain.ErrExecNoCommand
	}
	return line, nil
}

type ExecLogPathParams struct {
	StateDir string
	Branch   string
}

func ExecLogPath(params ExecLogPathParams) string {
	if params.StateDir == "" {
		return ""
	}
	return filepath.Join(params.StateDir, domain.ExecLogDirName, EncodeBranchSegment(params.Branch)+domain.ExecLogFileExt)
}

type ResolveExecTargetsParams struct {
	Candidates []domain.GitWorktree
	Names      []string
}

func ResolveExecTargets(params ResolveExecTargetsParams) ([]domain.GitWorktree, error) {
	byBranch := make(map[string]domain.GitWorktree, len(params.Candidates))
	for _, candidate := range params.Candidates {
		byBranch[candidate.Branch] = candidate
	}
	targets := make([]domain.GitWorktree, 0, len(params.Names))
	var unknown []string
	for _, name := range params.Names {
		candidate, ok := byBranch[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		targets = append(targets, candidate)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrExecUnknownWorktree, strings.Join(unknown, ", "))
	}
	return targets, nil
}

type ExecCurrentParams struct {
	Candidates []domain.GitWorktree
	Dir        string
}

// ExecCurrent takes the deepest match: linked worktrees may live inside the
// main checkout, and the main checkout would otherwise win for all of them.
func ExecCurrent(params ExecCurrentParams) string {
	best, bestLen := "", -1
	for _, candidate := range params.Candidates {
		inside := params.Dir == candidate.Path || strings.HasPrefix(params.Dir, candidate.Path+string(filepath.Separator))
		if !inside || len(candidate.Path) <= bestLen {
			continue
		}
		best, bestLen = candidate.Branch, len(candidate.Path)
	}
	return best
}

func CountExec(results []domain.ExecResult) domain.ExecCounts {
	var counts domain.ExecCounts
	for _, result := range results {
		switch result.Status {
		case domain.ExecStatusPassed:
			counts.Passed++
		case domain.ExecStatusFailed:
			counts.Failed++
		case domain.ExecStatusInterrupted:
			counts.Interrupted++
		case domain.ExecStatusNotStarted:
			counts.NotStarted++
		}
	}
	return counts
}

func ExecFailedBranches(results []domain.ExecResult) []string {
	failed := []string{}
	for _, result := range results {
		if result.Status != domain.ExecStatusPassed {
			failed = append(failed, result.Branch)
		}
	}
	return failed
}

func ExecResultLabel(result domain.ExecResult) string {
	duration := HookDuration(time.Duration(result.DurationMs) * time.Millisecond)
	switch {
	case result.Status == domain.ExecStatusInterrupted:
		return fmt.Sprintf(domain.ExecStateLabelFmt, result.Branch, domain.ExecInterruptedLabel)
	case result.Status == domain.ExecStatusNotStarted:
		return fmt.Sprintf(domain.ExecStateLabelFmt, result.Branch, domain.ExecNotStartedLabel)
	case result.Error != "":
		return fmt.Sprintf(domain.ExecStateLabelFmt, result.Branch, result.Error)
	case result.Status == domain.ExecStatusFailed && result.ExitCode != nil:
		return fmt.Sprintf(domain.ExecFailedLabelFmt, result.Branch, fmt.Sprintf(domain.ExecExitFmt, *result.ExitCode), duration)
	}
	return fmt.Sprintf(domain.ExecPassedLabelFmt, result.Branch, duration)
}
