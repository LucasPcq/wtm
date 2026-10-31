package process

import (
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// printed collects what a child prints, safe to read while it runs.
type printed struct {
	mu  sync.Mutex
	out strings.Builder
}

func (p *printed) write(chunk []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.out.Write(chunk)
}

func (p *printed) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

// spawned starts line in a temporary directory, under a terminal when terminal, and stops its group when the test ends: a test signals only the groups it started.
func spawned(t *testing.T, line string, terminal bool) (*Child, *printed) {
	t.Helper()
	out := &printed{}
	child, err := Spawn(run.Command{Line: line, Dir: t.TempDir(), Terminal: terminal, Output: out.write})
	require.NoError(t, err)
	t.Cleanup(func() { child.Stop(0) })
	return child, out
}
