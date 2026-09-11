package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposePatchLines describes the rewrites a patch would perform, one line
// each. Both surfaces read it: the wizard step asking for authorization and the
// recap reporting what was done — so what the user agreed to and what they are
// told happened are the same sentences.
func ComposePatchLines(byFile map[string][]domain.ComposePortBinding) []string {
	var lines []string
	for _, file := range SortedComposeFiles(byFile) {
		for _, b := range byFile[file] {
			lines = append(lines, fmt.Sprintf(domain.ComposePatchLineFmt, file, b.Service, b.Token, b.Replacement))
		}
	}
	return lines
}

// ComposeWithheldLine prefers the reason a mapping carries: a frozen port
// withheld over a variable conflict is not withheld for being frozen, and
// saying so would send the reader at a fix that recreates the conflict.
func ComposeWithheldLine(b domain.ComposePortBinding) string {
	if b.Reason != "" {
		return fmt.Sprintf(domain.ComposeUnsupportedFmt, b.File, b.Service, b.Reason)
	}
	return fmt.Sprintf(domain.ComposeFrozenLineFmt, b.File, b.Service, b.Token)
}

type ComposeFixLinesParams struct {
	Binding domain.ComposePortBinding
	// Job is the run.toml job running the binding's file, when there is one. It
	// turns the second line from a placeholder into a command to paste.
	Job string
}

// ComposeFixLines is the geste the user performs to isolate a port wtm did not
// declare: the mapping to write, then the declaration that makes it effective.
// A mapping wtm cannot rewrite at all has no fix to offer.
func ComposeFixLines(params ComposeFixLinesParams) []string {
	var lines []string
	switch {
	// A frozen mapping carrying a reason was withheld over a conflict, not over
	// being frozen: templating it would not help.
	case params.Binding.Status == domain.ComposePortFrozen && params.Binding.Reason != "":
		return nil
	case params.Binding.Status == domain.ComposePortFrozen && params.Binding.Replacement != "":
		lines = []string{fmt.Sprintf(domain.ComposeFixLineFmt, params.Binding.Replacement)}
	case ComposeNeedsDefault(params.Binding):
		lines = []string{fmt.Sprintf(domain.ComposeFixDefaultFmt, ComposeSuggestedDefault(params.Binding))}
	default:
		return nil
	}

	if params.Job == "" {
		return append(lines, fmt.Sprintf(domain.ComposeFixNoJobFmt, params.Binding.Var, params.Binding.Base))
	}
	return append(lines, fmt.Sprintf(domain.ComposeFixCmdFmt, params.Job, params.Binding.Var, params.Binding.Base))
}

func ComposeDroppedLine(d DroppedPort) string {
	return fmt.Sprintf(domain.ComposeDroppedLineFmt,
		d.Port.Name, d.Port.Job, d.Port.Base,
		d.Against.Name, d.Against.Job, d.Against.Base,
		d.Worktrees)
}

func ComposeUnreadableLine(scan domain.ComposeScan) string {
	return fmt.Sprintf(domain.ComposeUnreadableFmt, scan.File, scan.Err)
}

type ComposeJobNameParams struct {
	Config domain.RunConfig
	File   string
}

// ComposeJobName matches on the same "-f <file> " fragment BuildDockerJobs
// emits. Empty when no job runs the file.
//
// A lifted job carries that fragment too, and is emitted before the job it was
// taken out of — so the first match is not the file's own job. Skipping the
// shared ones is what makes this the file's job whatever the order: the rewrite
// that trims the stack to what stayed landed on the lifted job itself
// otherwise, leaving it starting every service but its own.
func ComposeJobName(params ComposeJobNameParams) string {
	needle := DockerComposeFileFlag(params.File)
	for _, job := range params.Config.Jobs {
		if !IsShared(job) && jobRunsComposeFile(job, needle) {
			return job.Name
		}
	}
	return ""
}

// ComposeJobsRunningFile is the other question: not which job is the file's
// own, but whether the file is run at all. A file whose every service was
// lifted has no job of its own and is still entirely covered.
func ComposeJobsRunningFile(cfg domain.RunConfig, file string) []domain.JobConfig {
	needle := DockerComposeFileFlag(file)
	var jobs []domain.JobConfig
	for _, job := range cfg.Jobs {
		if jobRunsComposeFile(job, needle) {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

type ComposeFilesNeedingAJobParams struct {
	Config domain.RunConfig
	Files  []string
}

// ComposeFilesNeedingAJob narrows a selection to the files no job runs yet.
// Merging by job name alone re-adds a compose file whose job was renamed, which
// then declares the same ports twice and loses both to the collision check.
func ComposeFilesNeedingAJob(params ComposeFilesNeedingAJobParams) []string {
	if len(params.Config.Jobs) == 0 {
		return params.Files
	}
	needing := make([]string, 0, len(params.Files))
	for _, file := range params.Files {
		if len(ComposeJobsRunningFile(params.Config, file)) == 0 {
			needing = append(needing, file)
		}
	}
	return needing
}

// ComposeNeedsDefault singles out the one withheld mapping wtm can still help
// with: an explicit template whose variable carries no default, so the file
// never says which port it stands for. Every other refusal resolves without a
// variable name, which is what tells the two apart.
func ComposeNeedsDefault(b domain.ComposePortBinding) bool {
	return b.Status == domain.ComposePortUnsupported && b.Var != "" && b.Token != ""
}

// ComposeSuggestedDefault rewrites the mapping with the container port as the
// default. It is a suggestion for the reader to confirm, never something wtm
// writes — the whole reason the mapping is withheld is that wtm cannot know.
func ComposeSuggestedDefault(b domain.ComposePortBinding) string {
	withDefault := fmt.Sprintf(domain.ComposeTemplatedPortFmt, b.Var, b.Base)
	if braced := "${" + b.Var + "}"; strings.Contains(b.Token, braced) {
		return strings.Replace(b.Token, braced, withDefault, 1)
	}
	return strings.Replace(b.Token, "$"+b.Var, withDefault, 1)
}
