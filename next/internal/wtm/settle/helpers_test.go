package settle

import (
	"maps"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/require"
)

type planOneParams struct {
	Content string
	Base    int
	Offset  int
}

func planOne(t *testing.T, params planOneParams) Entry {
	t.Helper()
	plan := Writes{bases: map[portRef]int{{Job: "svc", Name: "DB_PORT"}: params.Base}, block: 10, worktree: worktree{Offset: params.Offset}}.plan(planOfParams{
		Links: []run.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "DB_PORT"}},
		Lines: map[string][]dotenv.Line{".env": dotenv.Parse([]byte(params.Content))},
	})
	require.Len(t, plan.Entries, 1)
	return plan.Entries[0]
}

func cookieParent(host string) string {
	if _, parent, found := strings.Cut(host, "."); found {
		return parent
	}
	return host
}

func renderPlanned(lines []dotenv.Line) string {
	return string(dotenv.Render(lines))
}

func resolvedWrites(t *testing.T, params ResolveParams) Writes {
	t.Helper()
	writes, err := Resolve(params)
	require.NoError(t, err)
	return writes
}

type appliedWrites struct {
	Files   map[string]string
	Changed []string
	Plan    Plan
}

func applyWrites(writes Writes, files map[string]string) appliedWrites {
	out := maps.Clone(files)
	if out == nil {
		out = map[string]string{}
	}
	read := map[string][]dotenv.Line{}
	var changed []string
	for _, file := range writes.Files() {
		content, present := files[file]
		if !present && !writes.Creates(file) {
			continue
		}
		var lines []dotenv.Line
		if present {
			lines = dotenv.Parse([]byte(content))
		}
		read[file] = lines
		written := string(dotenv.Render(writes.Edit(EditParams{File: file, Lines: lines})))
		if present && written == content {
			continue
		}
		out[file] = written
		changed = append(changed, file)
	}
	return appliedWrites{Files: out, Changed: changed, Plan: writes.Settled(read)}
}
