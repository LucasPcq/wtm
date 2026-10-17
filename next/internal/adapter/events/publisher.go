// Package events publishes the worktree and operation events of wtm-next to the run daemon, which relays them to wtm events and the dashboard.
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
	schemaVersion  = 1
	publishTimeout = 250 * time.Millisecond
	globalDirName  = "wtm"
	socketName     = "wtm.sock"
	actionPublish  = "publish"
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

// A Publisher sends an operation's events to the run daemon, best effort: an
// event that cannot be delivered within a short timeout is dropped, never an
// error, so a missing daemon never slows nor fails a command.
type Publisher struct {
	repo          repo
	socketPath    string
	opID          string
	correlationID string
	now           func() time.Time
}

// NewPublisher returns a Publisher for params, with the common directory
// resolved through symbolic links since the daemon keys repositories by it.
func NewPublisher(params PublisherParams) Publisher {
	commonDir := params.CommonDir
	if resolved, err := filepath.EvalSymlinks(commonDir); err == nil {
		commonDir = resolved
	}

	now := params.Now
	if now == nil {
		now = time.Now
	}

	return Publisher{
		repo:          repo{Root: params.Root, CommonDir: commonDir},
		socketPath:    params.SocketPath,
		opID:          params.OpID,
		correlationID: params.CorrelationID,
		now:           now,
	}
}

// SocketPath returns the path of the socket wtm's daemon listens on, under
// the user's config directory, or "" when that directory is unknown.
func SocketPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, globalDirName, socketName)
}

// Publish sends event to the daemon stamped with the publisher's repository,
// operation and time, if wtm publishes that kind of event; it returns once the
// daemon replied or the timeout passed.
func (p Publisher) Publish(event domain.Event) {
	message, published := messageOf(event)
	if !published || p.socketPath == "" {
		return
	}

	message.V = schemaVersion
	message.TS = p.now().UTC().Format(time.RFC3339Nano)
	message.CorrelationID = p.correlationID
	message.Repo = &p.repo
	message.OpID = p.opID

	payload, err := json.Marshal(message)
	if err != nil {
		return
	}
	p.send(payload)
}

// request is what the daemon reads on its socket: one JSON line.
type request struct {
	Action  string          `json:"action"`
	Repo    string          `json:"repo,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
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
