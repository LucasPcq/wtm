package runlogs

import (
	"fmt"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type BoardParams struct {
	Service Service
	// Jobs are the worktree's declared jobs, in the order a surface lists them.
	Jobs    []domain.JobConfig
	WorkDir string
	// Worktree is WorkDir's branch, what a merged board shows above this
	// worktree's rows.
	Worktree string
	LogDir   string
	// SharedLogDir is the main checkout's, where a shared service persists its
	// output. Empty leaves every job reading LogDir, as before sharing existed.
	SharedLogDir string
	// Addresses is where each declared job answers in this worktree, keyed by
	// job name. Computed by the surface, which is the side that reads the
	// worktree's offset and the proxy's port.
	Addresses map[string]domain.JobAddress
	// Logged names the jobs that left output in this worktree. With the daemon's
	// index it decides which declared jobs the board lists at all: a project
	// declaring fifteen of them listed all fifteen, and opening any of the twelve
	// that had never run here answered "No output recorded for this job."
	Logged map[string]bool
}

// NewBoard builds the surface's view of a worktree's jobs. It reads nothing
// until Refresh: a surface opens before the daemon is asked anything, and a job
// nobody has started yet is still a job to show.
func NewBoard(params BoardParams) Board {
	return &board{
		service:      params.Service,
		jobs:         params.Jobs,
		workDir:      params.WorkDir,
		worktree:     params.Worktree,
		logDir:       params.LogDir,
		sharedLogDir: params.SharedLogDir,
		addresses:    params.Addresses,
		logged:       params.Logged,
	}
}

type board struct {
	service  Service
	jobs     []domain.JobConfig
	workDir  string
	worktree string
	logDir   string
	// sharedLogDir is the main checkout's, where a shared service persists its
	// output. A claim carries no log of its own, so reading a shared job's tail
	// from this worktree's directory found an empty file.
	sharedLogDir string
	addresses    map[string]domain.JobAddress
	logged       map[string]bool

	// mu guards live, which a surface refreshes off the goroutine that renders it.
	mu        sync.RWMutex
	live      map[string]domain.JobInfo
	liveOrder []string
}

func (b *board) Refresh() error {
	infos, err := b.service.List(b.workDir)
	if err != nil {
		return err
	}

	live := make(map[string]domain.JobInfo, len(infos))
	order := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.WorkDir != b.workDir {
			continue
		}
		if _, known := live[info.Name]; !known {
			order = append(order, info.Name)
		}
		live[info.Name] = info
	}

	b.mu.Lock()
	b.live = live
	b.liveOrder = order
	b.mu.Unlock()
	return nil
}

func (b *board) Jobs() []JobView {
	b.mu.RLock()
	live, order := b.live, b.liveOrder
	b.mu.RUnlock()

	// A job that neither lives nor left a trace here has nothing to show and no
	// stream to bind: listing it only ever led the reader to an empty pane.
	visible, _ := rules.VisibleJobs(rules.VisibleJobsParams{
		Jobs: b.jobs, Up: live, Traces: b.logged,
	})

	declared := make(map[string]bool, len(b.jobs))
	views := make([]JobView, 0, len(visible))
	for _, job := range b.jobs {
		declared[job.Name] = true
	}
	for _, job := range visible {
		views = append(views, b.own(declaredView(declaredViewParams{
			Job:     job.Job,
			Info:    live[job.Job.Name],
			Address: b.addresses[job.Job.Name],
		})))
	}

	// A job the daemon holds but run.toml no longer declares is still running:
	// hiding it would leave no way to read or stop it from here.
	for _, name := range order {
		if declared[name] {
			continue
		}
		views = append(views, b.own(undeclaredView(live[name])))
	}
	return views
}

// own stamps a view with the worktree it came from, which is what lets a merged
// board tell two jobs of the same name apart.
func (b *board) own(view JobView) JobView {
	view.WorkDir = b.workDir
	view.Worktree = b.worktree
	return view
}

func (b *board) Attach(params AttachParams) (Stream, error) {
	view, found := b.view(params.Job)
	if !found {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
	}
	if !view.Attachable {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotAttachable, params.Job)
	}
	return b.service.Attach(AttachRequest{Name: params.Job, WorkDir: b.workDir, Size: params.Size})
}

func (b *board) History(params HistoryParams) ([]string, error) {
	lines := params.Lines
	if lines <= 0 {
		lines = domain.JobLogTailLines
	}
	return b.service.Tail(TailRequest{LogDir: b.logDirFor(params.Job), Job: params.Job, Lines: lines})
}

func (b *board) view(name string) (JobView, bool) {
	for _, view := range b.Jobs() {
		if view.Name == name {
			return view, true
		}
	}
	return JobView{}, false
}

type declaredViewParams struct {
	Job     domain.JobConfig
	Info    domain.JobInfo
	Address domain.JobAddress
}

func declaredView(params declaredViewParams) JobView {
	view := JobView{
		Name:    params.Job.Name,
		Kind:    params.Job.Kind,
		Status:  domain.JobStatusStopped,
		Address: params.Address,
	}
	if params.Info.Name != "" {
		view.Status = params.Info.Status
		view.StartedAt = params.Info.StartedAt
		view.ExitCode = params.Info.ExitCode
	}
	// A claim on a shared service is attachable too: it owns no stream of its
	// own, and the daemon resolves it to the one there is. Refusing it here left
	// every worktree but the main checkout unable to read a shared job's output.
	up := view.Status == domain.JobStatusRunning || view.Status == domain.JobStatusAttached
	view.Attachable = up && !rules.IsDetached(params.Job)
	return view
}

// undeclaredView describes a job whose config is gone. Nothing says whether the
// service was a detached launcher, so the stream is offered and the daemon has
// the last word on refusing it.
func undeclaredView(info domain.JobInfo) JobView {
	return JobView{
		Name:       info.Name,
		Kind:       info.Kind,
		Status:     info.Status,
		StartedAt:  info.StartedAt,
		ExitCode:   info.ExitCode,
		Attachable: info.Status == domain.JobStatusRunning || info.Status == domain.JobStatusAttached,
	}
}

// logDirFor is where a job's persisted output lives: the main checkout's for a
// shared service, this worktree's for everything else.
func (b *board) logDirFor(name string) string {
	if b.sharedLogDir == "" {
		return b.logDir
	}
	for _, job := range b.jobs {
		if job.Name == name && rules.IsShared(job) {
			return b.sharedLogDir
		}
	}
	return b.logDir
}
