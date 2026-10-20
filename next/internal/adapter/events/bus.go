package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	daemonPoll    = 50 * time.Millisecond
	daemonTimeout = 5 * time.Second
)

// A Bus is the event bus as a reader subscribes to it: the daemon listening on SocketPath, which Start starts when none answers (nil starts none).
type Bus struct {
	SocketPath string
	Start      func() error
}

// Subscribe returns the lines published for repos, every repository's when
// empty, once the daemon acknowledged; it starts the daemon first when none
// answers, waiting for one still stopping to be gone. The channel is closed
// when the daemon ends the subscription, the connection drops or ctx is done.
func (b Bus) Subscribe(ctx context.Context, repos []string) (<-chan []byte, error) {
	if err := b.ensure(ctx); err != nil {
		return nil, err
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", b.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to the event bus: %w", err)
	}
	stopWatching := context.AfterFunc(ctx, func() { _ = conn.Close() })

	decoder, err := acknowledged(conn, repos)
	if err != nil {
		stopWatching()
		_ = conn.Close()
		return nil, err
	}

	lines := make(chan []byte)
	go func() {
		defer close(lines)
		defer stopWatching()
		defer conn.Close()
		for {
			var resp response
			if decoder.Decode(&resp) != nil {
				return
			}
			if resp.Status != statusEvent {
				continue
			}
			select {
			case lines <- resp.Payload:
			case <-ctx.Done():
				return
			}
		}
	}()
	return lines, nil
}

// acknowledged sends the subscription of repos on conn and returns the decoder of what follows, once the daemon accepted it.
func acknowledged(conn net.Conn, repos []string) (*json.Decoder, error) {
	if err := json.NewEncoder(conn).Encode(request{Action: actionSubscribe, Repos: repos}); err != nil {
		return nil, fmt.Errorf("subscribe to the event bus: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var ack response
	if err := decoder.Decode(&ack); err != nil {
		return nil, fmt.Errorf("subscribe to the event bus: %w", err)
	}
	if ack.Status != statusOK {
		return nil, errors.New("the event bus refused the subscription: " + ack.Message)
	}
	return decoder, nil
}

// ensure returns once a daemon answers: the one running, or the one it starts
// after any daemon holding the lock without answering, stopping or not yet
// listening, is gone.
func (b Bus) ensure(ctx context.Context) error {
	if b.answers() {
		return nil
	}
	if err := poll(ctx, func() bool { return b.answers() || !b.locked() }); err != nil {
		return fmt.Errorf("a stopping event bus did not exit: %w", err)
	}
	if b.answers() {
		return nil
	}

	if b.Start == nil {
		return errors.New("no event bus answers on " + b.SocketPath)
	}
	if err := b.Start(); err != nil {
		return fmt.Errorf("start the event bus: %w", err)
	}
	if err := poll(ctx, b.answers); err != nil {
		return fmt.Errorf("the event bus did not start: %w", err)
	}
	return nil
}

// answers reports whether a daemon accepts a connection on the socket.
func (b Bus) answers() bool {
	conn, err := net.DialTimeout("unix", b.SocketPath, daemonPoll)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// locked reports whether a daemon holds the lock beside the socket.
func (b Bus) locked() bool {
	file, err := statefile.TryLock(lockPath(b.SocketPath), domain.LockExclusive)
	if err != nil {
		return errors.Is(err, statefile.ErrBusy)
	}
	_ = file.Close()
	return false
}

// poll asks done every daemonPoll until it holds, failing once daemonTimeout passed or ctx is done.
func poll(ctx context.Context, done func() bool) error {
	ctx, cancel := context.WithTimeout(ctx, daemonTimeout)
	defer cancel()
	ticker := time.NewTicker(daemonPoll)
	defer ticker.Stop()
	for !done() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}
