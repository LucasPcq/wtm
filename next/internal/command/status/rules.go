package status

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what status reads: the repository, the worktrees the request reads (Targets), the meta record of each live branch (and the error of a record that exists but does not read), what each worktree read holds, what the run daemon and GitHub said, and the branches behind their merged pull request (repository.Behind).
type Facts struct {
	worktree.Repository
	repository.Asked
	Targets    []worktree.Worktree
	Records    map[string]worktree.Meta // by branch
	RecordErrs map[string]error         // by branch
	States     map[string]State         // by worktree path
	Behind     map[string]bool          // by branch
	Stack      worktree.Stack
}

// A State is what one worktree holds: whether its directory is gone, which leaves nothing else to read, its uncommitted changes and how much they weigh, its last commits, how its branch stands against the base branch (nil when it is not counted), the commits of its parent it lacks (zero for a root or when not counted), its declared .env files as read, what of these could not be read and why, and the error that kept the worktree from being read.
type State struct {
	Missing  bool
	Changes  []worktree.Change
	Diffstat worktree.Diffstat
	Commits  []worktree.Commit
	Base     *worktree.AheadBehind
	Lacks    int
	Env      []EnvRead
	Unread   []string
	Err      error
}

// An EnvRead is one .env file the project declares, as read for a worktree: the worktree's own copy and what it is rebuilt and compared from, its template, the parent worktree's copy and the main worktree's, each nil when it does not exist.
type EnvRead struct {
	File     project.EnvFile
	Child    []dotenv.Line
	Template []dotenv.Line
	Parent   []dotenv.Line
	Main     []dotenv.Line
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is one worktree as status reports it: where it is and what it is on, whether its directory is gone, its place in the stack, where its branch stands, its uncommitted work and last commits, its pull request when asked, then how it runs (its isolation, run.toml's addressing and its port offset, its jobs) and its .env files, and the problems found, each with the command that fixes it.
type Detail struct {
	Branch    string             `json:"branch"`
	Path      string             `json:"path"`
	Main      bool               `json:"main"`
	Locked    bool               `json:"locked"`
	Rebasing  bool               `json:"rebasing"`
	Missing   bool               `json:"missing"`    // its directory is gone: nothing of it is read
	CreatedAt *time.Time         `json:"created_at"` // from its meta record, null without one
	Parent    string             `json:"parent"`     // the branch it was created from, empty for a root
	Children  []string           `json:"children"`   // the branches created from it
	Base      *Base              `json:"base"`       // null for the main worktree, on no branch, or when the base branch does not exist
	Origin    *worktree.Origin   `json:"origin"`     // null when origin has no branch of that name
	Changes   *Changes           `json:"changes"`    // null when its directory is gone
	Commits   []worktree.Commit  `json:"commits"`    // null when its directory is gone
	Review    *Review            `json:"review"`     // null without a pull request, or when GitHub was not asked
	Isolation *project.Isolation `json:"isolation"`  // null while the worktree has not chosen, or when its record does not read
	RunToml   RunToml            `json:"run_toml"`
	Run       *Run               `json:"run"`  // null without a run.toml that reads
	Jobs      []Job              `json:"jobs"` // null when the run daemon cannot tell
	Env       Env                `json:"env"`
	Problems  []Problem          `json:"problems"`
}

// Base is where a worktree's branch stands against the base branch.
type Base struct {
	Name   string `json:"name"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

// Changes are a worktree's uncommitted changes: the files staged, modified (deleted and renamed included) and untracked, the lines inserted and deleted, and each file.
type Changes struct {
	Staged     int               `json:"staged"`
	Modified   int               `json:"modified"`
	Untracked  int               `json:"untracked"`
	Insertions int               `json:"insertions"`
	Deletions  int               `json:"deletions"`
	Files      []worktree.Change `json:"files"`
}

// Review is a worktree's newest pull request: its number, its page and its state, open, merged or closed; for an open one only, where its review stands; and what its pull request leaves to do, the same follow-up as the problem it raises: clean a worktree it merged.
type Review struct {
	Number int                       `json:"number"`
	URL    string                    `json:"url"`
	State  worktree.PullRequestState `json:"state"`
	*worktree.Progress
	Fix *domain.FollowUp `json:"fix"` // null when the pull request leaves nothing to do
}

// RunToml is whether the project's run.toml exists and reads.
type RunToml string

// The states of run.toml.
const (
	RunTomlDeclared RunToml = "declared"
	RunTomlAbsent   RunToml = "absent"
	RunTomlInvalid  RunToml = "invalid"
)

// Run is how run.toml runs a worktree: how its jobs are addressed, how far its ports sit above the main worktree's (null while no run has numbered it), and which ports it binds.
type Run struct {
	Addressing project.Addressing `json:"addressing"`
	Offset     *int               `json:"offset"`
	Ports      Ports              `json:"ports"`
}

// Ports says which ports a worktree binds against the main worktree's.
type Ports string

// The ports a worktree binds: the main worktree's own, its own offset above them, its source's (verbatim), none yet (not numbered), none until it chooses its isolation, or unknown when its record does not read.
const (
	PortsBase        Ports = "base"
	PortsOffset      Ports = "offset"
	PortsSource      Ports = "source"
	PortsNotNumbered Ports = "not_numbered"
	PortsNotChosen   Ports = "not_chosen"
	PortsUnknown     Ports = "unknown"
)

// A Job is one job a worktree can run, as wtm's events report it: its state, its url, the port it publishes, the code it crashed with, when it started, and for a shared job the worktree that runs it, when it is another.
type Job struct {
	Name      string          `json:"name"`
	Kind      project.JobKind `json:"kind"`
	State     run.State       `json:"state"`
	URL       string          `json:"url"`
	Port      *int            `json:"port,omitempty"`
	ExitCode  *int            `json:"exit_code,omitempty"`
	StartedAt *time.Time      `json:"started_at,omitempty"`
	Shared    bool            `json:"shared,omitempty"`
	Owner     *Owner          `json:"owner,omitempty"`
}

// Owner is the worktree a shared job runs in.
type Owner struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// Env is a worktree's .env files: how many the project declares, and each one, whether it is missing and how its keys stand against its sources. It never carries a value.
type Env struct {
	Declared int       `json:"declared"`
	Files    []EnvFile `json:"files"` // null when its directory is gone
}

// EnvFile is one declared .env file of a worktree.
type EnvFile struct {
	Target  string    `json:"target"`
	Missing bool      `json:"missing"`
	Keys    env.Drift `json:"keys"`
}

// A Problem is something to fix in a worktree, and the command that fixes it.
type Problem struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Fix     *domain.FollowUp `json:"fix"` // null when no command fixes it
}

// The codes of status's problems, warnings and failures.
const (
	CodeMissing          = Name + ".missing"
	CodeMerged           = Name + ".merged"
	CodeBehind           = Name + ".behind"
	CodeEnvMissing       = Name + ".env_missing"
	CodeIsolationPending = Name + ".isolation_pending"
	CodeJobCrashed       = Name + ".job_crashed"
	CodeJobsUnknown      = Name + domain.JobsUnknown
	CodePRsUnavailable   = Name + domain.PRsUnavailable
	CodeRunInvalid       = Name + ".run_invalid"
	CodeBaseMissing      = Name + ".base_missing"
	CodeMetaUnreadable   = Name + domain.MetaUnreadable
	CodePartial          = Name + ".partial"
	CodeNoWorktree       = Name + ".no_worktree"
	CodeUnreadable       = Name + domain.Unreadable
	CodeAmbiguous        = Name + ".ambiguous"
	CodeAllWithWorktree  = Name + ".all_with_worktree"
)

// ----------------------------------------------------------------------------
// The worktrees read

// Targets are the worktrees req reads of snapshot: every one with All, the main first; else the one it names, by its branch, by a path (located, resolved and existing, "" when the name is none) in it, or by a part of its branch found in no other; else the deepest holding req.Caller.Cwd, where the caller stands; else the main one. It refuses All with a worktree named, a name that matches no worktree or several, and a repository with no worktree to read.
func Targets(snapshot worktree.Snapshot, req Request, located string) ([]worktree.Worktree, error) {
	worktrees := listed(snapshot)
	switch {
	case len(worktrees) == 0 && !req.All:
		return nil, &domain.Error{Kind: domain.KindPrecondition, Code: CodeNoWorktree, Message: "the repository has no worktree to read"}
	case req.All && req.Worktree != "":
		return nil, &domain.Error{Kind: domain.KindInvalid, Code: CodeAllWithWorktree, Message: "the request is invalid", Fields: []domain.FieldError{
			{Path: pathAll, Code: CodeAllWithWorktree, Template: "{path} reads every worktree: drop the worktree named", With: []string{domain.PathWorktree}},
		}}
	case req.All:
		return worktrees, nil
	case req.Worktree != "":
		named, err := named(worktrees, req.Worktree, located)
		if err != nil {
			return nil, err
		}
		return []worktree.Worktree{named}, nil
	}

	if at := worktree.Holding(worktrees, req.Caller.Cwd, pathOf); at >= 0 {
		return []worktree.Worktree{worktrees[at]}, nil
	}
	return worktrees[:1], nil
}

// named is the worktree query names: the one on that branch, else the deepest holding the path located, else the only one whose branch holds query.
func named(worktrees []worktree.Worktree, query, located string) (worktree.Worktree, error) {
	if at := slices.IndexFunc(worktrees, func(w worktree.Worktree) bool { return w.Branch() == query }); at >= 0 {
		return worktrees[at], nil
	}
	if at := worktree.Holding(worktrees, located, pathOf); at >= 0 {
		return worktrees[at], nil
	}

	var matches []worktree.Worktree
	var names []string
	for _, w := range worktrees {
		if w.Branch() != "" && strings.Contains(w.Branch(), query) {
			matches, names = append(matches, w), append(names, w.Branch())
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return worktree.Worktree{}, domain.Invalid([]domain.FieldError{{Path: domain.PathWorktree, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": query}}})
	}
	return worktree.Worktree{}, domain.Invalid([]domain.FieldError{{Path: domain.PathWorktree, Code: CodeAmbiguous, Template: "{value} matches several worktrees: {matches}", Params: domain.Params{"value": query, "matches": strings.Join(names, ", ")}}})
}

// listed are the worktrees status reads, the main first: all but a bare repository's entry, which has no working tree.
func listed(snapshot worktree.Snapshot) []worktree.Worktree {
	worktrees := snapshot.Checkouts()
	slices.SortStableFunc(worktrees, func(a, b worktree.Worktree) int {
		switch {
		case a.Main == b.Main:
			return 0
		case a.Main:
			return -1
		}
		return 1
	})

	return worktrees
}

// ----------------------------------------------------------------------------
// Report

// Report is each worktree the facts read as an item, in order, and the warnings of what could not be read.
func Report(facts Facts) ([]Item, []domain.Warning) {
	items := make([]Item, 0, len(facts.Targets))
	for _, w := range facts.Targets {
		items = append(items, facts.item(w))
	}

	return items, facts.warnings()
}

func (f Facts) item(w worktree.Worktree) Item {
	state := f.States[w.Path]
	subject := w.Name()
	if state.Err != nil {
		item := domain.Unread[Detail](domain.UnreadParams{Command: Name, Subject: subject, Path: w.Path, Err: state.Err})
		item.Warnings = f.recordWarnings(w.Branch())
		return item
	}

	branch := w.Branch()
	record, recordErr := f.Records[branch], f.RecordErrs[branch]
	_, rebasing := w.State.(worktree.Rebasing)
	parent, _ := f.Stack.Parent(branch)
	pending := recordErr == nil && f.pending(w)
	offset := f.offset(w)
	pr, hasPR := worktree.PullRequestOf(f.PullRequests, branch)
	merged := !state.Missing && f.Repository.Cleans(pr, w, f.Behind[branch])

	detail := Detail{
		Branch:    branch,
		Path:      w.Path,
		Main:      w.Main,
		Locked:    w.Locked,
		Rebasing:  rebasing,
		Missing:   state.Missing,
		CreatedAt: record.Created(),
		Parent:    parent,
		Children:  orEmpty(f.Stack.Children(branch)),
		Base:      f.base(state),
		Origin:    f.OriginOf(branch),
		Changes:   changesOf(state),
		Commits:   commitsOf(state),
		Review:    reviewOf(pr, hasPR, merged),
		Isolation: isolationOf(record, pending || recordErr != nil),
		RunToml:   f.runToml(),
		Run:       f.runOf(w, portsParams{record: record, pending: pending, unread: recordErr != nil, offset: offset}),
		Jobs:      f.jobs(w, offset),
		Env:       f.envOf(w, state),
	}
	detail.Problems = Problems(ProblemsParams{Branch: branch, Gone: state.Missing, Merged: merged, Parent: parent, Lacks: state.Lacks, Missing: f.missing(w, state), Pending: pending, Jobs: detail.Jobs})

	item := Item{Subject: subject, Status: domain.StatusDone, Detail: &detail, Warnings: f.recordWarnings(branch)}
	if len(state.Unread) > 0 {
		item.Warnings = append(item.Warnings, domain.Warning{Code: CodePartial, Message: "couldn't read all of " + subject + ": " + strings.Join(state.Unread, "; "), Params: domain.Params{"path": w.Path}})
	}
	return item
}

// recordWarnings are the warnings of the record of branch when it exists but does not read, none otherwise.
func (f Facts) recordWarnings(branch string) []domain.Warning {
	err := f.RecordErrs[branch]
	if err == nil {
		return nil
	}

	return []domain.Warning{{Code: CodeMetaUnreadable, Message: "couldn't read the record of " + branch + ": its isolation, parent, env strategy and port offset are not known: " + err.Error(), Params: domain.Params{"branch": branch}}}
}

func (f Facts) base(state State) *Base {
	if state.Base == nil {
		return nil
	}

	return &Base{Name: f.Config.Worktrees.BaseBranch, Ahead: state.Base.Ahead, Behind: state.Base.Behind}
}

func changesOf(state State) *Changes {
	if state.Missing {
		return nil
	}

	changes := &Changes{Staged: state.Diffstat.Staged, Modified: state.Diffstat.Unstaged, Insertions: state.Diffstat.Insertions, Deletions: state.Diffstat.Deletions, Files: orEmpty(state.Changes)}
	for _, change := range state.Changes {
		if change.Status == worktree.ChangeUntracked {
			changes.Untracked++
		}
	}

	return changes
}

func commitsOf(state State) []worktree.Commit {
	if state.Missing {
		return nil
	}
	return orEmpty(state.Commits)
}

// reviewOf is the review of pr when ok, its fix the merged problem's when it merged the worktree.
func reviewOf(pr worktree.PullRequest, ok, merged bool) *Review {
	if !ok {
		return nil
	}

	review := &Review{Number: pr.Number, URL: pr.URL, State: pr.State, Progress: pr.Progress()}
	if merged {
		review.Fix = mergedFix(pr.Branch)
	}
	return review
}

// pending reports whether w has yet to choose its isolation, which run up and run start then refuse: a worktree on a branch other than the main one, whose record holds no choice, while run.toml declares something a worktree isolates.
func (f Facts) pending(w worktree.Worktree) bool {
	return !w.Main && w.Branch() != "" && f.RunErr == nil && f.Records[w.Branch()].Isolation == "" && f.Run.IsolationApplies()
}

func isolationOf(record worktree.Meta, unknown bool) *project.Isolation {
	if unknown {
		return nil
	}

	isolation := record.Isolation.Effective()
	return &isolation
}

// offset is how far w's ports sit above the main worktree's: none for the main one; for another, its ordinal's, unless no run has numbered it yet or another branch, first by name, holds the same number; nil when it cannot be told.
func (f Facts) offset(w worktree.Worktree) *int {
	if w.Main {
		return new(int)
	}

	branch := w.Branch()
	record, recorded := f.Records[branch]
	if branch == "" || !recorded || record.Ordinal <= 0 {
		return nil
	}
	for _, other := range f.Worktrees {
		if name := other.Branch(); name != branch && name < branch && f.Records[name].Ordinal == record.Ordinal {
			return nil
		}
	}

	offset := env.JobParams{Ordinal: record.Ordinal, Isolation: record.Isolation, Run: f.Run}.Offset()
	return &offset
}

func (f Facts) runToml() RunToml {
	switch {
	case f.RunErr != nil:
		return RunTomlInvalid
	case f.Run.Declared:
		return RunTomlDeclared
	}
	return RunTomlAbsent
}

// portsParams are what decides which ports a worktree binds: its record, whether it has yet to choose its isolation, whether its record does not read, and its offset.
type portsParams struct {
	record  worktree.Meta
	pending bool
	unread  bool
	offset  *int
}

func (f Facts) runOf(w worktree.Worktree, params portsParams) *Run {
	if f.runToml() != RunTomlDeclared {
		return nil
	}

	return &Run{Addressing: env.EffectiveAddressing(f.Run), Offset: params.offset, Ports: portsOf(w, params)}
}

func portsOf(w worktree.Worktree, params portsParams) Ports {
	switch {
	case params.pending:
		return PortsNotChosen
	case params.unread:
		return PortsUnknown
	case params.record.Isolation == project.Verbatim:
		return PortsSource
	case w.Main:
		return PortsBase
	case params.offset == nil:
		return PortsNotNumbered
	}
	return PortsOffset
}

// ----------------------------------------------------------------------------
// The jobs

// jobs are the jobs w can run, nil when the daemon cannot tell: each job run.toml declares, in its order, as the daemon holds it in w or stopped when it holds nothing, then each job up in w that run.toml no longer declares.
func (f Facts) jobs(w worktree.Worktree, offset *int) []Job {
	if f.JobsErr != nil {
		return nil
	}

	held := f.held(w.Path)
	var declared []project.Job
	if f.RunErr == nil {
		declared = f.Run.Jobs
	}

	jobs := []Job{}
	for _, job := range declared {
		at := slices.IndexFunc(held, func(h Job) bool { return h.Name == job.Name })
		found := Job{Name: job.Name, Kind: job.Kind, State: run.Stopped}
		if at >= 0 {
			found = held[at]
		}
		found.Port = portOf(job, offset)
		jobs = append(jobs, found)
	}
	for _, h := range held {
		up := h.State == run.Running || h.State == run.Starting
		if up && !slices.ContainsFunc(declared, func(job project.Job) bool { return job.Name == h.Name }) {
			jobs = append(jobs, h)
		}
	}

	return jobs
}

// held are the jobs the daemon holds in the worktree at path, by name, one each: a job it runs there, else its claim on a shared job, which reads as the instance it claims, with that instance's url and its owner.
func (f Facts) held(path string) []Job {
	var held []Job
	add := func(job Job) {
		if job.State != "" && !slices.ContainsFunc(held, func(h Job) bool { return h.Name == job.Name }) {
			held = append(held, job)
		}
	}

	for _, job := range f.Jobs {
		if job.WorkDir == path && !job.Claim {
			add(jobOf(job))
		}
	}
	for _, claim := range f.Jobs {
		if claim.WorkDir != path || !claim.Claim || claim.SharedDir == "" {
			continue
		}
		at := slices.IndexFunc(f.Jobs, func(job run.Job) bool { return job.Name == claim.Name && job.WorkDir == claim.SharedDir && !job.Claim })
		if at < 0 {
			continue
		}
		instance := jobOf(f.Jobs[at])
		instance.URL = cmp.Or(instance.URL, claim.URL)
		instance.Owner = &Owner{Branch: f.Occupant(worktree.Location{Path: claim.SharedDir}), Path: claim.SharedDir}
		add(instance)
	}
	slices.SortFunc(held, func(a, b Job) int { return strings.Compare(a.Name, b.Name) })

	return held
}

// jobOf is the job the daemon holds, its exit code kept only for a crash: a stop's signal leaves one that says nothing of the job.
func jobOf(job run.Job) Job {
	found := Job{Name: job.Name, Kind: job.Kind, State: job.State, URL: job.URL, Shared: job.SharedDir != ""}
	if job.State == run.Crashed {
		found.ExitCode = job.ExitCode
	}
	if !job.StartedAt.IsZero() {
		found.StartedAt = &job.StartedAt
	}

	return found
}

// portOf is the port job publishes in a worktree whose ports sit offset above the main's: its url's port, the same in every worktree for a shared job; nil when it publishes none or the offset is unknown.
func portOf(job project.Job, offset *int) *int {
	if job.URL == nil || job.Ports[job.URL.Port] == 0 || offset == nil && job.Scope != project.Shared {
		return nil
	}

	port := job.Ports[job.URL.Port]
	if job.Scope != project.Shared {
		port += *offset
	}
	return &port
}

// ----------------------------------------------------------------------------
// The .env files

func (f Facts) envOf(w worktree.Worktree, state State) Env {
	if state.Missing {
		return Env{Declared: len(f.Config.Env.Files)}
	}

	strategy := f.strategy(w)
	files := []EnvFile{}
	for _, read := range state.Env {
		file := EnvFile{Target: read.File.Target, Missing: read.Child == nil}
		if !file.Missing {
			parent, main := sourcesOf(strategy, read)
			file.Keys = env.KeyDrift(env.DriftParams{Child: read.Child, Template: read.Template, Parent: parent, Main: main, Owned: env.Owned(f.Run, read.File.Target)})
		}
		files = append(files, file)
	}

	return Env{Declared: len(f.Config.Env.Files), Files: files}
}

// strategy is where w's .env files come from: its record's strategy, else the project's, else the default.
func (f Facts) strategy(w worktree.Worktree) project.EnvStrategy {
	return cmp.Or(f.Records[w.Branch()].EnvStrategy, f.Config.Env.Strategy, project.DefaultEnvStrategy)
}

// sourcesOf are the copies a file's values come from under strategy: none with the template's, the main worktree's, or the parent's when it has one, else the main's.
func sourcesOf(strategy project.EnvStrategy, read EnvRead) (parent, main []dotenv.Line) {
	switch {
	case strategy == project.EnvMain:
		return nil, read.Main
	case strategy == project.EnvParent && read.Parent != nil:
		return read.Parent, nil
	case strategy == project.EnvParent:
		return nil, read.Main
	}

	return nil, nil
}

// missing are the declared .env files w lacks, each with what rebuilds it.
func (f Facts) missing(w worktree.Worktree, state State) []Missing {
	strategy := f.strategy(w)
	var missing []Missing
	for _, read := range state.Env {
		if read.Child == nil {
			missing = append(missing, Missing{Target: read.File.Target, From: rebuildFrom(strategy, read)})
		}
	}

	return missing
}

// rebuildFrom is what a missing file is rebuilt from: the worktree's own source when it has a copy, else the template, else the main worktree's copy, else nothing.
func rebuildFrom(strategy project.EnvStrategy, read EnvRead) Rebuild {
	parent, main := sourcesOf(strategy, read)
	switch {
	case strategy == project.EnvExample && read.Template != nil, parent != nil, main != nil:
		return FromSource
	case read.Template != nil:
		return FromExample
	case read.Main != nil:
		return FromMain
	}

	return FromNowhere
}

// ----------------------------------------------------------------------------
// The problems

// Rebuild is what a missing .env file is rebuilt from.
type Rebuild string

// The sources of a missing .env file: the worktree's own strategy, the template, the main worktree's copy, or none.
const (
	FromSource  Rebuild = ""
	FromExample Rebuild = "example"
	FromMain    Rebuild = "main"
	FromNowhere Rebuild = "nowhere"
)

// Missing is a declared .env file a worktree lacks, and what rebuilds it.
type Missing struct {
	Target string
	From   Rebuild
}

// ProblemsParams are what a worktree has to fix: its branch, which every fix names, empty on none; whether its directory is gone, whether its merged pull request leaves it to be cleaned, the .env files it lacks, whether it has yet to choose its isolation, and its jobs.
type ProblemsParams struct {
	Branch  string
	Gone    bool
	Merged  bool
	Parent  string
	Lacks   int // the commits of Parent the worktree lacks
	Missing []Missing
	Pending bool
	Jobs    []Job
}

// The commands and the answers of the fixes, as the legacy wtm runs them until they migrate.
const (
	commandEnv        = domain.CommandEnv
	commandConfigEdit = "config edit"
	commandRunStart   = "run start"
	commandSync       = "sync"
	pathFrom          = "from"
	pathJob           = "job"
	pathDetach        = "detach"
)

// Problems are the problems of params, each with its fix: a gone directory alone, since only cleaning the worktree fixes it, with no fix on no branch, which clean cannot name; nothing else on no branch, since every fix names a branch; else a merged pull request first, since cleaning the worktree makes every other fix moot, then each missing .env file, then a pending isolation, then each crashed job, whose fix starts it again in the worktree that runs it, a shared job's owner.
func Problems(params ProblemsParams) []Problem {
	if params.Gone {
		gone := Problem{Code: CodeMissing, Message: "its directory is gone"}
		if params.Branch != "" {
			gone.Fix = fix(CodeMissing, "remove what is left of it", domain.CommandClean, params.Branch)
		}
		return []Problem{gone}
	}

	problems := []Problem{}
	if params.Branch == "" {
		return problems
	}
	if params.Merged {
		problems = append(problems, Problem{Code: CodeMerged, Message: "its pull request is merged", Fix: mergedFix(params.Branch)})
	}
	if params.Lacks > 0 {
		problems = append(problems, Problem{Code: CodeBehind, Message: params.Parent + " has " + commits(params.Lacks) + " it lacks", Fix: fix(CodeBehind, "sync it onto "+params.Parent, commandSync, params.Branch)})
	}
	for _, missing := range params.Missing {
		problems = append(problems, missingProblem(params.Branch, missing))
	}
	if params.Pending {
		problems = append(problems, Problem{
			Code:    CodeIsolationPending,
			Message: "predates the isolation choice: run up and run start refuse it",
			Fix:     fix(CodeIsolationPending, "isolate it", commandEnv, params.Branch, domain.Answered{Path: domain.PathIsolation, Values: []string{string(project.Isolated)}}),
		})
	}
	for _, job := range params.Jobs {
		if job.State != run.Crashed {
			continue
		}
		owner := params.Branch
		if job.Owner != nil {
			owner = cmp.Or(job.Owner.Branch, job.Owner.Path)
		}
		problems = append(problems, Problem{
			Code:    CodeJobCrashed,
			Message: crashed(job),
			Fix:     fix(CodeJobCrashed, "start it again", commandRunStart, owner, domain.Answered{Path: pathJob, Values: []string{job.Name}}, domain.Answered{Path: pathDetach, Values: []string{"true"}}),
		})
	}

	return problems
}

func missingProblem(subject string, missing Missing) Problem {
	problem := Problem{Code: CodeEnvMissing, Message: missing.Target + " is missing"}
	switch missing.From {
	case FromSource:
		problem.Fix = fix(CodeEnvMissing, "rebuild it", commandEnv, subject)
	case FromExample, FromMain:
		problem.Fix = fix(CodeEnvMissing, "rebuild it", commandEnv, subject, domain.Answered{Path: pathFrom, Values: []string{string(missing.From)}})
	case FromNowhere:
		problem.Message = missing.Target + " is missing, with no copy to rebuild it from and no template: declare it where it exists, or write it by hand"
		problem.Fix = &domain.FollowUp{Command: commandConfigEdit, Code: CodeEnvMissing, Message: "declare where it comes from"}
	}

	return problem
}

// fix is the follow-up that runs command on the worktree subject with answers, saying what it does with message.
func fix(code, message, command, subject string, answers ...domain.Answered) *domain.FollowUp {
	return &domain.FollowUp{Command: command, Answers: append([]domain.Answered{{Path: domain.PathWorktree, Values: []string{subject}}}, answers...), Code: code, Message: message}
}

func commits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return strconv.Itoa(n) + " commits"
}

func mergedFix(branch string) *domain.FollowUp {
	return fix(CodeMerged, "clean it up", domain.CommandClean, branch)
}

func crashed(job Job) string {
	switch {
	case job.ExitCode == nil:
		return job.Name + " crashed"
	case *job.ExitCode < 0:
		return job.Name + " crashed (killed by a signal)"
	}

	return job.Name + " crashed (exit " + strconv.Itoa(*job.ExitCode) + ")"
}

// ----------------------------------------------------------------------------
// The warnings

func (f Facts) warnings() []domain.Warning {
	var warnings []domain.Warning
	if base, missing := f.MissingBase(); missing {
		warnings = append(warnings, domain.Warning{Code: CodeBaseMissing, Message: "the base branch " + base + " does not exist: where a branch stands against it is not shown", Params: domain.Params{"branch": base}})
	}
	if f.RunErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeRunInvalid, Message: "run.toml does not read: its jobs and ports are not shown: " + f.RunErr.Error()})
	}

	return append(warnings, f.Asked.Warnings(Name)...)
}

// ----------------------------------------------------------------------------
// Helpers

// pathOf is where w is.
func pathOf(w worktree.Worktree) string {
	return w.Path
}

// orEmpty is list, empty rather than nil: a list status reads is never null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
