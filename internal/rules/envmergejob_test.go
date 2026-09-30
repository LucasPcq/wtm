package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestMergeEnvReplacesRatherThanAppends(t *testing.T) {
	got := MergeEnv(MergeEnvParams{
		Env:       []string{"PATH=/bin", domain.EnvOrdinal + "=9"},
		Overrides: map[string]string{domain.EnvOrdinal: "2"},
	})

	if value, _ := LookupEnv(got, domain.EnvOrdinal); value != "2" {
		t.Errorf("%s = %q, want %q", domain.EnvOrdinal, value, "2")
	}
	if len(got) != 2 {
		t.Errorf("environment has %d entries, want 2 — the key was appended, not replaced", len(got))
	}
}

// Clear is what makes a request that resolved nothing safe: the job must end up
// with no worktree identity rather than with the one it inherited.
func TestMergeEnvClearsBeforeApplying(t *testing.T) {
	inherited := []string{"PATH=/bin", domain.EnvComposeProjectName + "=another-worktree"}

	got := MergeEnv(MergeEnvParams{Env: inherited, Clear: domain.WorktreeScopedEnv})
	if value, ok := LookupEnv(got, domain.EnvComposeProjectName); ok {
		t.Errorf("%s survived as %q, want it dropped", domain.EnvComposeProjectName, value)
	}

	got = MergeEnv(MergeEnvParams{
		Env:       inherited,
		Clear:     domain.WorktreeScopedEnv,
		Overrides: map[string]string{domain.EnvComposeProjectName: "feat-x"},
	})
	if value, _ := LookupEnv(got, domain.EnvComposeProjectName); value != "feat-x" {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, value, "feat-x")
	}
}

func TestMergeEnvIsDeterministic(t *testing.T) {
	overrides := map[string]string{"B": "2", "A": "1", "C": "3"}

	first := MergeEnv(MergeEnvParams{Env: []string{"PATH=/bin"}, Overrides: overrides})
	for range 20 {
		next := MergeEnv(MergeEnvParams{Env: []string{"PATH=/bin"}, Overrides: overrides})
		for i := range first {
			if first[i] != next[i] {
				t.Fatalf("map iteration leaked into the result: %v then %v", first, next)
			}
		}
	}
}
