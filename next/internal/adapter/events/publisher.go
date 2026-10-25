// Package events is wtm-next's event bus: the daemon that relays the events of every repository to the processes subscribed to it, the publisher every operation reports through, and the subscription a stream reads, all over one unix socket.
package events

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	publishTimeout = 250 * time.Millisecond
	globalDirName  = "wtm"
	socketName     = "wtm.sock"
)

// PublisherParams is what a [Publisher] stamps on every event: the repository
// (Root and CommonDir), the daemon's socket at SocketPath ("" to publish
// nothing), the operation OpID, the CorrelationID of the client that asked,
// and the clock Now (time.Now when nil).
type PublisherParams struct {
	Root          string
	CommonDir     string
	SocketPath    string
	OpID          string
	CorrelationID string
	Now           func() time.Time
}

// A Publisher sends events to the bus's daemon, best effort: an event that
// cannot be delivered within a short timeout is dropped, never an error, so a
// missing daemon never slows nor fails a command, and it never starts one.
type Publisher struct {
	repo          domain.Repo
	socketPath    string
	opID          string
	correlationID string
	now           func() time.Time
}

// NewPublisher returns a Publisher for params, with the common directory
// resolved through symbolic links since the daemon keys repositories by it.
func NewPublisher(params PublisherParams) Publisher {
	now := params.Now
	if now == nil {
		now = time.Now
	}

	return Publisher{
		repo:          domain.Repo{Root: params.Root, CommonDir: Resolved(params.CommonDir)},
		socketPath:    params.SocketPath,
		opID:          params.OpID,
		correlationID: params.CorrelationID,
		now:           now,
	}
}

// Resolved returns the common directory dir with its symbolic links resolved, the key the bus files a repository's events under, or dir when they cannot be.
func Resolved(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}

// SocketPath returns the path of the socket the bus's daemon listens on, under
// the user's config directory, or "" when that directory is unknown.
func SocketPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, globalDirName, socketName)
}

// Publish sends event to the daemon as a line of the stream stamped with the
// publisher's repository, operation and time, if the bus carries that kind of
// event; it returns once the daemon replied or the timeout passed.
func (p Publisher) Publish(event domain.Event) {
	if !published(event) || p.socketPath == "" {
		return
	}

	payload, err := domain.MarshalLine(domain.Line{
		V:             domain.StreamVersion,
		TS:            p.now().UTC().Format(time.RFC3339Nano),
		Repo:          &p.repo,
		CorrelationID: p.correlationID,
		OpID:          p.opID,
		Event:         event,
	})
	if err != nil {
		return
	}
	p.send(payload)
}

// published reports whether the bus carries event: units started, phases and
// their output stay in the process, and a snapshot, ready and a worktree the
// stream polled changed are the stream's own lines, never published.
func published(event domain.Event) bool {
	switch event.(type) {
	case domain.WorktreeCreated, domain.WorktreeUpdated, domain.WorktreeProvisioned, domain.WorktreeRemoved, domain.OperationStarted, domain.UnitFinished, domain.OperationFinished, domain.RepoAdded, domain.RepoRemoved:
		return true
	case domain.UnitStarted, domain.PhaseStarted, domain.PhaseFinished, domain.Output, domain.Snapshot, domain.Ready, domain.WorktreeChanged:
	}
	return false
}

// send delivers payload to the daemon and waits for its reply, giving up
// silently on any failure.
func (p Publisher) send(payload json.RawMessage) {
	conn, err := net.DialTimeout("unix", p.socketPath, publishTimeout)
	if err != nil {
		return
	}
	defer conn.Close()

	if conn.SetDeadline(time.Now().Add(publishTimeout)) != nil {
		return
	}
	if json.NewEncoder(conn).Encode(request{Action: actionPublish, Repo: p.repo.CommonDir, Payload: payload}) != nil {
		return
	}

	// Waiting for the reply keeps the connection open until the daemon has read the event.
	var reply json.RawMessage
	_ = json.NewDecoder(conn).Decode(&reply)
}
