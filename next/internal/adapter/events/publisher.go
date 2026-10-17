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

type PublisherParams struct {
	Root          string
	CommonDir     string
	SocketPath    string
	OpID          string
	CorrelationID string
	Now           func() time.Time
}

type Publisher struct {
	repo          repo
	socketPath    string
	opID          string
	correlationID string
	now           func() time.Time
}

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

func SocketPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, globalDirName, socketName)
}

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

type request struct {
	Action  string          `json:"action"`
	Repo    string          `json:"repo,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

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
	var reply json.RawMessage
	_ = json.NewDecoder(conn).Decode(&reply)
}
