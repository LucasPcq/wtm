// Package events is the `wtm events` bus as wtm itself uses it: the publisher
// every flow reports through, and the watcher behind both consumers. The
// daemon relays these and writes only job.*, from domain.JobEvent.
package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type PublisherParams struct {
	ProjectDir string
	// SocketPath is the daemon's; empty is the one every command talks to.
	SocketPath    string
	CorrelationID string
}

// Publisher satisfies flow.Publisher. The repository is resolved once, on the
// first event, so a run that changes nothing costs nothing.
type Publisher struct {
	projectDir    string
	socketPath    string
	correlationID string
	once          sync.Once
	repo          domain.EventRepo
	repoErr       error
}

func NewPublisher(params PublisherParams) *Publisher {
	socket := params.SocketPath
	if socket == "" {
		socket = process.SocketPath()
	}
	return &Publisher{projectDir: params.ProjectDir, socketPath: socket, correlationID: params.CorrelationID}
}

// Publish is opportunistic: a consumer that misses an event gets the state back
// from its next snapshot, so nothing here may fail or slow the command.
func (p *Publisher) Publish(ctx context.Context, event domain.Event) {
	p.resolve(ctx)
	if p.repoErr != nil || p.socketPath == "" {
		return
	}
	event.CorrelationID = p.correlationID
	payload, err := json.Marshal(stamp(stampParams{Event: event, Repo: p.repo}))
	if err != nil {
		return
	}
	_ = process.Publish(process.PublishParams{SocketPath: p.socketPath, Repo: p.repo.CommonDir, Payload: payload})
}

func (p *Publisher) Origin(ctx context.Context) (domain.EventOrigin, bool) {
	p.resolve(ctx)
	if p.repoErr != nil {
		return domain.EventOrigin{}, false
	}
	return domain.EventOrigin{Repo: p.repo, CorrelationID: p.correlationID}, true
}

func (p *Publisher) resolve(ctx context.Context) {
	p.once.Do(func() {
		p.repo, p.repoErr = worktree.RepoOf(ctx, worktree.RepoOfParams{ProjectDir: p.projectDir})
	})
}

// Listening is a dial, never cached: a run may start the daemon halfway
// through, and the events it makes after that have someone to reach.
func (p *Publisher) Listening() bool {
	return p.socketPath != "" && process.IsDaemonRunning(p.socketPath)
}

type stampParams struct {
	Event domain.Event
	Repo  domain.EventRepo
}

func stamp(params stampParams) domain.Event {
	event := params.Event
	event.V = domain.EventsSchemaVersion
	event.TS = now()
	repo := params.Repo
	event.Repo = &repo
	return event
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
