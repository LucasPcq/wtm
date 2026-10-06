package process

import (
	"encoding/json"
	"slices"
	"sync"
)

// eventHub relays events it never reads: the payload is opaque, so a daemon of
// an older build relays types it does not know, and the daemon stays blind to
// the git state the events describe. publishJob is its one writer of its own.
type eventHub struct {
	mu     sync.Mutex
	subs   map[int]*eventSub
	nextID int
	closed bool
	queue  int
}

type eventSub struct {
	ch    chan eventDelivery
	repos []string
}

type eventDelivery struct {
	repo    string
	payload json.RawMessage
}

func newEventHub(queue int) *eventHub {
	return &eventHub{subs: map[int]*eventSub{}, queue: queue}
}

func (h *eventHub) subscribe(repos []string) (*eventSub, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub := &eventSub{ch: make(chan eventDelivery, h.queue), repos: repos}
	if h.closed {
		close(sub.ch)
		return sub, func() {}
	}
	id := h.nextID
	h.nextID++
	h.subs[id] = sub
	return sub, func() { h.drop(id) }
}

func (h *eventHub) drop(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[id]
	if !ok {
		return
	}
	delete(h.subs, id)
	close(sub.ch)
}

type eventHubPublishParams struct {
	Repo    string
	Payload json.RawMessage
}

// publish never waits: a subscriber whose queue is full is cut off, and its
// consumer resynchronises from a fresh snapshot when it reconnects.
func (h *eventHub) publish(params eventHubPublishParams) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		if len(sub.repos) > 0 && !slices.Contains(sub.repos, params.Repo) {
			continue
		}
		select {
		case sub.ch <- eventDelivery{repo: params.Repo, payload: params.Payload}:
		default:
			delete(h.subs, id)
			close(sub.ch)
		}
	}
}

func (h *eventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, sub := range h.subs {
		delete(h.subs, id)
		close(sub.ch)
	}
}
