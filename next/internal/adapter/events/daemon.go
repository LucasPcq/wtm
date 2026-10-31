package events

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	lockName        = "wtm.lock"
	subscriberQueue = 256
	// writeTimeout bounds a write to a subscriber: one that stopped reading is cut off rather than holding the daemon.
	writeTimeout = time.Second
	// lockWait is how long a daemon starting waits for the lock: a client probing whether a daemon holds it takes it for an instant.
	lockWait  = 250 * time.Millisecond
	lockRetry = 10 * time.Millisecond
	// acceptRetry is the first wait after an accept that failed, doubling up to acceptRetryMax.
	acceptRetry    = 5 * time.Millisecond
	acceptRetryMax = time.Second

	actionPublish   = "publish"
	actionSubscribe = "subscribe"
	statusOK        = "ok"
	statusEvent     = "event"
	statusError     = "error"
)

// ErrDaemonRunning is the error of [Serve] when another daemon holds the lock beside the socket.
var ErrDaemonRunning = errors.New("a daemon already serves this socket")

// request is one line a client writes on a connection: publish Payload, filed under the repository Repo, or subscribe to the events of Repos, every repository's when empty.
type request struct {
	Action  string          `json:"action"`
	Repo    string          `json:"repo,omitempty"`
	Repos   []string        `json:"repos,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// response is one line the daemon writes back: the status of the request, then, to a subscriber, an event per line.
type response struct {
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ServeParams is where [Serve] listens, SocketPath; how long it waits with no
// subscriber, no request and nothing Busy before it stops, Idle; the Relay it
// serves (a new one when nil); and what a run daemon it hosts plugs in: Ready,
// run once the lock is held and before it listens, Other, which answers a
// request of another action than the bus's (its first line, read from conn;
// stop stops the daemon), Busy, which keeps it from idling out while true,
// and Stopping, run once it stopped listening, before the requests in flight
// end. A nil hook does nothing; with no Other an action not the bus's is
// refused as unknown.
type ServeParams struct {
	SocketPath string
	Idle       time.Duration
	Relay      *Relay
	Ready      func()
	Other      func(conn net.Conn, line []byte, stop func())
	Busy       func() bool
	Stopping   func()
}

// A Relay hands each event published on the bus to the subscribers of its
// repository: the daemon's own, which a run daemon hosted in its process
// publishes on directly.
type Relay struct {
	hub *hub
}

// NewRelay returns a Relay with no subscriber.
func NewRelay() *Relay {
	return &Relay{hub: newHub(subscriberQueue)}
}

// Publish relays line to the subscribers of its repository, without a round
// trip through the socket; a line that names no repository, or cannot be
// written, is dropped.
func (r *Relay) Publish(line domain.Line) {
	payload, err := domain.MarshalLine(line)
	if err != nil || line.Repo == nil {
		return
	}
	r.hub.publish(delivery{repo: Resolved(line.Repo.CommonDir), payload: payload})
}

// Serve runs the bus's daemon until ctx is done or it idles out: it relays each
// event published on params.SocketPath to the subscribers of its repository,
// one request per connection. Only one daemon serves a socket, holding the lock
// beside it until its process exits; Serve returns [ErrDaemonRunning] when
// another one holds it. Stopping ends every subscription.
func Serve(ctx context.Context, params ServeParams) error {
	lock, err := statefile.TryLock(lockPath(params.SocketPath), domain.LockExclusive)
	for waited := time.Duration(0); errors.Is(err, statefile.ErrBusy) && waited < lockWait; waited += lockRetry {
		time.Sleep(lockRetry)
		lock, err = statefile.TryLock(lockPath(params.SocketPath), domain.LockExclusive)
	}
	if errors.Is(err, statefile.ErrBusy) {
		return ErrDaemonRunning
	}
	if err != nil {
		return err
	}
	defer lock.Close()

	if params.Ready != nil {
		params.Ready()
	}
	// The lock proves no daemon listens on a socket file left behind: one that died without removing it.
	_ = os.Remove(params.SocketPath)
	listener, err := listenOwned(params.SocketPath)
	if err != nil {
		return err
	}
	defer os.Remove(params.SocketPath)

	ctx, stop := context.WithCancel(ctx)
	defer stop()
	relay := params.Relay
	if relay == nil {
		relay = NewRelay()
	}
	d := &daemon{hub: relay.hub, last: time.Now(), other: params.Other, stop: stop}
	go d.idleOut(ctx, idleOutParams{Stop: stop, Idle: params.Idle, Busy: params.Busy})
	context.AfterFunc(ctx, func() {
		_ = listener.Close()
		d.hub.close()
	})

	var served sync.WaitGroup
	defer served.Wait()
	backoff := acceptRetry
	for {
		conn, err := listener.Accept()
		if err == nil {
			backoff = acceptRetry
			served.Go(func() { d.serve(ctx, conn) })
			continue
		}
		if ctx.Err() != nil {
			if params.Stopping != nil {
				params.Stopping()
			}
			return nil
		}
		// A failed accept (too many open files) is no reason to stop the jobs a daemon holds: wait, then accept again, as net/http does.
		time.Sleep(backoff)
		backoff = min(2*backoff, acceptRetryMax)
	}
}

// listenOwned listens on the unix socket at path, which only this user may connect
// to: a request on it runs a command as this user. The umask is narrowed
// around its creation, so the socket is never open to others, even for an
// instant; nothing else of this process creates a file meanwhile, its
// requests not being served yet.
func listenOwned(path string) (net.Listener, error) {
	umask := syscall.Umask(0o077)
	listener, err := net.Listen("unix", path)
	syscall.Umask(umask)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}

	return listener, nil
}

func lockPath(socketPath string) string {
	return filepath.Join(filepath.Dir(socketPath), lockName)
}

// A daemon is the bus's state: its hub, what answers another action and stops
// it, the requests and subscriptions in flight and when the last one ended.
type daemon struct {
	hub   *hub
	other func(conn net.Conn, line []byte, stop func())
	stop  func()

	mu     sync.Mutex
	active int
	last   time.Time
}

type idleOutParams struct {
	Stop func()
	Idle time.Duration
	Busy func() bool
}

// idleOut stops the daemon once it has had no subscriber, no request and
// nothing busy for params.Idle.
func (d *daemon) idleOut(ctx context.Context, params idleOutParams) {
	ticker := time.NewTicker(max(params.Idle/4, time.Millisecond))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		d.mu.Lock()
		idle := d.active == 0 && time.Since(d.last) >= params.Idle
		d.mu.Unlock()
		if idle && (params.Busy == nil || !params.Busy()) {
			params.Stop()
			return
		}
	}
}

// busy records a request or a subscription starting (by 1) or ending (by -1).
func (d *daemon) busy(by int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active += by
	d.last = time.Now()
}

// serve answers the one request conn carries.
func (d *daemon) serve(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return
	}
	var req request
	// A line that is no request is the run daemon's to refuse, when one is hosted.
	if json.Unmarshal(line, &req) != nil && d.other == nil {
		return
	}
	d.busy(1)
	defer d.busy(-1)

	encoder := json.NewEncoder(conn)
	switch {
	case req.Action == actionPublish:
		d.hub.publish(delivery{repo: req.Repo, payload: req.Payload})
		_ = encoder.Encode(response{Status: statusOK})
	case req.Action == actionSubscribe:
		d.subscribe(ctx, subscription{Conn: conn, Encoder: encoder, Repos: req.Repos})
	case d.other != nil:
		// What the client sent past its request line is in the reader: an attached terminal's first keys.
		d.other(buffered{Conn: conn, reader: reader}, line, d.stop)
	default:
		_ = encoder.Encode(response{Status: statusError, Message: "unknown action: " + req.Action})
	}
}

type subscription struct {
	Conn    net.Conn
	Encoder *json.Encoder
	Repos   []string
}

// buffered is a connection read through the reader that read its request.
type buffered struct {
	net.Conn
	reader *bufio.Reader
}

func (b buffered) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

// subscribe streams until the hub ends the subscription (a full queue, the
// daemon stopping) or the client leaves. The read side is watched because a
// quiet stream never writes: without it a departed subscriber would keep the
// daemon alive forever.
func (d *daemon) subscribe(ctx context.Context, params subscription) {
	sub, unsubscribe := d.hub.subscribe(params.Repos)
	defer unsubscribe()
	if params.Encoder.Encode(response{Status: statusOK}) != nil {
		return
	}

	gone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, params.Conn)
		close(gone)
	}()

	for {
		select {
		case <-gone:
			return
		case <-ctx.Done():
			return
		case delivered, open := <-sub:
			if !open || params.Conn.SetWriteDeadline(time.Now().Add(writeTimeout)) != nil || params.Encoder.Encode(response{Status: statusEvent, Payload: delivered.payload}) != nil {
				return
			}
		}
	}
}

// ----------------------------------------------------------------------------
// The hub

// A hub relays each event published to the subscribers of its repository. It
// never reads an event: a daemon of an older build relays types it does not
// know.
type hub struct {
	mu     sync.Mutex
	subs   map[int]*subscriber
	nextID int
	closed bool
	queue  int
}

type subscriber struct {
	ch    chan delivery
	repos []string
}

type delivery struct {
	repo    string
	payload json.RawMessage
}

func newHub(queue int) *hub {
	return &hub{subs: map[int]*subscriber{}, queue: queue}
}

// subscribe returns the queue of the events of repos, every repository's when empty, and the function that ends the subscription; the queue is closed when the subscription ends.
func (h *hub) subscribe(repos []string) (<-chan delivery, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub := &subscriber{ch: make(chan delivery, h.queue), repos: repos}
	if h.closed {
		close(sub.ch)
		return sub.ch, func() {}
	}

	id := h.nextID
	h.nextID++
	h.subs[id] = sub
	return sub.ch, func() { h.drop(id) }
}

func (h *hub) drop(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sub, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(sub.ch)
	}
}

// publish never waits: a subscriber whose queue is full is cut off, and its
// reader starts again from a fresh snapshot when it subscribes again.
func (h *hub) publish(event delivery) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		if len(sub.repos) > 0 && !slices.Contains(sub.repos, event.repo) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			delete(h.subs, id)
			close(sub.ch)
		}
	}
}

// close ends every subscription and refuses the later ones.
func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, sub := range h.subs {
		delete(h.subs, id)
		close(sub.ch)
	}
}
