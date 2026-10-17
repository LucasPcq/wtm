package env

import (
	"maps"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"

	"github.com/stretchr/testify/require"
)

func sources(t *testing.T, request CopyRequest, exists func(string) (bool, error)) ([]Source, []CopyWarning) {
	t.Helper()
	found, warnings, err := Sources(request, exists)
	require.NoError(t, err)
	return found, warnings
}

func existsIn(present map[string]bool) func(string) (bool, error) {
	return func(path string) (bool, error) { return present[path], nil }
}

type planOneParams struct {
	Content string
	Base    int
	Offset  int
}

func planOne(t *testing.T, params planOneParams) Entry {
	t.Helper()
	plan := Writes{bases: map[portRef]int{{Job: "svc", Name: "DB_PORT"}: params.Base}, block: 10, worktree: settling{Offset: params.Offset}}.plan(
		[]project.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "DB_PORT"}},
		map[string][]dotenv.Line{".env": dotenv.Parse([]byte(params.Content))},
	)
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
		written := string(dotenv.Render(writes.Edit(file, lines)))
		if present && written == content {
			continue
		}
		out[file] = written
		changed = append(changed, file)
	}
	return appliedWrites{Files: out, Changed: changed, Plan: writes.Settled(read)}
}
