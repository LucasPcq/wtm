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
	seen := make(map[string]bool, len(params.Names))
	var unknown []string
	for _, name := range params.Names {
		if seen[name] {
			continue
		}
		seen[name] = true
		candidate, ok := byBranch[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		targets = append(targets, candidate)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %w: %s", domain.ErrBranchNotFound, domain.ErrExecUnknownWorktree, strings.Join(unknown, ", "))
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

type ExecJobsParams struct {
	Requested int
	CPUs      int
}

func ExecJobs(params ExecJobsParams) int {
	if params.Requested == 0 {
		return params.CPUs
	}
	return params.Requested
}

// TerminalLine is a line as a terminal would have left it: the frames a
// progress bar rewrote with \r collapse to the last one, and escape sequences
// (colours, cursor moves, hyperlinks) are dropped.
func TerminalLine(line string) string {
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	var out strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] != domain.AnsiEscByte {
			out.WriteByte(line[i])
			continue
		}
		i = escapeEnd(line, i)
	}
	return out.String()
}

// escapeEnd returns the index of the last byte of the escape sequence starting
// at start: CSI ends on a final byte in 0x40–0x7E, OSC on BEL or ESC \.
func escapeEnd(line string, start int) int {
	if start+1 >= len(line) {
		return start
	}
	switch line[start+1] {
	case '[':
		for i := start + 2; i < len(line); i++ {
			if line[i] >= 0x40 && line[i] <= 0x7e {
				return i
			}
		}
	case ']':
		for i := start + 2; i < len(line); i++ {
			if line[i] == domain.AnsiBelByte {
				return i
			}
			if line[i] == domain.AnsiEscByte && i+1 < len(line) && line[i+1] == '\\' {
				return i + 1
			}
		}
	default:
		return start + 1
	}
	return len(line) - 1
}
