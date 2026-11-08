package daemonstopjobs

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what stop-jobs decides from: the daemon as it stands, and the main checkout of the repository the request names, or why the path names none.
type Facts struct {
	Daemon  run.Daemon
	Repo    string
	RepoErr error
}

// path is the repository's path the request names, a relative one read from where the caller stands.
func (r Request) path() string {
	if filepath.IsAbs(r.Repo) {
		return r.Repo
	}
	return filepath.Join(r.Caller.Cwd, r.Repo)
}

// ----------------------------------------------------------------------------
// The form

// NewQuestions builds the questions for the request on the facts: a repository path in no repository is its problem.
func NewQuestions(f Facts, req Request) Questions {
	questions := Questions{Request: req}
	if req.Repo != "" && f.RepoErr != nil {
		questions.Errors = append(questions.Errors, domain.FieldError{Path: domain.PathRepo, Code: domain.CodeNotFound, Template: "{value} is in no repository ({reason})", Params: domain.Params{"value": req.Repo, "reason": f.RepoErr.Error()}})
	}
	return questions
}

// ----------------------------------------------------------------------------
// Decide

// The codes of a stop-jobs refused or warned of: the daemon's jobs cannot be read, it is of another build.
const (
	CodeDaemonSkew = Name + domain.DaemonSkew
)

// Decide turns the facts and the request into the plan: each worktree's jobs up, of the repository asked or of every one, by repository, a crashed job cleared, a shared job's own run left to its holders' stop when the main checkout does not hold it. A daemon whose jobs cannot be read is refused.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	d := f.Daemon
	if d.Answering && d.Skew == run.SkewProtocol {
		return Plan{}, nil, &domain.Error{Kind: domain.KindPrecondition, Code: CodeDaemonSkew, Message: "nothing stopped: wtm's daemon speaks another protocol, its jobs can't be read — restart it"}
	}

	plan := Plan{Repo: f.Repo, Repos: []Repo{}}
	jobs := run.Placed(d.Jobs)
	for _, job := range jobs {
		if f.Repo != "" && job.Repo != f.Repo {
			continue
		}
		stop, ok := stopOf(job, jobs)
		if !ok {
			continue
		}
		plan.add(job, stop)
	}
	plan.Recap, plan.Empty = plan.recap(), len(plan.Repos) == 0
	return plan, nil, nil
}

// stopOf is how job stops when every job does: up, it stops (a stack by its stop command, a shared service let go of by its main checkout and every worktree holding it, which it names); crashed, it is cleared.
func stopOf(job run.Job, jobs []run.Job) (runjobs.StopJob, bool) {
	stop := runjobs.StopJob{Name: job.Name, Kind: job.Kind, Hold: job.Hold, Fate: run.FateStop}
	switch {
	case !job.Up && job.State == run.Crashed:
		stop.Fate = run.FateClear
		return stop, true
	case !job.Up:
		return stop, false
	case job.Hold == run.HoldDetached:
		stop.StopCommand = job.StopCommand
	}
	if job.SharedDir == "" {
		return stop, true
	}
	// A hold on a shared service, or its own run, which the main checkout's stop lets go of: it stops with its last hold.
	stop.SharedIn = job.SharedDir
	if job.Hold != run.HoldClaim {
		for _, held := range jobs {
			if held.Up && held.Hold == run.HoldClaim && held.Name == job.Name && held.SharedDir == job.WorkDir {
				stop.HeldBy = append(stop.HeldBy, cmp.Or(held.Branch, held.WorkDir))
			}
		}
	}
	return stop, true
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the repository it was narrowed to (none: every one), each repository's worktrees with the jobs that stop there: then the recap, which the fingerprint leaves out.
type Plan struct {
	Repo  string `json:"repo,omitempty"`
	Repos []Repo `json:"repos"`
	domain.Recapped
	correlationID string
}

// A Repo is the worktrees of one repository whose jobs stop, by its main checkout's root (empty for the jobs started for none).
type Repo struct {
	Root      string               `json:"root"`
	Worktrees []runjobs.StopTarget `json:"worktrees"`
}

// add puts the stop of job under its repository and its worktree, each kept in the order of their paths.
func (p *Plan) add(job run.Job, stop runjobs.StopJob) {
	at, found := slices.BinarySearchFunc(p.Repos, job.Repo, func(r Repo, root string) int { return cmp.Compare(r.Root, root) })
	if !found {
		p.Repos = slices.Insert(p.Repos, at, Repo{Root: job.Repo})
	}
	repo := &p.Repos[at]
	wt, found := slices.BinarySearchFunc(repo.Worktrees, job.WorkDir, func(t runjobs.StopTarget, path string) int { return cmp.Compare(t.Path, path) })
	if !found {
		repo.Worktrees = slices.Insert(repo.Worktrees, wt, runjobs.StopTarget{Worktree: subjectOf(job), Branch: job.Branch, Path: job.WorkDir})
	}
	repo.Worktrees[wt].Jobs = append(repo.Worktrees[wt].Jobs, stop)
}

// subjectOf is how a worktree is named across repositories: its branch, else its path, then its repository's.
func subjectOf(job run.Job) string {
	name := cmp.Or(job.Branch, job.WorkDir)
	if job.Repo == "" {
		return name
	}
	return name + " (" + filepath.Base(job.Repo) + ")"
}

// Locks are none: the daemon is no repository's.
func (Plan) Locks() []domain.Lock {
	return nil
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// targets are the worktrees of the plan as a stop takes them.
func (p Plan) targets() []runjobs.StopTarget {
	var targets []runjobs.StopTarget
	for _, repo := range p.Repos {
		targets = append(targets, repo.Worktrees...)
	}
	return targets
}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: the repository it narrows to, then what stops in each worktree, a stack by its stop command, a hold on a shared service, a crashed job cleared.
func (p Plan) recap() []domain.RecapRow {
	var rows []domain.RecapRow
	if p.Repo != "" {
		rows = append(rows, domain.RecapRow{Key: "repo", Label: "Repository", Path: domain.PathRepo, Values: domain.RecapValues([]string{p.Repo})})
	}
	for _, t := range p.targets() {
		var jobs []string
		for _, job := range t.Jobs {
			jobs = append(jobs, words(job))
		}
		rows = append(rows, domain.RecapRow{Key: "jobs:" + t.Worktree, Label: "Stops in " + t.Worktree, Values: domain.RecapValues(jobs)})
	}
	return rows
}

// words is how the recap names one job's stop.
func words(job runjobs.StopJob) string {
	switch {
	case job.Fate == run.FateClear:
		return job.Name + " (crashed: nothing to stop)"
	case job.Hold == run.HoldDetached:
		return job.Name + " (a stack, by its stop command)"
	case job.Hold == run.HoldClaim:
		return job.Name + " (held)"
	case job.Shared():
		return job.Name + " (shared)"
	}
	return job.Name
}

// ----------------------------------------------------------------------------
// The outcome

// CodeAgain is the code of the follow-up that stops again the jobs that did not stop.
const CodeAgain = Name + ".again"

// next is the follow-up of a stop some worktree failed: the same request again, which stops what is left.
func (p Plan) next(items []Item) []domain.FollowUp {
	if !slices.ContainsFunc(items, func(item Item) bool { return item.Status == domain.StatusFailed }) {
		return nil
	}
	var answers []domain.Answered
	if p.Repo != "" {
		answers = []domain.Answered{{Path: domain.PathRepo, Values: []string{p.Repo}}}
	}
	return []domain.FollowUp{{Command: Name, Answers: answers, Code: CodeAgain, Message: "stop the ones left"}}
}
