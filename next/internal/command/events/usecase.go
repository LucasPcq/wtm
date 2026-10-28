// Package events streams the changes of a repository's worktrees, or of every repository wtm knows, as they happen: a snapshot, ready, then each change another process publishes on the event bus and each one a poll finds made with no operation of wtm (a commit from a terminal, a job started). A stream, it changes nothing and never ends on its own.
package events

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/registry"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "events"

// CodeConfigNotFound refuses a repository with no wtm config, and ends the stream of one that loses it.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// CodeSchemaNewer ends the stream on a line of a version newer than this wtm reads.
const CodeSchemaNewer = Name + ".schema_newer"

// recheckEvery is how often a stream polls what it watches, when the use case says nothing: the legacy dashboard polled its jobs every 3 seconds and git every 20.
const recheckEvery = 5 * time.Second

// dirtyEvery is how many polls apart a poll reads whether each worktree is dirty, one at a time: a git status per worktree cost 2.3 CPU-seconds a poll on 20 worktrees of 4,600 files read 8 at once, 1 one at a time, and the legacy dashboard read git every 20 seconds.
const dirtyEvery = 4

// A UseCase streams the events of the repositories wtm knows: the bus it subscribes to, the registry of those repositories, how it opens one, the jobs of the run daemon, its clock, and how often it polls what it watches (Recheck, 5 seconds when zero): whether it is still there, and what changed in it with no operation of wtm. Jobs may be nil: the jobs are then never polled.
type UseCase struct {
	Bus      Bus
	Registry registry.Service
	Open     Open
	Jobs     Jobs
	Now      func() time.Time
	Recheck  time.Duration
}

// A Source is one repository a stream watches: how the events name it, the service that reads its worktrees, their meta records, and its git as a poll reads it.
type Source struct {
	Repo       domain.Repo
	Repository repository.Service
	Meta       Meta
	Git        Git
}

// A Received is one line of the stream: decoded, and as the JSON a reader receives, and whether the stream watches every repository, a person then told which one each line is about. A line relayed from the bus keeps the bytes it was published with, so that a newer publisher's fields reach the reader; its Event is nil when this wtm does not know its type.
type Received struct {
	Line  domain.Line
	JSON  json.RawMessage
	Every bool
}

// Out is where a stream goes: Emit receives each line in order, an error it returns ending the stream with it; Warn receives what kept the stream from a line or a repository for a while, nil dropping it.
type Out struct {
	Emit func(Received) error
	Warn func(error)
}

// ----------------------------------------------------------------------------
// Watch

// Watch streams the events req asks for to out until ctx is done: a snapshot of each repository watched, ready, then every change published on the bus, and every Recheck what a poll found changed with no operation of wtm: a worktree.changed per worktree whose head, checkout, jobs or dirtiness moved, a fresh snapshot when a worktree went away. When the bus goes away it subscribes again, after a backoff, and opens on fresh snapshots: nothing is replayed, so a reader takes every event as an upsert keyed by branch and every snapshot as a reset.
// It returns nil once ctx is done after the stream started, ctx's error when it is done before; otherwise the refusal of req, or what ended the stream: a line of a newer version, a repository watched that lost its wtm config, or Emit's error.
func (u UseCase) Watch(ctx context.Context, req Request, out Out) error {
	// 1. Settle what the stream watches, refusing what no retry can change.
	source, every, err := u.scope(ctx, req)
	if err != nil {
		return err
	}

	// 2. Stream, subscribing again whenever the bus goes away.
	s := &stream{UseCase: u, out: out, every: every}
	var backoff Backoff
	for {
		if every {
			err = s.everyRepository(ctx)
		} else {
			err = s.oneRepository(ctx, source)
		}

		if ending, ok := errors.AsType[ended](err); ok && ctx.Err() == nil {
			return ending.err
		}
		if err != nil && ctx.Err() == nil {
			s.warn(err)
		}
		if ctx.Err() != nil || !sleep(ctx, backoff.Next(s.ready)) {
			return s.interrupted(ctx)
		}
		s.ready = false
	}
}

// scope returns the repository req watches, or reports that it watches every one: --all, or no --repo from nowhere or from an unnamed directory in no repository. A relative --repo is read from where the caller stands; from nowhere it names no directory.
func (u UseCase) scope(ctx context.Context, req Request) (Source, bool, error) {
	if err := req.Validate(); err != nil {
		return Source{}, false, err
	}
	if req.All || req.Repo == "" && req.Caller.Dir == "" {
		return Source{}, true, nil
	}

	repo := req.Repo
	if repo != "" && !filepath.IsAbs(repo) {
		if req.Caller.Cwd == "" {
			return Source{}, false, relativeFromNowhere(req.Repo)
		}
		repo = filepath.Join(req.Caller.Cwd, repo)
	}

	source, err := u.Open(ctx, cmp.Or(repo, req.Caller.Dir))
	switch {
	case req.Repo != "" && errors.Is(err, fs.ErrNotExist):
		return Source{}, false, notADirectory(req.Repo)
	case req.Repo == "" && !req.Caller.Named && notARepository(err):
		return Source{}, true, nil
	case err != nil:
		return Source{}, false, err
	}

	// A repository with no wtm config is refused before the stream starts, as it ends one that loses it; one that has it joins those wtm knows.
	if _, err := source.Repository.Observe(ctx, Name); err != nil {
		return Source{}, false, err
	}
	u.Registry.Register(ctx, source.Repo)
	return source, false, nil
}

// ----------------------------------------------------------------------------
// The stream

// A stream is one Watch: what it watches with, where it goes, whether it sent a line yet and whether its attempt reached ready.
type stream struct {
	UseCase
	out     Out
	every   bool
	started bool
	ready   bool
}

// ended is the error that ends the stream rather than an attempt.
type ended struct{ err error }

func (e ended) Error() string { return e.err.Error() }
func (e ended) Unwrap() error { return e.err }

// oneRepository is one attempt at the stream of source: subscribe first, so that a change made while the snapshot is read arrives after ready rather than falling between the two, then the snapshot, ready and every change of the repository.
func (s *stream) oneRepository(ctx context.Context, source Source) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lines, err := s.Bus.Subscribe(ctx, []string{source.Repo.CommonDir})
	if err != nil {
		return err
	}

	watched := &watch{source: source}
	if err := s.snapshot(ctx, watched); err != nil {
		return lost(err)
	}
	if err := s.emitReady(); err != nil {
		return err
	}

	return s.relay(ctx, relayParams{
		Lines: lines,
		Recheck: func(ctx context.Context) error {
			// only a lost config is worth the full read that names it; any other failure waits for the next poll
			if _, _, err := source.Repository.Config.Load(); errors.Is(err, project.ErrConfigNotFound) {
				return s.stillThere(ctx, source)
			}
			jobs, known := s.jobs(ctx)
			err := s.poll(ctx, pollParams{Watch: watched, Jobs: jobs, Known: known})
			if _, ending := errors.AsType[ended](err); ending {
				return err
			}
			return nil
		},
		Route: func(ctx context.Context, received Received) error {
			// The bus filters by repository already: a line of another one is a newer daemon's mistake.
			if received.Line.Repo == nil || received.Line.Repo.CommonDir != source.Repo.CommonDir {
				return nil
			}
			if err := s.emit(received); err != nil {
				return err
			}
			if _, removed := received.Line.Event.(domain.RepoRemoved); removed {
				return s.stillThere(ctx, source)
			}
			return s.announced(ctx, watched, received.Line)
		},
	})
}

// everyRepository is one attempt at the stream of every repository wtm knows: subscribe first, then a snapshot of each repository still initialized, ready, and every change of a repository the stream sent a snapshot of; a repository joining is sent with its snapshot, once.
func (s *stream) everyRepository(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lines, err := s.Bus.Subscribe(ctx, nil)
	if err != nil {
		return err
	}

	repos, err := s.Registry.Prune(ctx)
	if err != nil {
		return err
	}
	watched := map[string]*watch{}
	for _, repo := range repos {
		if err := s.join(ctx, joinParams{Repo: repo, Watched: watched}); err != nil {
			return err
		}
	}
	if err := s.emitReady(); err != nil {
		return err
	}

	return s.relay(ctx, relayParams{
		Lines: lines,
		// A repository that goes away is announced by the prune, and its repo.removed reaches this stream like any line; one the poll cannot read waits for it.
		Recheck: func(ctx context.Context) error {
			_, _ = s.Registry.Prune(ctx)
			jobs, known := s.jobs(ctx)
			for _, commonDir := range slices.Sorted(maps.Keys(watched)) {
				err := s.poll(ctx, pollParams{Watch: watched[commonDir], Jobs: jobs, Known: known})
				if _, ending := errors.AsType[ended](err); ending {
					return err
				}
			}
			return nil
		},
		Route: func(ctx context.Context, received Received) error {
			repo := received.Line.Repo
			if repo == nil {
				return nil
			}

			// a repository joining is sent once, with its snapshot; a line of a repository with no snapshot sent is dropped
			_, added := received.Line.Event.(domain.RepoAdded)
			_, removed := received.Line.Event.(domain.RepoRemoved)
			switch {
			// the bus is shared whatever the state: a repository another process announces would be read with this one's moved state
			case added && s.Registry.Inert():
				return nil
			case added && watched[repo.CommonDir] == nil:
				if err := s.emit(received); err != nil {
					return err
				}
				return s.join(ctx, joinParams{Repo: *repo, Watched: watched})
			case added || watched[repo.CommonDir] == nil:
				return nil
			case removed:
				delete(watched, repo.CommonDir)
				return s.emit(received)
			}

			if err := s.emit(received); err != nil {
				return err
			}
			return s.announced(ctx, watched[repo.CommonDir], received.Line)
		},
	})
}

type joinParams struct {
	Repo    domain.Repo
	Watched map[string]*watch
}

// A watch is a repository the stream sent a snapshot of: how it reads it, what its last poll read, how many polls read it, the operations of wtm the bus says run on it, by their op id, with when each started, and the branches their events named, whose changes are theirs.
type watch struct {
	source     Source
	polled     Polled
	polls      int
	operations map[string]time.Time
	named      map[string]bool
}

// operationLimit is how long an operation the bus said started holds the poll back without saying it finished: its process may have died.
const operationLimit = 10 * time.Minute

// busy reports whether an operation of wtm runs on the repository watched at now: what it changes is its own events', never the poll's.
func (w *watch) busy(now time.Time) bool {
	for id, started := range w.operations {
		if now.Sub(started) < operationLimit {
			return true
		}
		delete(w.operations, id)
	}
	return false
}

// join sends the snapshot of params.Repo and counts it among those watched. A repository it cannot read is skipped with a warning rather than ending the stream: one broken repository must not blind the reader to every other.
func (s *stream) join(ctx context.Context, params joinParams) error {
	source, err := s.Open(ctx, params.Repo.Root)
	watched := &watch{source: source}
	if err == nil {
		err = s.snapshot(ctx, watched)
	}
	if ending, ok := errors.AsType[ended](err); ok {
		return ending
	}
	if err != nil {
		s.warn(fmt.Errorf("%s: %w", params.Repo.Root, err))
		return nil
	}

	params.Watched[source.Repo.CommonDir] = watched
	return nil
}

// snapshot sends every worktree on a branch of the repository watched, and keeps what it read for the next poll.
func (s *stream) snapshot(ctx context.Context, watched *watch) error {
	facts, err := s.observe(ctx, watched.source)
	if err != nil {
		return err
	}

	jobs, known := s.jobs(ctx)
	reads := repository.Each(facts.Worktrees, func(w worktree.Worktree) *bool { return dirtyOf(ctx, watched.source.Git, w) })
	watched.polled = PollOf(facts.Snapshot, jobs, known, byPath(facts.Worktrees, reads))
	return s.emitLine(domain.Line{Repo: &watched.source.Repo, Event: domain.Snapshot{Worktrees: Identities(facts)}})
}

type pollParams struct {
	Watch *watch
	Jobs  []run.Job
	Known bool
	Dirty bool
}

// poll reads the worktrees and branch tips of the repository watched again, and whether each worktree is dirty every dirtyEvery polls, never while an operation of wtm runs on it, and sends what changed since the last read. It returns the error that kept it from reading, or the reader's.
func (s *stream) poll(ctx context.Context, params pollParams) error {
	if params.Watch.busy(s.Now()) {
		return nil
	}
	params.Watch.polls++
	params.Dirty = params.Watch.polls%dirtyEvery == 0
	now, err := s.read(ctx, params)
	if err != nil {
		return err
	}
	return s.send(ctx, params.Watch, now)
}

// send sends what changed in the repository watched from its last read to now, but for the changes of the branches the operations of wtm named: a worktree.changed per worktree that moved, named as a full read names it, a fresh snapshot when one went away. Now is its last read once sent, and the branches named are forgotten; a read that fails keeps both for the next poll.
func (s *stream) send(ctx context.Context, watched *watch, now Polled) error {
	changes, gone := Changes(watched.polled, now)
	// no event of an operation reports a removal: a worktree gone was removed by someone else, whoever named it
	changes = slices.DeleteFunc(changes, func(change Change) bool { return watched.named[change.Branch] })
	if len(changes) == 0 && len(gone) == 0 {
		watched.polled, watched.named = now, nil
		return nil
	}

	facts, err := s.observe(ctx, watched.source)
	if err != nil {
		return err
	}
	repo := &watched.source.Repo
	identities := Identities(facts)
	if len(gone) > 0 {
		changes = nil
		if err := s.emitLine(domain.Line{Repo: repo, Event: domain.Snapshot{Worktrees: identities}}); err != nil {
			return err
		}
	}
	for _, change := range changes {
		i := slices.IndexFunc(identities, func(identity worktree.Identity) bool { return identity.Branch == change.Branch })
		if i < 0 {
			continue
		}
		if err := s.emitLine(domain.Line{Repo: repo, Event: domain.WorktreeChanged{Worktree: identities[i], Changed: change.Fields}}); err != nil {
			return err
		}
	}
	watched.polled, watched.named = now, nil
	return nil
}

// read is what a poll reads of the repository params watches, whether each worktree is dirty when params asks, one at a time, the jobs and the dirtiness it does not know taken from its last read.
func (s *stream) read(ctx context.Context, params pollParams) (Polled, error) {
	git := params.Watch.source.Git
	worktrees, err := git.Worktrees(ctx)
	if err != nil {
		return nil, err
	}
	branches, err := git.Branches(ctx)
	if err != nil {
		return nil, err
	}

	var reads []*bool
	if params.Dirty {
		for _, w := range worktrees {
			reads = append(reads, dirtyOf(ctx, git, w))
		}
	}

	return Carried(params.Watch.polled, PollOf(worktree.Snapshot{Worktrees: worktrees, Branches: branches}, params.Jobs, params.Known, byPath(worktrees, reads))), nil
}

// dirtyOf is whether the worktree w holds uncommitted changes, a missing one holding none; nil when it is on no branch or git could not tell.
func dirtyOf(ctx context.Context, git Git, w worktree.Worktree) *bool {
	if w.Branch() == "" {
		return nil
	}
	presence, err := repository.Present(ctx, git, w)
	if err != nil {
		return nil
	}
	return new(presence.Dirty)
}

// byPath is each of reads, read of the worktree at its index, by the worktree's path, those unread left out.
func byPath(worktrees []worktree.Worktree, reads []*bool) map[string]bool {
	dirty := map[string]bool{}
	for i, read := range reads {
		if read != nil {
			dirty[worktrees[i].Path] = *read
		}
	}
	return dirty
}

// announced follows a line the bus carried for the repository watched: an operation of wtm holds the poll back from its start to its finish, the branches its events name are its own, and once the last one running finished, what the others changed meanwhile is sent.
func (s *stream) announced(ctx context.Context, watched *watch, line domain.Line) error {
	_, started := line.Event.(domain.OperationStarted)
	_, finished := line.Event.(domain.OperationFinished)
	switch {
	case started:
		if watched.operations == nil {
			watched.operations = map[string]time.Time{}
		}
		watched.operations[line.OpID] = s.Now()
	case finished:
		delete(watched.operations, line.OpID)
	}
	if watched.busy(s.Now()) {
		if branch := namedBy(line.Event); branch != "" {
			if watched.named == nil {
				watched.named = map[string]bool{}
			}
			watched.named[branch] = true
		}
		return nil
	}
	if !finished {
		return nil
	}

	jobs, known := s.jobs(ctx)
	now, err := s.read(ctx, pollParams{Watch: watched, Jobs: jobs, Known: known})
	if err != nil {
		return nil
	}
	err = s.send(ctx, watched, now)
	if _, ending := errors.AsType[ended](err); ending {
		return err
	}
	return nil
}

// namedBy is the branch an event of an operation names, "" for one that names none.
func namedBy(event domain.Event) string {
	switch e := event.(type) {
	case domain.WorktreeCreated:
		return e.Worktree.Branch
	case domain.WorktreeUpdated:
		return e.Worktree.Branch
	case domain.WorktreeProvisioned:
		return e.Worktree.Branch
	case domain.UnitFinished:
		return e.Subject
	default:
		return ""
	}
}

// jobs are the jobs of the run daemon, and whether it could tell them.
func (s *stream) jobs(ctx context.Context) ([]run.Job, bool) {
	if s.Jobs == nil {
		return nil, false
	}
	jobs, err := s.Jobs(ctx)
	return jobs, err == nil
}

// stillThere ends the stream of source once its repository has no wtm config any more; any other failure to read it waits for the next check.
func (s *stream) stillThere(ctx context.Context, source Source) error {
	if _, err := source.Repository.Observe(ctx, Name); configLost(err) {
		return ended{err}
	}
	return nil
}

// lost is err ending the stream when it says the repository watched lost its wtm config, else err as it is.
func lost(err error) error {
	if configLost(err) {
		return ended{err}
	}
	return err
}

func configLost(err error) bool {
	failure, ok := errors.AsType[*domain.Error](err)
	return ok && failure.Code == CodeConfigNotFound
}

func (s *stream) emitReady() error {
	if err := s.emitLine(domain.Line{Event: domain.Ready{}}); err != nil {
		return err
	}

	s.ready = true
	return nil
}

// emitLine stamps line with the stream's version and time, and sends it.
func (s *stream) emitLine(line domain.Line) error {
	line.V = domain.StreamVersion
	line.TS = s.Now().UTC().Format(time.RFC3339Nano)
	data, err := domain.MarshalLine(line)
	if err != nil {
		return err
	}

	return s.emit(Received{Line: line, JSON: data})
}

// emit sends received; the reader's error ends the stream.
func (s *stream) emit(received Received) error {
	s.started = true
	received.Every = s.every
	if err := s.out.Emit(received); err != nil {
		return ended{err}
	}
	return nil
}

func (s *stream) warn(err error) {
	if s.out.Warn != nil {
		s.out.Warn(err)
	}
}

// interrupted is how a stream ctx ended returns: nil once it sent a line, the interruption before.
func (s *stream) interrupted(ctx context.Context) error {
	if s.started {
		return nil
	}
	return ctx.Err()
}

// ----------------------------------------------------------------------------
// The relay

// relayParams is a subscription relayed: its Lines, what to check every Recheck, and where each line decoded goes.
type relayParams struct {
	Lines   <-chan []byte
	Recheck func(ctx context.Context) error
	Route   func(ctx context.Context, received Received) error
}

// relay hands each line of the subscription to its route until the bus ends it or ctx is done. A line of a newer version ends the stream; one of an older version, or one that cannot be decoded, is dropped with a warning.
func (s *stream) relay(ctx context.Context, params relayParams) error {
	ticker := time.NewTicker(cmp.Or(s.Recheck, recheckEvery))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := params.Recheck(ctx); err != nil {
				return err
			}
		case data, open := <-params.Lines:
			if !open {
				return nil
			}
			// A line of a type this wtm does not know is relayed, its event nil.
			line, err := domain.UnmarshalLine(data)
			if line.V > domain.StreamVersion {
				return ended{&domain.Error{Kind: domain.KindPrecondition, Code: CodeSchemaNewer, Message: "the event stream speaks a newer schema than this wtm: upgrade wtm"}}
			}
			if err != nil && !errors.Is(err, domain.ErrUnknownEventType) {
				s.warn(fmt.Errorf("an event the bus carried could not be read, and was dropped: %w", err))
				continue
			}
			// Whichever daemon holds the socket is the bus: a legacy one relays v1 lines, never translated.
			if line.V < domain.StreamVersion {
				s.warn(fmt.Errorf("a line of the event stream v%d was dropped: this wtm reads v%d alone", line.V, domain.StreamVersion))
				continue
			}
			if err := params.Route(ctx, Received{Line: line, JSON: data}); err != nil {
				return err
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Observe

// observe reads what a snapshot of source reports: its worktrees and their meta records.
func (s *stream) observe(ctx context.Context, source Source) (Facts, error) {
	repo, err := source.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Metas: map[string]worktree.Meta{}}
	for _, w := range repo.Worktrees {
		if branch := w.Branch(); branch != "" {
			if meta, err := source.Meta.Load(branch); err == nil {
				facts.Metas[branch] = meta
			}
		}
	}
	return facts, nil
}

// ----------------------------------------------------------------------------
// Helpers

func notARepository(err error) bool {
	failure, ok := errors.AsType[*domain.Error](err)
	return ok && failure.Code == domain.CodeNotARepository
}

// sleep waits d, and reports whether it did before ctx was done.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
