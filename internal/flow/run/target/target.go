// Package target holds the questions every run command shares: which worktree
// it acts on, and what inside it — a job or a profile. They live here rather
// than in each command's package for the reason internal/flow/decide exists:
// one wording, one bypass taxonomy, one place a surface has to learn.
package target

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeyWorktree = "run.worktree"
	KeyJob      = "run.job"
	KeyProfile  = "run.profile"
)

type ResolveParams struct {
	ProjectDir string
	// Query is the positional as it was typed: a branch, a path, a prefix.
	Query string
}

// Resolved is a named worktree: where it is, and the branch it carries.
type Resolved struct {
	Dir    string
	Branch string
}

// Resolve turns the positional into a worktree, and refuses a name matching
// none or several. It is the flow's job rather than the runner's: naming a
// worktree is a business input, and the two surfaces must refuse the same names.
func Resolve(params ResolveParams) (Resolved, error) {
	result, err := worktree.Resolve(domain.ResolveParams{ProjectDir: params.ProjectDir, Query: params.Query})
	if err != nil {
		return Resolved{}, fmt.Errorf("worktree %q: %w", params.Query, err)
	}
	if result.Ambiguous {
		return Resolved{}, fmt.Errorf("worktree %q is ambiguous: matches %s", params.Query, branchNames(result.Matches))
	}
	return Resolved{Dir: result.Path, Branch: result.Branch}, nil
}

// Root is the worktree containing dir as git spells it, and dir itself when git
// cannot say — a path outside any repository is refused later, by the command
// that needed a worktree, rather than here.
func Root(dir string) string {
	root, err := worktree.Root(dir)
	if err != nil {
		return dir
	}
	return root
}

// BranchOf names a worktree the way a reader recognises it, and answers empty
// for one git cannot name — which is what makes such a worktree persist nothing
// rather than share another's log directory.
func BranchOf(dir string) string {
	branch, err := worktree.CurrentBranch(worktree.CurrentBranchParams{Dir: dir})
	if err != nil {
		return ""
	}
	return branch
}

type NamedBranchParams struct {
	Named []Resolved
	Dir   string
}

// NamedBranch is the branch the positional named at dir, else the one git
// reports there.
func NamedBranch(params NamedBranchParams) string {
	for _, named := range params.Named {
		if named.Dir == params.Dir && named.Branch != "" {
			return named.Branch
		}
	}
	return BranchOf(params.Dir)
}

// ProjectOf names the repository a worktree belongs to — its main checkout's
// directory, as every run name spells the project — empty when git cannot say.
func ProjectOf(dir string) string {
	main, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: dir})
	if err != nil {
		return ""
	}
	return filepath.Base(main)
}

// RunningJobs counts what the daemon holds per worktree, and answers nothing
// when it cannot be reached: the counts decorate a picker, they never gate it.
func RunningJobs(service runlogs.Service) map[string]int {
	if service == nil {
		return nil
	}
	jobs, err := service.List("")
	if err != nil {
		return nil
	}
	return rules.RunningJobsByWorktree(jobs)
}

// DeclaredJob turns an answered job name into its declaration, so a name
// matching nothing in run.toml fails precisely instead of at the daemon.
func DeclaredJob(cfg domain.RunConfig, name string) (domain.JobConfig, error) {
	if name == "" {
		return domain.JobConfig{}, domain.ErrJobRequired
	}
	job, ok := rules.FindJob(cfg, name)
	if !ok {
		return domain.JobConfig{}, fmt.Errorf("%w: %s", domain.ErrJobNotFound, name)
	}
	return job, nil
}

func branchNames(worktrees []domain.GitWorktree) string {
	names := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		names = append(names, wt.Branch)
	}
	return strings.Join(names, domain.RunURLListSep)
}

// worktreeList reads the repository's worktrees once, however many of the step's
// callbacks ask for them. A failure is remembered too: it means "do not ask",
// and the step's Resolve then answers with the current worktree.
type worktreeList struct {
	projectDir string
	once       sync.Once
	worktrees  []domain.GitWorktree
	err        error
}

func (l *worktreeList) get() ([]domain.GitWorktree, error) {
	l.once.Do(func() {
		l.worktrees, l.err = worktree.ListAll(worktree.ListAllParams{ProjectDir: l.projectDir})
	})
	return l.worktrees, l.err
}

// branchesOf names a set of worktrees the way a recap shows them, capped so a
// wide selection does not overflow the line.
func (l *worktreeList) branchesOf(paths []string) string {
	if len(paths) == 0 {
		return domain.SummaryNone
	}
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, l.branchOf(path))
	}
	const maxNames = 5
	if len(names) <= maxNames {
		return strings.Join(names, domain.RunURLListSep)
	}
	return strings.Join(names[:maxNames], domain.RunURLListSep) + fmt.Sprintf(" +%d", len(names)-maxNames)
}

func (l *worktreeList) branchOf(path string) string {
	worktrees, err := l.get()
	if err != nil {
		return path
	}
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt.Branch
		}
	}
	return path
}

// Named resolves the positional, and reports nil when there was none — which is
// the difference between "this worktree" and "no worktree named yet".
func Named(params ResolveParams) (*Resolved, error) {
	if params.Query == "" {
		return nil, nil
	}
	resolved, err := Resolve(params)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

type ResolveAllParams struct {
	ProjectDir string
	// Queries are the positionals as they were typed, in that order.
	Queries []string
}

// NamedAll resolves every positional, and reports nil when there was none. A
// query matching nothing or several worktrees refuses the whole run: acting on
// part of what was asked for would be worse than not acting.
func NamedAll(params ResolveAllParams) ([]Resolved, error) {
	if len(params.Queries) == 0 {
		return nil, nil
	}
	resolved := make([]Resolved, 0, len(params.Queries))
	for _, query := range params.Queries {
		one, err := Resolve(ResolveParams{ProjectDir: params.ProjectDir, Query: query})
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, one)
	}
	return resolved, nil
}

type PreselectedParams struct {
	Named []Resolved
	// Precheck is what a surface already knowing a likely answer offers, as
	// worktree roots git spells: it only arrives ticked, and never presets the
	// step — the selection stays exact.
	Precheck []string
}

// Preselected is what the worktree step opens with ticked: the positionals when
// there are any, else what the surface proposed.
func Preselected(params PreselectedParams) []string {
	if len(params.Named) > 0 {
		return Dirs(params.Named)
	}
	return params.Precheck
}

// Dirs is where a set of named worktrees are, in the order they were named.
func Dirs(resolved []Resolved) []string {
	dirs := make([]string, 0, len(resolved))
	for _, one := range resolved {
		dirs = append(dirs, one.Dir)
	}
	return dirs
}

type WorkDirsParams struct {
	Answers flow.Answers
	Named   []Resolved
	Cwd     string
}

// WorkDirs is the set of worktrees a run acts on: what the step answered, else
// the positionals, else where the command was launched. It is WorkDir's
// cumulative form, and answers a set of one wherever that one is all there is.
func WorkDirs(params WorkDirsParams) []string {
	if answered := params.Answers.Values(KeyWorktree); len(answered) > 0 {
		return dedupe(answered)
	}
	if len(params.Named) > 0 {
		return dedupe(Dirs(params.Named))
	}
	return []string{Root(params.Cwd)}
}

// dedupe keeps the first mention of each worktree. A branch and a path can name
// the same one, and running it twice would race two identical sequences against
// each other — the second failing on jobs the first had just started.
func dedupe(dirs []string) []string {
	seen := make(map[string]bool, len(dirs))
	unique := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		unique = append(unique, dir)
	}
	return unique
}

type WorkDirParams struct {
	Answers flow.Answers
	Named   *Resolved
	Cwd     string
}

// WorkDir is the worktree a run acts on, as git spells it: what the step
// answered, else the positional, else where the command was launched.
func WorkDir(params WorkDirParams) string {
	if answered := params.Answers.Value(KeyWorktree); answered != "" {
		return answered
	}
	if params.Named != nil {
		return params.Named.Dir
	}
	return Root(params.Cwd)
}

type PickOneParams struct {
	Prompter flow.Prompter
	// Step is the picker to run when the positional did not answer.
	Step  flow.Step
	Given string
	// Subject names what the argument would have carried and Label the command
	// asking for it. The refusal is worded here rather than left to the step
	// because these commands take their subject positionally, so the step's own
	// message would name a flag they do not have.
	Subject string
	Label   string
}

// PickOne answers "which one" without asking when the positional already did,
// and refuses rather than guessing when nobody can be asked.
func PickOne(params PickOneParams) (string, error) {
	if params.Given != "" {
		return params.Given, nil
	}
	if !params.Prompter.Interactive() {
		return "", fmt.Errorf(domain.RunCRUDNeedsArgFmt, params.Label, params.Subject)
	}
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: params.Label,
		Steps:    []flow.Step{params.Step},
	})
	if err != nil {
		return "", err
	}
	return answers.Value(params.Step.Key), nil
}

type PresetParams struct {
	Named *Resolved
	// Worktrees are the paths a cumulative command's positionals named. It is the
	// set form of Named, and the two are never both set.
	Worktrees []string
	Job       string
	Profile   string
}

// Presets carry what the flags and the positional already answered. A preset
// step is not asked but is still read back, which is what keeps a flag from
// erasing a line from the recap.
func Presets(params PresetParams) flow.Answers {
	values := map[string]string{KeyJob: params.Job, KeyProfile: params.Profile}
	if params.Named != nil {
		values[KeyWorktree] = params.Named.Dir
	}
	return flow.NewAnswers(values).WithValues(KeyWorktree, params.Worktrees)
}
