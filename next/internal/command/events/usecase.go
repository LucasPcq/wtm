// Package events streams the changes of a repository's worktrees, or of every repository wtm knows, as they happen: a snapshot, ready, then each change another process publishes on the event bus. A stream, it changes nothing and never ends on its own.
package events

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
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

// recheckEvery is how often a stream checks that what it watches is still initialized, when the use case says nothing.
const recheckEvery = 30 * time.Second

// A UseCase streams the events of the repositories wtm knows: the bus it subscribes to, the registry of those repositories, how it opens one, its clock, and how often it checks what it watches is still there (Recheck, 30 seconds when zero).
type UseCase struct {
	Bus      Bus
	Registry Registry
	Open     Open
	Now      func() time.Time
	Recheck  time.Duration
}

// A Source is one repository a stream watches: how the events name it, the service that reads its worktrees, and their meta records.
type Source struct {
	Repo       domain.Repo
	Repository repository.Service
	Meta       Meta
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

// Watch streams the events req asks for to out until ctx is done: a snapshot of each repository watched, ready, then every change published on the bus. When the bus goes away it subscribes again, after a backoff, and opens on fresh snapshots: nothing is replayed, so a reader takes every event as an upsert keyed by branch and every snapshot as a reset.
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

// scope returns the repository req watches, or reports that it watches every one: --all, or no --repo from an unnamed directory in no repository.
func (u UseCase) scope(ctx context.Context, req Request) (Source, bool, error) {
	if err := req.Validate(); err != nil {
		return Source{}, false, err
	}
	if req.All {
		return Source{}, true, nil
	}

	dir := cmp.Or(req.Repo, req.Dir)
	source, err := u.Open(ctx, dir)
	switch {
	case req.Repo != "" && errors.Is(err, fs.ErrNotExist):
		return Source{}, false, notADirectory(req.Repo)
	case req.Repo == "" && !req.Named && notARepository(err):
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

	if err := s.snapshot(ctx, source); err != nil {
		return lost(err)
	}
	if err := s.emitReady(); err != nil {
		return err
	}

	return s.relay(ctx, relayParams{
		Lines: lines,
		Recheck: func(ctx context.Context) error {
			return s.stillThere(ctx, source)
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
			return nil
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
	watched := map[string]bool{}
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
		// A repository that goes away is announced by the prune, and its repo.removed reaches this stream like any line.
		Recheck: func(ctx context.Context) error {
			_, _ = s.Registry.Prune(ctx)
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
			case added && !watched[repo.CommonDir]:
				if err := s.emit(received); err != nil {
					return err
				}
				return s.join(ctx, joinParams{Repo: *repo, Watched: watched})
			case added || !watched[repo.CommonDir]:
				return nil
			case removed:
				delete(watched, repo.CommonDir)
			}

			return s.emit(received)
		},
	})
}

type joinParams struct {
	Repo    domain.Repo
	Watched map[string]bool
}

// join sends the snapshot of params.Repo and counts it among those watched. A repository it cannot read is skipped with a warning rather than ending the stream: one broken repository must not blind the reader to every other.
func (s *stream) join(ctx context.Context, params joinParams) error {
	source, err := s.Open(ctx, params.Repo.Root)
	if err == nil {
		err = s.snapshot(ctx, source)
	}
	if ending, ok := errors.AsType[ended](err); ok {
		return ending
	}
	if err != nil {
		s.warn(fmt.Errorf("%s: %w", params.Repo.Root, err))
		return nil
	}

	params.Watched[source.Repo.CommonDir] = true
	return nil
}

// snapshot sends every worktree of source on a branch.
func (s *stream) snapshot(ctx context.Context, source Source) error {
	facts, err := s.observe(ctx, source)
	if err != nil {
		return err
	}

	return s.emitLine(domain.Line{Repo: &source.Repo, Event: domain.Snapshot{Worktrees: Identities(facts)}})
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
