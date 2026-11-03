// Package runlogs reads what the jobs of some worktrees printed: each job's last lines, from its log or the run daemon, and, followed, each new one as it prints it; a read, it changes nothing and never starts the run daemon.
package runlogs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandRunLogs

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase reads the jobs' output of one repository: the ports it reads it through, where wtm keeps the repository's state (the jobs' logs under it), and the service that reads the repository.
type UseCase struct {
	Git        Git
	Daemon     Daemon
	ReadLog    ReadLog
	ScanLog    ScanLog
	Meta       Meta
	EnvFiles   EnvFiles
	StateDir   string
	Repository repository.Service
}

// An Outcome is what Run read: one Item per worktree, and the warnings of what could not be asked.
type Outcome = domain.Read[Detail]

// An Item is one worktree and the jobs read there.
type Item = domain.Unit[Detail]

// An Entry is one line a job of a worktree printed, as Watch streams it, numbered in the whole log when it was read whole.
type Entry struct {
	Worktree string    `json:"worktree"`
	Job      string    `json:"job"`
	At       time.Time `json:"at,omitzero"`
	Text     string    `json:"text"`
	N        int       `json:"n,omitempty"`
}

// An Absent is what Watch has no line of: a worktree where no job ran (Job empty), or the job a request named, which never ran there.
type Absent struct {
	Worktree string `json:"worktree"`
	Job      string `json:"job,omitempty"`
}

// Out is where Watch streams: Emit receives each line, its error ending the stream with it; Absent each worktree, or job named, it has no line of, as the read says it; Warn what kept lines away (the daemon's skew, a log that cannot be read, lines dropped by a follow that fell behind).
type Out struct {
	Emit   func(Entry) error
	Absent func(Absent) error
	Warn   func(domain.Warning)
}

// ----------------------------------------------------------------------------
// Run

// Run reads the last lines of the jobs of the worktrees req names, or of the one it stands in: the job req names, else each job the daemon lists there or that left a log; with a search or a time, what of each whole log matches. A read, it takes no lock, runs no operation and never starts the run daemon. An error returned is a refusal of the request or of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository and the jobs the daemon knows, and settle the worktrees read.
	facts, req, targets, err := u.resolved(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Read each job's log, and the project of a compose stack, whose lines are Docker's.
	sources := Sources(facts, req, targets)
	read := Logged{Found: map[int]Found{}, Unreadable: map[int]error{}, Projects: u.projects(facts, sources)}
	for i, s := range sources {
		read.Found[i], read.Unreadable[i] = u.found(ctx, req, s)
	}

	// 3. Report each worktree.
	items, warnings := Report(facts, targets, sources, read)
	return Outcome{Items: items, Warnings: warnings, Interrupted: ctx.Err() != nil}, nil
}

// ----------------------------------------------------------------------------
// Watch

// The codes of what kept a stream from lines: a follow that fell behind and lost some, a follow the daemon broke off.
const (
	CodeLinesDropped = Name + ".lines_dropped"
	CodeFollowBroken = Name + ".follow_broken"
)

// Watch streams to out the last lines of the jobs Run reads, a job after the other, and each worktree or job named it has none of, then, when req.Follow, each new line of each job running as it prints it (those a search matches; a running job's past lines are then the last ones the daemon keeps), until every one ended or ctx is done. It returns nil once ctx is done after the request was settled; else the refusal, or the error of Emit or Absent that ended it.
func (u UseCase) Watch(ctx context.Context, req Request, out Out) error {
	facts, req, targets, err := u.resolved(ctx, req)
	if err != nil {
		return err
	}
	for _, warning := range facts.Asked.Warnings(Name) {
		out.Warn(warning)
	}

	s := newStream(ctx, out)
	defer s.cancel(nil)
	match, _ := MatcherOf(req)
	var following sync.WaitGroup
	sources := Sources(facts, req, targets)
	for _, t := range targets {
		shown := false
		for _, src := range sources {
			if src.Target.Path != t.Path {
				continue
			}
			if req.Follow && src.Follows(facts) {
				shown = true
				following.Go(func() { u.follow(s, src, match) })
				continue
			}
			found, err := u.found(s.ctx, req, src)
			logged := found.Logged || err != nil
			if !src.Shows(logged) {
				continue
			}
			shown = true
			if err != nil {
				s.warn(unreadable(src, err))
			}
			if !src.Ran(logged) {
				s.absent(Absent{Worktree: t.Worktree, Job: src.Name})
				continue
			}
			s.emit(src, found.Lines...)
		}
		// said where the read says it: in the worktree's place
		if !shown {
			s.absent(Absent{Worktree: t.Worktree})
		}
	}
	following.Wait()

	return s.end(ctx)
}

// follow streams src's last lines then each new one until its job ends: a follower the daemon let go as it fell behind follows again, the lines it already sent dropped (a gap warned); one the daemon refuses before any line falls back on the log.
func (u UseCase) follow(s *stream, src Source, match Matcher) {
	tail := src.Lines
	tail.Follow = true
	var last *run.Line
	sent := 0 // the lines sent with last's time
	for {
		var resume *Resume
		if last != nil {
			resume = &Resume{Last: *last, Sent: sent}
		}
		_, err := u.Daemon.Lines(s.ctx, tail, func(line run.Line) {
			if resume != nil && resume.Drops(line) {
				return
			}
			if last != nil && line.At.Equal(last.At) {
				sent++
			} else {
				sent = 1
			}
			last = &line
			if match == nil || match(line.Text) {
				s.emit(src, Line{At: line.At, Text: line.Text})
			}
		})
		if resume != nil && resume.Gap {
			s.warn(domain.Warning{Code: CodeLinesDropped, Message: fmt.Sprintf("%s · %s fell behind: some of its lines were dropped", src.Target.Worktree, src.Name), Params: domain.Params{"worktree": src.Target.Worktree, "job": src.Name}})
		}

		var refused *run.Error
		switch {
		case errors.As(err, &refused) && refused.Code == run.CodeFollowerBehind:
			continue
		case err == nil || s.ctx.Err() != nil:
		case last == nil:
			lines, readErr := u.ReadLog(src.Log, run.MaxTail)
			if readErr != nil {
				s.warn(unreadable(src, readErr))
			}
			s.emit(src, LastOf(lines).Lines...)
		default:
			s.warn(domain.Warning{Code: CodeFollowBroken, Message: fmt.Sprintf("%s · %s: %v", src.Target.Worktree, src.Name, err), Params: domain.Params{"worktree": src.Target.Worktree, "job": src.Name}})
		}
		return
	}
}

// A stream is a Watch under way: its context, cancelled with the error of Emit, and its out, called by one job at a time.
type stream struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	mu     sync.Mutex
	out    Out
}

func newStream(ctx context.Context, out Out) *stream {
	s := &stream{out: out}
	s.ctx, s.cancel = context.WithCancelCause(ctx)
	return s
}

func (s *stream) emit(src Source, lines ...Line) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, line := range lines {
		if s.ctx.Err() != nil {
			return
		}
		if err := s.out.Emit(Entry{Worktree: src.Target.Worktree, Job: src.Name, At: line.At, Text: line.Text, N: line.N}); err != nil {
			s.cancel(err)
		}
	}
}

func (s *stream) absent(absent Absent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	if err := s.out.Absent(absent); err != nil {
		s.cancel(err)
	}
}

func (s *stream) warn(warning domain.Warning) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Warn(warning)
}

// end is how the stream ended: the error of Emit or Absent, nil when parent was done.
func (s *stream) end(parent context.Context) error {
	if cause := context.Cause(s.ctx); cause != nil && parent.Err() == nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is run logs' form over the repository: the worktrees it may read, the jobs run.toml declares, and its problems.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository run logs cannot read: run.toml does not exist (followed by run init), or does not load.
const (
	CodeNotInitialized = Name + ".not_initialized"
	CodeInit           = Name + ".init"
	CodeRunInvalid     = Name + ".run_invalid"
)

// commandRunInit sets the run module up in a repository: not migrated yet.
const commandRunInit = "run init"

// Observe reads what run logs reads from: the project's config and its run.toml, the repository's snapshot, which worktrees' directory is gone, where the caller stands, and the jobs the run daemon knows, never starting it. A repository with no wtm config is refused with CodeConfigNotFound, one with no run.toml with CodeNotInitialized.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}
	if repo.RunErr != nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeRunInvalid, Message: "invalid run.toml: " + repo.RunErr.Error()}
	}
	if !repo.Run.Declared {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeNotInitialized, Message: "no run.toml: no job has run", FollowUp: &domain.FollowUp{Command: commandRunInit, Code: CodeInit, Message: "declare the jobs"}}
	}

	facts := Facts{Repository: repo, Gone: map[string]bool{}, StateDir: u.StateDir}
	for _, w := range repo.Checkouts() {
		if facts.Gone[w.Path], err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, repository.Interrupted(ctx, err)
		}
	}
	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)
	facts.Asked = repository.Ask(ctx, repository.AskParams{Jobs: u.Daemon.Jobs, Restart: u.Repository.Restart}, func() {})

	return facts, ctx.Err()
}

// ----------------------------------------------------------------------------
// Helpers

// resolved is what req reads: the facts, req settled on them, and its worktrees.
func (u UseCase) resolved(ctx context.Context, req Request) (Facts, Request, []Target, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Facts{}, req, nil, err
	}
	req, targets, err := Resolve(facts, req)
	return facts, req, targets, err
}

// found is what req reads of src's log: the whole log sifted when req searches it or reads it from a time, else its last lines; and why it could not be read.
func (u UseCase) found(ctx context.Context, req Request, src Source) (Found, error) {
	if !req.Whole() || src.Log.Dir == "" {
		lines, err := u.last(ctx, src)
		return LastOf(lines), err
	}
	sift := NewSift(req)
	err := u.ScanLog(src.Log, func(line run.Line) bool {
		sift.Take(line)
		return ctx.Err() == nil
	})
	return sift.Found(), err
}

// projects are the compose projects of the worktrees a compose stack of sources runs in, by path: COMPOSE_PROJECT_NAME of their environment, none under the verbatim isolation.
func (u UseCase) projects(f Facts, sources []Source) map[string]string {
	projects := map[string]string{}
	for _, s := range sources {
		if !s.Compose() {
			continue
		}
		home := s.Home
		record, _ := u.Meta.Load(home.Branch)
		jobEnv, err := env.ReadHookEnv(env.JobParams{Branch: home.Branch, ProjectDir: f.MainPath, Ordinal: record.Ordinal, Isolation: record.Isolation.Effective(), Run: f.Run}, home.Path, u.EnvFiles.Read)
		if err == nil {
			projects[home.Path] = jobEnv[env.ComposeProjectKey]
		}
	}
	return projects
}

// last are src's last lines: its log's, else, for a job that keeps none, the daemon's; and why its log could not be read.
func (u UseCase) last(ctx context.Context, src Source) ([]run.Line, error) {
	if src.Log.Dir != "" {
		return u.ReadLog(src.Log, run.MaxTail)
	}
	if !src.Listed {
		return nil, nil
	}
	var lines []run.Line
	_, _ = u.Daemon.Lines(ctx, src.Lines, func(line run.Line) { lines = append(lines, line) })
	return lines, nil
}

// CodeLogUnreadable warns of a job's log that exists and cannot be read.
const CodeLogUnreadable = Name + ".log_unreadable"

func unreadable(src Source, err error) domain.Warning {
	return domain.Warning{Code: CodeLogUnreadable, Message: fmt.Sprintf("%s · %s: its log cannot be read: %v", src.Target.Worktree, src.Name, err), Params: domain.Params{"worktree": src.Target.Worktree, "job": src.Name, "cause": err.Error()}}
}
