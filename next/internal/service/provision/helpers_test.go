package provision_test

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/require"
)

// recorder records the events a run reports, safe across goroutines; on, when
// set, is called with each one as it arrives.
type recorder struct {
	mu     sync.Mutex
	events []domain.Event
	on     func(domain.Event)
}

func (r *recorder) add(event domain.Event) {
	r.mu.Lock()
	r.events = append(r.events, event)
	on := r.on
	r.mu.Unlock()
	if on != nil {
		on(event)
	}
}

func (r *recorder) all() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func (r *recorder) lines() []string {
	var lines []string
	for _, event := range r.all() {
		if output, ok := event.(domain.Output); ok {
			lines = append(lines, output.Line)
		}
	}
	return lines
}

func (r *recorder) phases() []string {
	var phases []string
	for _, event := range r.all() {
		if started, ok := event.(domain.PhaseStarted); ok {
			phases = append(phases, started.Phase)
		}
	}
	return phases
}

func (r *recorder) logOf(phase string) string {
	for _, event := range r.all() {
		if started, ok := event.(domain.PhaseStarted); ok && started.Phase == phase {
			return started.Log
		}
	}
	return ""
}

func (r *recorder) worktreeEvents() []domain.Event {
	var events []domain.Event
	for _, event := range r.all() {
		switch event.(type) {
		case domain.WorktreeCreated, domain.WorktreeUpdated, domain.WorktreeProvisioned:
			events = append(events, event)
		}
	}
	return events
}

func codes(warnings []domain.Warning) []string {
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

func fastForwardEach(subject string, snapshot worktree.Snapshot) provision.FastForwardEachParams {
	return provision.FastForwardEachParams{Command: "create", Subjects: []string{subject}, Snapshot: snapshot, Fallback: "it starts from your local branch"}
}
