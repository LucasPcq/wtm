// Package events publishes the worktree and operation events of wtm-next to the run daemon, which relays them to wtm events and the dashboard.
package events

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const (
	schemaVersion  = 1
	publishTimeout = 250 * time.Millisecond
	globalDirName  = "wtm"
	socketName     = "wtm.sock"
	actionPublish  = "publish"
	statusOK       = "ok"
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

func (p Publisher) Publish(event wtm.Event) {
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
	_ = p.send(payload)
}

type request struct {
	Action  string          `json:"action"`
	Repo    string          `json:"repo,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (p Publisher) send(payload json.RawMessage) error {
	conn, err := net.DialTimeout("unix", p.socketPath, publishTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(publishTimeout)); err != nil {
		return err
	}
	if err := json.NewEncoder(conn).Encode(request{Action: actionPublish, Repo: p.repo.CommonDir, Payload: payload}); err != nil {
		return err
	}
	var answer response
	if err := json.NewDecoder(conn).Decode(&answer); err != nil {
		return err
	}
	if answer.Status != statusOK {
		return errors.New(answer.Message)
	}
	return nil
}
