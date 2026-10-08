package runjobs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
)

func namespaceConfig(detach string) domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "db", Kind: domain.JobKindService, Cmd: "sleep 1",
		Scope:     domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "crm_{worktree}", Create: "true", Remove: detach},
	}}}
}

func worktreeEnv() map[string]string {
	return map[string]string{domain.EnvWorktree: "feat_a", domain.EnvOrdinal: "2"}
}

func TestDetachWorktreeRunsTheCommandWithTheNamespaceName(t *testing.T) {
	dir := t.TempDir()
	witness := filepath.Join(dir, "witness")

	got := runjobs.RemoveWorktreeNamespaces(t.Context(), runjobs.RemoveNamespacesParams{
		Config:  namespaceConfig("printf '%s\\n' \"$WTM_NAMESPACE\" >> " + witness),
		Env:     worktreeEnv(),
		WorkDir: dir,
		Up:      map[string]bool{"db": true},
	})

	if len(got.Failed) != 0 {
		t.Fatalf("failed = %v", got.Failed)
	}
	if len(got.Released) != 1 || got.Released[0].Job != "db" {
		t.Errorf("released = %v, want one entry for db", got.Released)
	}
	content, err := os.ReadFile(witness)
	if err != nil {
		t.Fatalf("read witness: %v", err)
	}
	if strings.TrimSpace(string(content)) != "crm_feat_a" {
		t.Errorf("detach ran with %q, want crm_feat_a", content)
	}
}

// Relighting a postgres to drop a database is worse than owing the drop.
func TestDetachWorktreeDefersWhenTheServiceIsDown(t *testing.T) {
	dir := t.TempDir()
	witness := filepath.Join(dir, "witness")

	got := runjobs.RemoveWorktreeNamespaces(t.Context(), runjobs.RemoveNamespacesParams{
		Config:  namespaceConfig("printf 'ran\\n' >> " + witness),
		Env:     worktreeEnv(),
		WorkDir: dir,
		Up:      map[string]bool{},
	})

	if len(got.Released) != 0 {
		t.Errorf("released = %v, want none", got.Released)
	}
	if len(got.Down) != 1 || got.Down[0].Worktree != "feat_a" {
		t.Errorf("down = %v, want one entry for feat_a", got.Down)
	}
	if len(got.Failed) != 0 {
		t.Errorf("failed = %v: a service down is not a failed drop", got.Failed)
	}
	if _, err := os.Stat(witness); !os.IsNotExist(err) {
		t.Error("the removal ran against a service that is down")
	}
}

func TestDetachWorktreeDefersAFailedCommand(t *testing.T) {
	got := runjobs.RemoveWorktreeNamespaces(t.Context(), runjobs.RemoveNamespacesParams{
		Config:  namespaceConfig("exit 3"),
		Env:     worktreeEnv(),
		WorkDir: t.TempDir(),
		Up:      map[string]bool{"db": true},
	})

	if len(got.Failed) != 1 || got.Failed[0].Ref.Job != "db" {
		t.Errorf("failed = %v, want db's drop named with its cause", got.Failed)
	}
	if len(got.Down) != 0 {
		t.Errorf("down = %v: the service was up, and saying otherwise is the wrong cause", got.Down)
	}
	if len(got.Deferred()) != 1 {
		t.Errorf("deferred = %v, want the failed namespace owed rather than lost", got.Deferred())
	}
}

// A drop blocked on a lock nobody releases must not hold the clean for ever.
func TestRemoveGivesUpOnACommandThatNeverReturns(t *testing.T) {
	started := time.Now()
	got := runjobs.RemoveWorktreeNamespaces(t.Context(), runjobs.RemoveNamespacesParams{
		Config:  namespaceConfig("sleep 600; true"),
		Env:     worktreeEnv(),
		WorkDir: t.TempDir(),
		Up:      map[string]bool{"db": true},
		Timeout: 200 * time.Millisecond,
	})

	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the removal took %s, want it cut off at its timeout", elapsed)
	}
	if len(got.Failed) != 1 || !strings.Contains(got.Failed[0].Err.Error(), "timed out") {
		t.Errorf("failed = %v, want the timeout named", got.Failed)
	}
	if len(got.Deferred()) != 1 {
		t.Errorf("deferred = %v, want the namespace owed", got.Deferred())
	}
}

func TestPendingDetachQueueRoundTrips(t *testing.T) {
	stateDir := t.TempDir()
	ref := domain.NamespaceRef{Job: "db", Worktree: "feat_a", Ordinal: 2}

	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: stateDir, Refs: []domain.NamespaceRef{ref}}); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if got := runjobs.LoadPendingRemovals(stateDir); len(got) != 1 || got[0] != ref {
		t.Fatalf("loaded = %v, want %v", got, ref)
	}

	// The same worktree cleaned twice owes one namespace, not two.
	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: stateDir, Refs: []domain.NamespaceRef{ref}}); err != nil {
		t.Fatalf("queue again: %v", err)
	}
	if got := runjobs.LoadPendingRemovals(stateDir); len(got) != 1 {
		t.Errorf("loaded = %v, want one entry", got)
	}

	if err := runjobs.SettleRemovals(runjobs.SettleRemovalsParams{StateDir: stateDir, Refs: []domain.NamespaceRef{ref}}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := runjobs.LoadPendingRemovals(stateDir); len(got) != 0 {
		t.Errorf("loaded = %v, want an empty queue", got)
	}
	if _, err := os.Stat(filepath.Join(stateDir, domain.PendingRemovalsFileName)); !os.IsNotExist(err) {
		t.Error("an empty queue left its file behind")
	}
}

func TestLoadPendingDetachOfAnAbsentFileIsEmpty(t *testing.T) {
	if got := runjobs.LoadPendingRemovals(t.TempDir()); len(got) != 0 {
		t.Errorf("loaded = %v, want none", got)
	}
}
