package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsBlankCommand reports whether a configured command holds nothing to run.
// The check has to be explicit: `sh -c "   "` succeeds and exits 0, so a blank
// stop command would otherwise mark a service stopped without stopping it.
func IsBlankCommand(line string) bool {
	return strings.TrimSpace(line) == ""
}

// ShellCommand is how every command written in a config file is executed: as a
// shell line, not as a whitespace-split argv. Quotes, `&&`, redirections, globs
// and `${VAR}` therefore mean what they mean in a terminal — which is what the
// file format looks like it promises, and what lets a job write `--port ${PORT}`
// against the port wtm resolved for its worktree.
func ShellCommand(line string) domain.ExecSpec {
	return domain.ExecSpec{
		Name: domain.ShellBin,
		Args: []string{domain.ShellCommandFlag, line},
	}
}
