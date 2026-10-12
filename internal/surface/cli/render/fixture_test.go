package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// The helpers the tests of the engine's rendering share.

const (
	waitFor = time.Second
	tick    = 5 * time.Millisecond
)

// asFlag spells a field path as a flag would.
func asFlag(path string) string {
	return "--" + path
}

// plain is what a terminal shows of out, without its escapes.
func plain(out string) string {
	return ansi.Strip(out)
}

func lines(rows ...string) string {
	return strings.Join(rows, "\n") + "\n"
}

// syncBuffer is a buffer a view may write to from its own goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buf.Bytes())
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func done(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Detail: json.RawMessage(`"/wt/` + subject + `"`)}
}

func unchanged(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Reason: kernel.ReasonUnchanged}
}
