package render_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
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

var (
	catalog = text.NewCatalog()
	wording = render.Wording{Catalog: catalog, Names: asFlag}
)

// plain is what a terminal shows of out, without its escapes.
type pathDetail struct{}

func (pathDetail) Plan(json.RawMessage) []styles.PlanRow { return nil }

func (pathDetail) Item(w io.Writer, item kernel.Item[json.RawMessage]) {
	fmt.Fprintf(w, "    %s  %s\n", item.Subject, item.Detail)
}

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
