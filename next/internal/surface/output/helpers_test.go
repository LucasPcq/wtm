package output

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update      = flag.Bool("update", false, "rewrite the golden files")
	testdata, _ = filepath.Abs("testdata")
)

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join(testdata, name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func rendered(draw func(w *bytes.Buffer)) string {
	var out bytes.Buffer
	draw(&out)
	return out.String()
}

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

const (
	time2s = 2 * time.Second
	tick   = 10 * time.Millisecond
)

func hookRun(progress *Progress, lines ...string) {
	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "worktree"})
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "worktree"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "on_create", Log: "/state/hooks/on_create-feat%2Fx.log"})
	for _, line := range lines {
		progress.Report(domain.Output{Subject: "feat/x", Line: line})
	}
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "on_create"})
	progress.Report(domain.UnitFinished{Subject: "feat/x", Status: domain.StatusDone})
}

func finalScreen(raw string) []string {
	var rows []string
	for line := range strings.SplitSeq(raw, "\n") {
		for {
			before, after, found := strings.Cut(line, "\x1b[")
			if !found {
				break
			}
			count, rest, _ := strings.Cut(after, "F")
			n, _ := strconv.Atoi(count)
			rows = append(rows, before)
			rows = rows[:max(len(rows)-1-n, 0)]
			line = strings.TrimPrefix(strings.TrimPrefix(rest, "\x1b[J"), "\x1b[2K")
		}
		rows = append(rows, line)
	}
	return rows[:len(rows)-1]
}

func envelopeOf(t *testing.T, params EnvelopeParams) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, Envelope(&out, params))
	return out.String()
}
