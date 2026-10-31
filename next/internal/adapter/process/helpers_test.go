package process

import (
	"os/exec"
	"strconv"
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

// zombies counts the zombies of the group pgid, as ps lists them.
func zombies(t *testing.T, pgid int) int {
	t.Helper()
	out, err := exec.Command("ps", "-Ao", "pgid=,stat=").Output()
	require.NoError(t, err)
	count := 0
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == strconv.Itoa(pgid) && strings.HasPrefix(fields[1], "Z") {
			count++
		}
	}
	return count
}
