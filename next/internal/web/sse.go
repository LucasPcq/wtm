package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// The names of the events of a stream besides its lines, which go out unnamed.
const (
	eventEnvelope = "envelope" // how the operation, or the stream, ended
	eventWarning  = "warning"  // what kept the event stream from a line or a repository for a while
)

// An eventStream writes server-sent events to a response, one writer at a time: an operation may report from several goroutines.
type eventStream struct {
	w      http.ResponseWriter
	mu     sync.Mutex
	opened bool
}

func newEventStream(w http.ResponseWriter) *eventStream {
	return &eventStream{w: w}
}

// open writes the headers of the stream, once.
func (e *eventStream) open() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.opened {
		return
	}

	e.opened = true
	e.w.Header().Set("Content-Type", "text/event-stream")
	e.w.Header().Set("Cache-Control", "no-cache")
	e.w.WriteHeader(http.StatusOK)
	e.flush()
}

// isOpen reports whether the stream wrote its headers.
func (e *eventStream) isOpen() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.opened
}

// line sends line stamped with the stream's version and the time, as the event stream spells it.
func (e *eventStream) line(line domain.Line) {
	line.V = domain.StreamVersion
	line.TS = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := domain.MarshalLine(line)
	if err != nil {
		return
	}
	_ = e.raw(data)
}

// raw sends data as an unnamed event; it fails once the client left.
func (e *eventStream) raw(data json.RawMessage) error {
	return e.write("", data)
}

// named sends v as the event called name.
func (e *eventStream) named(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = e.write(name, data)
}

func (e *eventStream) write(name string, data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if name != "" {
		if _, err := fmt.Fprintf(e.w, "event: %s\n", name); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(e.w, "data: %s\n\n", data); err != nil {
		return err
	}

	e.flush()
	return nil
}

func (e *eventStream) flush() {
	if flusher, ok := e.w.(http.Flusher); ok {
		flusher.Flush()
	}
}
