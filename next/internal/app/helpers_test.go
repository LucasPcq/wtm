package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu     sync.Mutex
	events []wtm.Event
}

func (r *recorder) add(event wtm.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) all() []wtm.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func refusal(t *testing.T, err error) *wtm.Error {
	t.Helper()
	failure, ok := errors.AsType[*wtm.Error](err)
	require.True(t, ok, err)
	return failure
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func containsKind[T wtm.Event](events []wtm.Event) bool {
	return slices.ContainsFunc(events, func(event wtm.Event) bool {
		_, is := event.(T)
		return is
	})
}

func codesOf(warnings []wtm.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}
