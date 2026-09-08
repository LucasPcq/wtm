package seam

import (
	"context"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type SetParams struct {
	ProjectDir string
	StateDir   string
	// WorkDirs are the worktrees this run covers, as git spells them, in the
	// order they were selected. That order is what every listing and every recap
	// reads back.
	WorkDirs []string
	// Jobs are what the board lists, shared by every worktree in the set:
	// run.toml lives in the common git dir, so the worktrees of a repository
	// declare the same jobs and the same profiles.
	Jobs []domain.JobConfig
	// Declared is every job run.toml holds; see Params.Declared.
	Declared []domain.JobConfig
	// PortAddressed keys the worktrees whose .env still spells its addresses as
	// ports; see Params.PortAddressed.
	PortAddressed map[string]bool
	// ProxyPort and PublicPort are Params' own; see the fields there.
	ProxyPort   int
	PublicPort  int
	ProbeBudget time.Duration
	NoProbe     bool
}

// Set is the seam over several worktrees at once. It holds one Seam each and
// adds exactly two things: a board that shows them as one, and a start sequence
// that runs them concurrently.
type Set struct {
	seams []Seam
}

func OpenSet(params SetParams) Set {
	seams := make([]Seam, 0, len(params.WorkDirs))
	for _, workDir := range params.WorkDirs {
		seams = append(seams, Open(Params{
			ProjectDir:    params.ProjectDir,
			StateDir:      params.StateDir,
			WorkDir:       workDir,
			Jobs:          params.Jobs,
			Declared:      params.Declared,
			ProxyPort:     params.ProxyPort,
			PublicPort:    params.PublicPort,
			PortAddressed: params.PortAddressed[workDir],
			ProbeBudget:   params.ProbeBudget,
			NoProbe:       params.NoProbe,
		}))
	}
	return Set{seams: seams}
}

func (s Set) Board() runlogs.Board {
	entries := make([]runlogs.MergedEntry, 0, len(s.seams))
	for _, seam := range s.seams {
		entries = append(entries, runlogs.MergedEntry{WorkDir: seam.workDir, Board: seam.Board()})
	}
	return runlogs.NewMergedBoard(entries)
}

func (s Set) Worktrees() []string {
	names := make([]string, 0, len(s.seams))
	for _, seam := range s.seams {
		names = append(names, seam.Worktree())
	}
	return names
}

// Starter runs every worktree's sequence concurrently and reports them all to
// one Sink. Concurrently is the whole point of the isolation: each worktree has
// its own ports and its own resource names, so starting one after another would
// only make a batch slower than two terminals.
//
// A worktree that aborts does not touch the others. They are isolated by
// construction, and an isolation that propagates a failure is not one — only the
// exit code the command owes its caller reads the set as a whole.
func (s Set) Starter(params StartParams) runlogs.StartFunc {
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
		if len(s.seams) == 1 {
			return s.seams[0].Starter(params)(ctx, sink)
		}

		serialized := &lockedSink{sink: sink}
		outcomes := make(runlogs.Outcomes, len(s.seams))
		errs := make([]error, len(s.seams))

		var wg sync.WaitGroup
		for index, seam := range s.seams {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outcomes[index], errs[index] = seam.run(ctx, serialized, params)
			}()
		}
		wg.Wait()

		return outcomes, firstError(errs)
	}
}

func firstError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// lockedSink upholds runlogs' contract that a Sink is emitted to from one
// goroutine at a time. N sequences report to the surface that opened them, and
// a surface has no reason to learn how many wrote to it.
type lockedSink struct {
	mu   sync.Mutex
	sink runlogs.Sink
}

func (l *lockedSink) Emit(event runlogs.Event) {
	if l.sink == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sink.Emit(event)
}

type PruneParams struct {
	// Jobs are the ones this run starts. Their logs are the run's own and are
	// opened fresh by the daemon anyway.
	Jobs []domain.JobConfig
	// Running is the daemon's whole index. A job up in one of these worktrees
	// keeps its log whatever this run starts: its sink is writing to that file.
	Running []domain.JobInfo
}

// PruneLogs makes each worktree's log directory hold this run and nothing else.
// It is `run up`'s to call and not the runner's: `run start` adds a job to a
// session rather than defining one, and clearing the directory under it would
// throw away the logs of everything already standing beside it.
func (s Set) PruneLogs(params PruneParams) {
	keep := make(map[string]bool, len(params.Jobs))
	for _, job := range params.Jobs {
		keep[job.Name] = true
	}
	for _, seam := range s.seams {
		s.pruneOne(seam, keep, params.Running)
	}
}

func (s Set) pruneOne(worktree Seam, starting map[string]bool, running []domain.JobInfo) {
	keep := make(map[string]bool, len(starting)+len(running))
	for name := range starting {
		keep[name] = true
	}
	for _, info := range running {
		if info.WorkDir == worktree.workDir && rules.IsJobUp(info.Status) {
			keep[info.Name] = true
		}
	}
	process.PruneJobLogs(process.PruneLogsParams{LogDir: worktree.logDir, Keep: keep})
}
