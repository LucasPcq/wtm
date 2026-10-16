package provision_test

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu     sync.Mutex
	events []wtm.Event
	on     func(wtm.Event)
}

func (r *recorder) add(event wtm.Event) {
	r.mu.Lock()
	r.events = append(r.events, event)
	on := r.on
	r.mu.Unlock()
	if on != nil {
		on(event)
	}
}

func (r *recorder) all() []wtm.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func (r *recorder) lines() []string {
	var lines []string
	for _, event := range r.all() {
		if output, ok := event.(wtm.Output); ok {
			lines = append(lines, output.Line)
		}
	}
	return lines
}

func (r *recorder) phases() []string {
	var phases []string
	for _, event := range r.all() {
		if started, ok := event.(wtm.PhaseStarted); ok {
			phases = append(phases, started.Phase)
		}
	}
	return phases
}

func (r *recorder) logOf(phase string) string {
	for _, event := range r.all() {
		if started, ok := event.(wtm.PhaseStarted); ok && started.Phase == phase {
			return started.Log
		}
	}
	return ""
}

func (r *recorder) worktreeEvents() []wtm.Event {
	var events []wtm.Event
	for _, event := range r.all() {
		switch event.(type) {
		case wtm.WorktreeCreated, wtm.WorktreeNumbered, wtm.WorktreeProvisioned:
			events = append(events, event)
		}
	}
	return events
}

func codes(warnings []wtm.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func gitState(t *testing.T, w world) gittest.State {
	t.Helper()
	return gittest.StateOf(t, w.git)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func gitStateNames(t *testing.T, w world) []string {
	t.Helper()
	return gittest.Names(gitState(t, w).Branches.Local)
}
