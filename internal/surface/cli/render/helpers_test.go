package render_test

import (
	"bytes"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

const (
	waitFor = time.Second
	tick    = 5 * time.Millisecond
)

func asFlag(path string) string {
	return "--" + path
}

func plain(out string) string {
	return ansi.Strip(out)
}

func lines(rows ...string) string {
	return strings.Join(rows, "\n") + "\n"
}

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

func emitRun(view *render.ProgressView) {
	report := kernel.Report(view, "feat/a")
	report.UnitStarted()
	report.Status(kernel.CodeLocksUnavailable, nil)
	finished := report.Phase("on_create")
	report.Output("installing")
	finished()
	report.UnitFinished(kernel.StatusDone)
}
