package runenv

import (
	"maps"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/require"
)

func expandFor(t *testing.T, value string, params expandValueParams) (string, error) {
	t.Helper()
	params.Link = run.EnvValueLink{File: ".env", Key: "K", Job: params.Job.Name, Value: value}
	return expandValue(params)
}

func expandedNamespace(t *testing.T, params namespaceParams) string {
	t.Helper()
	name, err := expandNamespace(params)
	require.NoError(t, err)
	return name
}

type planOneParams struct {
	Content string
	Base    int
	Offset  int
}

func planOne(t *testing.T, params planOneParams) Entry {
	t.Helper()
	plan := planPorts(planPortsParams{
		Links:  []run.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "DB_PORT"}},
		Bases:  map[portRef]int{{Job: "svc", Name: "DB_PORT"}: params.Base},
		Offset: params.Offset,
		Lines:  map[string][]dotenv.Line{".env": dotenv.Parse([]byte(params.Content))},
	})
	require.Len(t, plan.Entries, 1)
	return plan.Entries[0]
}

func cookieParent(host string) string {
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			return host[i+1:]
		}
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
