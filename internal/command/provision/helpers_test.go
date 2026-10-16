package provision_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	mu       sync.Mutex
	progress []kernel.Progress
	onLine   func(string)
}

func (r *recorder) Emit(progress kernel.Progress) {
	r.mu.Lock()
	r.progress = append(r.progress, progress)
	onLine := r.onLine
	r.mu.Unlock()
	if output, ok := progress.(*kernel.Output); ok && onLine != nil {
		onLine(output.Line)
	}
}

func (r *recorder) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var lines []string
	for _, progress := range r.progress {
		if output, ok := progress.(*kernel.Output); ok {
			lines = append(lines, output.Line)
		}
	}
	return lines
}

func (r *recorder) logOf(phase string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, progress := range r.progress {
		if started, ok := progress.(*kernel.PhaseStarted); ok && started.Phase == phase {
			return started.Log
		}
	}
	return ""
}

type runParams struct {
	Unit     kernel.Unit[provision.Detail]
	Recorder *recorder
}

func run(ctx context.Context, params runParams) kernel.Item[provision.Detail] {
	if params.Recorder == nil {
		params.Recorder = &recorder{}
	}
	items := kernel.Each(ctx, kernel.EachParams[string, provision.Detail]{
		Items:   []string{branch},
		Subject: func(subject string) string { return subject },
		Unit:    func(string) kernel.Unit[provision.Detail] { return params.Unit },
		Emit:    params.Recorder,
	})
	return items[0]
}

func phases(unit kernel.Unit[provision.Detail]) []string {
	names := make([]string, 0, len(unit.Then))
	for _, phase := range unit.Then {
		names = append(names, phase.Name)
	}
	return names
}

func codes(warnings []kernel.Warning) []kernel.Code {
	found := make([]kernel.Code, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func snapshot(t *testing.T, w world) func() string {
	return func() string {
		records, err := w.meta.Records()
		require.NoError(t, err)
		entries, err := os.ReadDir(w.root)
		require.NoError(t, err)
		folders := make([]string, 0, len(entries))
		for _, entry := range entries {
			folders = append(folders, entry.Name())
		}
		slices.Sort(folders)
		return fmt.Sprintf("%+v\n%+v\n%v", gittest.StateOf(t, w.git), records, folders)
	}
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
