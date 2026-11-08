// Package run is what wtm's daemon says of the jobs it runs.
package run

import (
	"errors"
	"path/filepath"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// A Job is one job the daemon knows: its name, the worktree it runs in, the profile it was started under (empty for one started alone), whether it is up (running, detached or joined), its kind, its State (empty for a claim, which runs no process), what of it the daemon holds (Hold), whether the daemon Reaped it (an orphan stopped as it started), the PID of a process of the daemon's own, its url, the ports it binds, the code its process exited with (-1 when a signal killed it), when the daemon started it (zero for a job taken over that ended before), when it ended (zero while it is up, or when that is not known), the namespace a shared job holds for its worktree (on a claim, and on the main checkout's own job when it holds it; empty when it makes none), the main checkout a shared job runs in, whether that checkout itself holds the shared job (MainHolds, on the job itself, never on a claim) and whether that cannot be told (MainHoldsUnknown: a daemon of another build listed it, and the index has no record of it), the main checkout of the repository it was started for (Repo, empty when its start named none) and the branch of its worktree there (Branch, the main checkout's for a shared job's own run), the command a stack stops with as it was started (StopCommand), and the last line it printed (nil before any).
type Job struct {
	Name             string
	WorkDir          string
	Profile          string
	Up               bool
	Kind             project.JobKind
	State            State
	Hold             Hold
	Reaped           bool
	PID              int
	URL              string
	Ports            map[string]int
	ExitCode         *int
	StartedAt        time.Time
	EndedAt          time.Time
	Namespace        string
	SharedDir        string
	MainHolds        bool
	MainHoldsUnknown bool
	Repo             string
	Branch           string
	StopCommand      string
	LastLine         *Line
}

// Hold is what of a job the daemon holds.
type Hold string

// The holds of a job.
const (
	HoldOwn      Hold = "own"      // a process of the daemon's, which it drains, a task or a launcher included
	HoldDetached Hold = "detached" // a stack its launcher left running (a service with a stop command)
	HoldClaim    Hold = "claim"    // a worktree's hold on a shared job another worktree runs
)

// Holds returns every Hold.
func Holds() []Hold {
	return []Hold{HoldOwn, HoldDetached, HoldClaim}
}

// State is a job's state, as wtm's events report it.
type State string

// The states of a job.
const (
	Starting State = "starting"
	Running  State = "running"
	Crashed  State = "crashed"
	Exited   State = "exited" // a task that ran to its end
	Stopped  State = "stopped"
	// NotStarted is a job run.toml declares that the daemon does not list and whose log holds nothing: it never ran, never a state the daemon gives.
	NotStarted State = "not_started"
)

// StandingParams are what tells where a job run.toml declares stands in a worktree: the jobs the daemon lists, the worktree's directory, the job's name, and run.toml (whose runners start some of its jobs).
type StandingParams struct {
	Jobs     []Job
	Dir      string
	Name     string
	Declared project.Run
}

// A Standing is what the daemon lists of a job run.toml declares in a worktree: the job itself (Runner empty: its own, or its claim on a shared one), or a runner listed there that starts it, its runners' own included (Runner its name, Job the runner).
type Standing struct {
	Job    Job
	Runner string
}

// StandingIn is what the daemon lists of the job run.toml declares under params.Name in the worktree at params.Dir: the job itself, else a runner listed there that starts it, one up before one that is not; false when it lists neither, the job's log then telling whether it ran (Unlisted).
func StandingIn(params StandingParams) (Standing, bool) {
	var runner *Job
	for i, job := range params.Jobs {
		if job.WorkDir != params.Dir {
			continue
		}
		if job.Name == params.Name {
			return Standing{Job: job}, true
		}
		if (runner == nil || !runner.Up && job.Up) && params.runs(job.Name) {
			runner = &params.Jobs[i]
		}
	}
	if runner == nil {
		return Standing{}, false
	}
	return Standing{Job: *runner, Runner: runner.Name}, true
}

// runs reports whether run.toml's job called runner starts the job of params, itself or through a runner it starts.
func (params StandingParams) runs(runner string) bool {
	at := slices.IndexFunc(params.Declared.Jobs, func(job project.Job) bool { return job.Name == runner })
	return at >= 0 && slices.ContainsFunc(params.Declared.Children(params.Declared.Jobs[at]), func(child project.Job) bool { return child.Name == params.Name })
}

// StartedUnder is the profile the jobs the worktree at dir runs for itself were started under, up or ended, "" when none says one: a worktree whose every job crashed keeps it, where ProfileIn, which counts the jobs up alone, says none.
func StartedUnder(jobs []Job, dir string) string {
	for _, job := range jobs {
		if job.WorkDir == dir && Own(job) && job.Profile != "" {
			return job.Profile
		}
	}
	return ""
}

// Unlisted is the state of a job run.toml declares that the daemon does not list: stopped once its log holds a line, so it ran; else NotStarted.
func Unlisted(logged bool) State {
	if logged {
		return Stopped
	}
	return NotStarted
}

// States returns every State.
func States() []State {
	return []State{Starting, Running, Crashed, Exited, Stopped, NotStarted}
}

// ErrUnknown says whether a job is up cannot be told: no daemon answers, yet its index holds jobs it would run.
var ErrUnknown = errors.New("wtm's daemon is not running: which jobs are up is unknown")

// UpIn returns the names of the jobs up in the worktree at dir, sorted, empty and not nil when none is.
func UpIn(jobs []Job, dir string) []string {
	names := []string{}
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir {
			names = append(names, job.Name)
		}
	}
	slices.Sort(names)

	return names
}

// OwnUpIn returns the names of the jobs up the worktree at dir runs for itself (Own), sorted, empty and not nil when none is.
func OwnUpIn(jobs []Job, dir string) []string {
	names := []string{}
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir && Own(job) {
			names = append(names, job.Name)
		}
	}
	slices.Sort(names)

	return names
}

// LogDir is where the jobs of the worktree of branch keep their logs under wtm's state stateDir, a log per job; none for a worktree on no branch.
func LogDir(stateDir, branch string) string {
	if branch == "" || stateDir == "" {
		return ""
	}
	return filepath.Join(stateDir, "logs", worktree.StateSegment(branch))
}

// Own reports whether a worktree runs job for itself: neither its claim on a shared job another worktree runs, nor a shared job it runs for every worktree.
func Own(job Job) bool {
	return job.Hold != HoldClaim && job.SharedDir == ""
}

// ProfileIn returns the profile the jobs up the worktree at dir runs for itself were started under, "" when none says one.
func ProfileIn(jobs []Job, dir string) string {
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir && Own(job) && job.Profile != "" {
			return job.Profile
		}
	}
	return ""
}

// URLsIn returns the URLs of the jobs up in the worktree at dir that have one, sorted, empty and not nil when none has.
func URLsIn(jobs []Job, dir string) []string {
	urls := []string{}
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir && job.URL != "" {
			urls = append(urls, job.URL)
		}
	}
	slices.Sort(urls)

	return urls
}

// UnknownMessage says why the jobs up are unknown, err being why the daemon could not tell.
func UnknownMessage(err error) string {
	if errors.Is(err, ErrUnknown) {
		return ErrUnknown.Error()
	}

	return "couldn't ask wtm's daemon which jobs are up: " + err.Error()
}
