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

// existsIn returns an exists function answering from present.
func existsIn(present map[string]bool) func(string) (bool, error) {
	return func(path string) (bool, error) { return present[path], nil }
}

type planOneParams struct {
	Content string
	Base    int
	Offset  int
}

// planOne plans the single key URL of a .env holding Content, linked to a port with base Base, for a worktree at Offset.
func planOne(t *testing.T, params planOneParams) Entry {
	t.Helper()
	plan := Writes{bases: map[portRef]int{{Job: "svc", Name: "DB_PORT"}: params.Base}, block: 10, worktree: settling{Offset: params.Offset}}.plan(
		[]project.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "DB_PORT"}},
		map[string][]dotenv.Line{".env": dotenv.Parse([]byte(params.Content))},
	)
	require.Len(t, plan.Entries, 1)
	return plan.Entries[0]
}

// cookieParent returns the domain a cookie set by host would be scoped to: host without its first label.
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

// appliedWrites is what applying writes to a set of files did: the files after, the ones that changed, and the settled plan.
type appliedWrites struct {
	Files   map[string]string
	Changed []string
	Plan    Plan
}

// applyWrites applies writes to files, given as path to content, the way a store would: a missing file is written only when writes creates it, and a file left byte for byte as it was is not counted as changed.
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

// lines are the lines of a .env file holding content.
func lines(content string) []dotenv.Line {
	return dotenv.Parse([]byte(content))
}
