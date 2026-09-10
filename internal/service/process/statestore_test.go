package process

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func detachedRecord(t *testing.T, workDir string) domain.JobRecord {
	t.Helper()
	return domain.JobRecord{
		Name:    "db",
		WorkDir: workDir,
		Config: domain.JobConfig{
			Name: "db",
			Kind: domain.JobKindService,
			Cmd:  "docker compose up -d",
			Stop: "docker compose down",
		},
		Env: map[string]string{"COMPOSE_PROJECT_NAME": "wtm-feature"},
	}
}

func TestStateStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	store := NewStateStore(path)

	if err := store.Save([]domain.JobRecord{detachedRecord(t, "/wt/feature")}); err != nil {
		t.Fatalf("save: %v", err)
	}

	records := NewStateStore(path).Load()
	if len(records) != 1 {
		t.Fatalf("loaded %d records, want 1", len(records))
	}
	if records[0].Env["COMPOSE_PROJECT_NAME"] != "wtm-feature" {
		t.Fatal("the env must survive: a stop that loses it dismantles the wrong project")
	}
	if records[0].Config.Stop != "docker compose down" {
		t.Fatalf("stop command lost: %+v", records[0].Config)
	}
}

func TestStateStoreLoadsNothingFromAbsentOrCorruptFile(t *testing.T) {
	dir := t.TempDir()

	if records := NewStateStore(filepath.Join(dir, "missing.json")).Load(); records != nil {
		t.Fatal("an absent index is an empty one, never an error")
	}

	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{ half-writ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if records := NewStateStore(corrupt).Load(); records != nil {
		t.Fatal("a truncated index must read as empty: a daemon refusing to start over its own bookkeeping is worse")
	}
}

func TestStateStoreNeverOverwritesAnUnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	future := []byte(`{"version":99,"jobs":[{"name":"db","work_dir":"/wt/x"}]}`)
	if err := os.WriteFile(path, future, 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewStateStore(path)
	if records := store.Load(); records != nil {
		t.Fatal("an unknown format must read as empty")
	}
	if !store.Frozen() {
		t.Fatal("a newer binary's index freezes the store, which is what a command has to be able to report")
	}
	if err := store.Save([]domain.JobRecord{detachedRecord(t, "/wt/feature")}); err != nil {
		t.Fatalf("save: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.DaemonState
	if err := json.Unmarshal(after, &state); err != nil {
		t.Fatal(err)
	}
	if state.Version != 99 {
		t.Fatal("an older binary must not destroy the index of a newer one")
	}
}

// The bug LUC-229 fixed: a stale file from an older format froze the store, so
// every Save became a no-op and the daemon silently recorded nothing at all.
func TestStateStoreReplacesAnIndexFromAnOlderFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"jobs":[]}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	store := NewStateStore(path)
	if got := store.Load(); len(got) != 0 {
		t.Fatalf("loaded %d records from a format this binary does not read", len(got))
	}
	if store.Frozen() {
		t.Fatal("an older format must not freeze the store: that is how a daemon stops recording anything")
	}

	if err := store.Save([]domain.JobRecord{detachedRecord(t, "/wt/feature")}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := NewStateStore(path).Load(); len(got) != 1 {
		t.Fatalf("reloaded %d records, want the one just saved over the stale file", len(got))
	}
}
