package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func (d *daemonServer) handlePublish(encoder replyEncoder, req Request) {
	d.events.publish(eventHubPublishParams{Repo: req.Repo, Payload: req.Payload})
	encoder.Encode(Response{Status: StatusOK})
}

// handleSubscribe streams until the hub ends it (shutdown, or a full queue) or
// the client leaves. The read side is watched because a quiet stream never
// writes: without it a departed subscriber would hold the daemon alive forever.
func (d *daemonServer) handleSubscribe(conn net.Conn, encoder replyEncoder, req Request) {
	sub, unsub := d.events.subscribe(req.Repos)
	defer unsub()
	if err := encoder.Encode(Response{Status: StatusOK}); err != nil {
		return
	}

	gone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		close(gone)
	}()

	for {
		select {
		case <-gone:
			return
		case <-d.shutdown:
			return
		case delivery, open := <-sub.ch:
			if !open {
				return
			}
			if err := encoder.Encode(Response{Status: StatusEvent, Repo: delivery.repo, Payload: delivery.payload}); err != nil {
				return
			}
		}
	}
}

type PublishParams struct {
	SocketPath string
	Repo       string
	Payload    json.RawMessage
}

// Publish never starts a daemon: with none running nobody listens, and the next
// subscriber reads the state from its snapshot. It skips the version check on
// purpose — the broker relays what it does not read, whatever its build.
func Publish(params PublishParams) error {
	conn, err := net.DialTimeout("unix", params.SocketPath, domain.EventsPublishTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(domain.EventsPublishTimeout)); err != nil {
		return err
	}
	if err := json.NewEncoder(conn).Encode(Request{Action: ActionPublish, Repo: params.Repo, Payload: params.Payload}); err != nil {
		return err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	if resp.Status != StatusOK {
		return errors.New(resp.Message)
	}
	return nil
}

type SubscribeParams struct {
	SocketPath string
	Repos      []string
}

type Delivery struct {
	Repo    string
	Payload json.RawMessage
}

// Subscribe returns once the daemon acknowledged. The channel closes when the
// daemon ends the stream, the connection drops or ctx is done; a daemon of
// another build is refused before anything is streamed.
func Subscribe(ctx context.Context, params SubscribeParams) (<-chan Delivery, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", params.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	stopWatching := context.AfterFunc(ctx, func() { conn.Close() })
	decoder, err := acknowledge(acknowledgeParams{Conn: conn, Repos: params.Repos})
	if err != nil {
		stopWatching()
		conn.Close()
		return nil, err
	}

	deliveries := make(chan Delivery)
	go func() {
		defer close(deliveries)
		defer stopWatching()
		defer conn.Close()
		for {
			var resp Response
			if err := decoder.Decode(&resp); err != nil {
				return
			}
			if resp.Status != StatusEvent {
				continue
			}
			select {
			case deliveries <- Delivery{Repo: resp.Repo, Payload: resp.Payload}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return deliveries, nil
}

type acknowledgeParams struct {
	Conn  net.Conn
	Repos []string
}

func acknowledge(params acknowledgeParams) (*json.Decoder, error) {
	if err := json.NewEncoder(params.Conn).Encode(Request{Action: ActionSubscribe, Repos: params.Repos}); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	decoder := json.NewDecoder(params.Conn)
	var ack Response
	if err := decoder.Decode(&ack); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if err := checkVersion(ack); err != nil {
		return nil, err
	}
	if ack.Status != StatusOK {
		return nil, errors.New(ack.Message)
	}
	return decoder, nil
}
