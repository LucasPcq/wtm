package exec

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what exec decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the meta record of each worktree on a branch, the worktrees whose directory is gone, where the caller stands, the CPUs a run uses at most by default, and what the shell says of the command (nil when it parses, or when none is given).
type Facts struct {
	worktree.Repository
	Records map[string]worktree.Meta // by branch
	Gone    map[string]bool          // by path
	Cwd     string                   // where the caller stands, through its symbolic links
	CPUs    int
	Syntax  error
}

// standing are the worktree the caller stands in, as the form picks it first; none when the caller stands in none.
func (f Facts) standing() []string {
	worktrees := f.standingWorktrees()
	if at := worktree.Holding(worktrees, f.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return []string{worktrees[at].Name()}
	}
	return nil
}

// options are the worktrees exec may run in: each one whose directory stands, the main one first, by its branch or, on no branch, its path; the caller's tagged current, the main one main.
func (f Facts) options() []domain.Option {
	return domain.WorktreeOptions(f.standingWorktrees(), domain.WorktreeQuery{Cwd: f.Cwd})
}

func (f Facts) standingWorktrees() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return f.Gone[w.Path] })
}

// named returns the worktree of name, its branch or, on no branch, its path as git spells it, and whether there is one.
func (f Facts) named(name string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		return worktree.Worktree{}, false
	}

	return f.Worktrees[at], true
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Target is a worktree the command runs in: its name, its branch (none on no branch) and its root, as git spells it.
type Target struct {
	Worktree string `json:"worktree"`
	Branch   string `json:"branch,omitempty"`
	Path     string `json:"path"`
}

// entries decides each worktree names asks: a Target, or the problem of its entry: blank, given twice, gone, or naming no worktree.
func (f Facts) entries(names []string) (targets []Target, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		switch w, found := f.named(name); {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found && f.Gone[w.Path]:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeWorktreeGone, Template: "{value}'s directory is gone: nothing to run in", Params: domain.Params{"value": name, "path": w.Path}})
		case found:
			targets = append(targets, target(w))
		default:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		}
	}

	return targets, problems
}

func target(w worktree.Worktree) Target {
	return Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path}
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings: the worktrees named, or every one that stands; the concurrency, never more than the targets; the worktrees --all leaves out because their directory is gone, and each target that runs without its run environment for want of an ordinal.
func Decide(f Facts, req Request) (Plan, []domain.Warning) {
	targets, _ := f.entries(req.Worktrees)
	var warnings []domain.Warning
	if req.All {
		targets = nil
		for _, w := range f.standingWorktrees() {
			targets = append(targets, target(w))
		}
		warnings = append(warnings, f.leftOut()...)
	}

	jobs, _ := jobsOf(req.Jobs)
	if jobs == 0 {
		jobs = max(f.CPUs, 1)
	}
	planned := Plan{Targets: targets, Command: req.Command, Jobs: min(jobs, max(len(targets), 1)), Print: req.Print, facts: f, asked: req}
	if planned.Targets == nil {
		planned.Targets = []Target{}
	}

	for _, t := range planned.Targets {
		if record := f.Records[t.Branch]; f.readsRun(t) && record.Ordinal == 0 {
			warnings = append(warnings, domain.Warning{Code: CodeEnvUnnumbered, Message: t.Worktree + " has no ordinal: the command runs without its ports and Compose project", Params: domain.Params{"worktree": t.Worktree}})
		}
	}

	return planned, warnings
}

// The codes of the warnings of a plan: worktrees --all leaves out, their directory gone, with the command that drops them, and a target that runs without its run environment, its ordinal missing.
const (
	CodeWorktreesGone = Name + ".worktrees_gone"
	CodePrune         = Name + ".prune"
	CodeEnvUnnumbered = Name + ".env_unnumbered"
)

// commandPrune drops the worktrees whose directory is gone: not migrated yet.
const commandPrune = "prune"

// leftOut warns of the worktrees whose directory is gone, which --all leaves out.
func (f Facts) leftOut() []domain.Warning {
	var gone []string
	for _, w := range f.MainFirst() {
		if f.Gone[w.Path] {
			gone = append(gone, w.Name())
		}
	}
	if len(gone) == 0 {
		return nil
	}

	names := strings.Join(gone, ", ")
	return []domain.Warning{{
		Code:     CodeWorktreesGone,
		Message:  "left out, their directory gone: " + names,
		Params:   domain.Params{"worktrees": names},
		FollowUp: &domain.FollowUp{Command: commandPrune, Code: CodePrune, Message: "drop them"},
	}}
}

// readsRun reports whether the command runs with the target's run environment, as its hooks do: under an isolation, when some job runs docker compose.
func (f Facts) readsRun(t Target) bool {
	return t.Branch != "" && env.HooksReadRun(f.Run, f.Records[t.Branch].Isolation.Effective())
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the worktrees the command runs in, in order, the command, how many run at once and whether each one's whole output is kept; then the recap, which the fingerprint leaves out.
type Plan struct {
	Targets []Target `json:"targets"`
	Command string   `json:"command"`
	Jobs    int      `json:"jobs"`
	Print   bool     `json:"print"`
	domain.Recapped
	Op    string `json:"-"` // the operation applying it, which names each target's log
	facts Facts
	asked Request
}

// Locks are the locks the plan needs: the repository and each target shared, so that two runs share a worktree and nothing removes or moves one under a command.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		names = append(names, t.Worktree)
	}

	return shared(names)
}

// Locks are the locks taken before planning: the repository and each worktree asked shared.
func (r Request) Locks() []domain.Lock {
	return shared(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

func shared(names []string) []domain.Lock {
	locks := domain.WorktreeLocks(names)
	for i := range locks {
		locks[i].Mode = domain.LockShared
	}
	return locks
}

// Fingerprinted is what the plan's fingerprint covers: the plan, and what the environment of each target reads of its record (its ordinal and isolation); its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	type runEnv struct {
		Ordinal   int               `json:"ordinal"`
		Isolation project.Isolation `json:"isolation"`
	}
	envs := map[string]runEnv{}
	for _, t := range p.Targets {
		record := p.facts.Records[t.Branch]
		envs[t.Worktree] = runEnv{Ordinal: record.Ordinal, Isolation: record.Isolation.Effective()}
	}

	return struct {
		Plan
		RunEnvs map[string]runEnv `json:"run_envs"`
	}{p, envs}
}

// recap is the plan row by row: the worktrees, the command, and how many run at once.
func (p Plan) recap() []domain.RecapRow {
	names := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		names = append(names, t.Worktree)
	}

	return []domain.RecapRow{
		{Key: "worktrees", Label: "Worktrees", Path: domain.PathWorktree, Values: domain.RecapValues(names)},
		{Key: "command", Label: "Command", Path: PathCommand, Values: domain.RecapValues([]string{p.Command})},
		{Key: "concurrency", Label: "Concurrency", Path: PathJobs, Values: domain.RecapValues([]string{strconv.Itoa(p.Jobs)})},
	}
}

// ----------------------------------------------------------------------------
// The outcome

// CodeRetry is the follow-up of a run where some target did not pass: the command again, in those targets.
const CodeRetry = Name + ".retry"

// retry runs the command again in each target of items that did not pass, failed, interrupted or never reached; none when every one passed.
func (p Plan) retry(items []Item) *domain.FollowUp {
	var again []string
	for _, item := range items {
		if item.Status != domain.StatusDone {
			again = append(again, item.Subject)
		}
	}
	if len(again) == 0 {
		return nil
	}

	req := Request{Worktrees: again, Command: p.Command, Jobs: p.asked.Jobs}
	return &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), req), Code: CodeRetry, Message: "run it again where it did not pass"}
}

// tailLines is how many of its last lines a target's detail keeps.
const tailLines = 20

// escapes are the terminal's escape sequences a line may hold: CSI (colours, cursor moves), OSC (titles, links) ended by BEL or ESC \, and the two-byte ones.
var escapes = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[0-~])`)

// A tail keeps the last lines a command printed, each as a terminal would have left it.
type tail []string

func (t *tail) add(line string) {
	// A progress bar redraws its line with \r: only its last frame stays on screen; a CRLF line's \r ends it, redrawing nothing.
	line = strings.TrimSuffix(line, "\r")
	if at := strings.LastIndexByte(line, '\r'); at >= 0 {
		line = line[at+1:]
	}
	*t = append(*t, escapes.ReplaceAllString(line, ""))
	if len(*t) > tailLines {
		*t = (*t)[len(*t)-tailLines:]
	}
}
