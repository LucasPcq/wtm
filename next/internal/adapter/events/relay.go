package events

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	lockName        = "wtm.lock"
	subscriberQueue = 256
	// writeTimeout bounds a write to a subscriber: one that stopped reading is cut off rather than holding the daemon.
	writeTimeout = time.Second

	actionPublish   = "publish"
	actionSubscribe = "subscribe"
	statusOK        = "ok"
	statusEvent     = "event"
	statusError     = "error"
)

// request is one line a client writes on a connection: publish Payload, filed under the repository Repo, or subscribe to the events of Repos, every repository's when empty.
type request struct {
	Action  string          `json:"action"`
	Repo    string          `json:"repo,omitempty"`
	Repos   []string        `json:"repos,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// response is one line the bus writes back: the status of the request, then, to a subscriber, an event per line.
type response struct {
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// A Relay is the event bus wtm's daemon carries: it hands each event
// published to the subscribers of its repository, those the daemon's socket
// brings it and the daemon's own events, which its job supervision publishes
// on it directly.
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

// Serve answers the bus's request line read from conn: a publish, relayed to
// the subscribers of its repository, or a subscription, streamed until the
// relay closes, ctx is done or the client leaves.
func (r *Relay) Serve(ctx context.Context, conn net.Conn, line []byte) {
	encoder := json.NewEncoder(conn)
	var req request
	if json.Unmarshal(line, &req) != nil {
		_ = encoder.Encode(response{Status: statusError, Message: "invalid request"})
		return
	}
	switch req.Action {
	case actionPublish:
		r.hub.publish(delivery{repo: req.Repo, payload: req.Payload})
		_ = encoder.Encode(response{Status: statusOK})
	case actionSubscribe:
		r.subscribe(ctx, subscription{Conn: conn, Encoder: encoder, Repos: req.Repos})
	default:
		_ = encoder.Encode(response{Status: statusError, Message: "unknown action: " + req.Action})
	}
}

// Close ends every subscription and refuses the later ones: the daemon is stopping.
func (r *Relay) Close() {
	r.hub.close()
}

type subscription struct {
	Conn    net.Conn
	Encoder *json.Encoder
	Repos   []string
}

// subscribe streams until the hub ends the subscription (a full queue, the
// relay closing) or the client leaves. The read side is watched because a
// quiet stream never writes: without it a departed subscriber would keep the
// daemon alive forever.
func (r *Relay) subscribe(ctx context.Context, params subscription) {
	sub, unsubscribe := r.hub.subscribe(params.Repos)
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
