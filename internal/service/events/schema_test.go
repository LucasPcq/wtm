package events

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/schemas"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/schematest"
)

// Every type v1 defines, built the way wtm builds it, validates against the
// schema it ships: a type added without its schema fails here.
func TestEveryEventTypeMatchesTheSchema(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.Git(t, dir, "worktree", "add", "-b", "feat/a", filepath.Join(t.TempDir(), "feat-a"))
	stateDir := filepath.Join(dir, ".git", "wtm")
	repo, err := worktree.RepoOf(worktree.RepoOfParams{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := worktree.Identity(worktree.WorktreeRef{ProjectDir: dir, StateDir: stateDir, Branch: "feat/a"})
	if err != nil {
		t.Fatal(err)
	}
	previous := listJobs
	crashed := 1
	listJobs = func(string) ([]domain.JobInfo, error) {
		return []domain.JobInfo{
			{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, State: domain.JobStateRunning, WorkDir: identity.Path, URL: "http://web.feat-a.app.localhost"},
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusCrashed, WorkDir: identity.Path, ExitCode: &crashed},
		}, nil
	}
	t.Cleanup(func() { listJobs = previous })
	snapshot, err := snapshotOf(snapshotParams{ProjectDir: dir, StateDir: stateDir, Repo: repo, Socket: "daemon.sock"})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := readyOf()
	if err != nil {
		t.Fatal(err)
	}
	about := func(event domain.Event) json.RawMessage {
		event.Worktree = &identity
		raw, err := json.Marshal(stamp(stampParams{Event: event, Repo: repo}))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	failed, passed, exitCode := false, true, 3
	ofRepo := func(typ domain.EventType) json.RawMessage {
		raw, err := json.Marshal(stamp(stampParams{Event: domain.Event{Type: typ}, Repo: repo}))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	zero := 0
	ofJob := func(event domain.JobEvent) json.RawMessage {
		event.V = domain.EventsSchemaVersion
		event.TS = now()
		event.Repo = repo
		event.Worktree = domain.WorktreeRef{Branch: "feat/a", Path: identity.Path}
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	examples := map[domain.EventType]json.RawMessage{
		domain.EventRepoAdded:             ofRepo(domain.EventRepoAdded),
		domain.EventRepoRemoved:           ofRepo(domain.EventRepoRemoved),
		domain.EventSnapshot:              snapshot.Raw,
		domain.EventReady:                 ready.Raw,
		domain.EventWorktreeCreated:       about(domain.Event{Type: domain.EventWorktreeCreated, CorrelationID: "popup-1"}),
		domain.EventWorktreeProvisioned:   about(domain.Event{Type: domain.EventWorktreeProvisioned, OK: &failed, Hook: "pnpm install", ExitCode: &exitCode}),
		domain.EventWorktreeUpdated:       about(domain.Event{Type: domain.EventWorktreeUpdated, Changed: []domain.IdentityField{domain.IdentityOrdinal}}),
		domain.EventWorktreeRelocated:     about(domain.Event{Type: domain.EventWorktreeRelocated, FromPath: "/old/feat-a"}),
		domain.EventWorktreeReparented:    about(domain.Event{Type: domain.EventWorktreeReparented, FromParent: "main"}),
		domain.EventWorktreeDeprovisioned: about(domain.Event{Type: domain.EventWorktreeDeprovisioned, OK: &passed}),
		domain.EventWorktreeRemoved:       about(domain.Event{Type: domain.EventWorktreeRemoved}),
		domain.EventJobStarted:            ofJob(domain.JobEvent{Type: domain.EventJobStarted, Job: domain.EventJob{Name: "web", Kind: domain.JobKindService, URL: "http://web.feat-a.app.localhost"}}),
		domain.EventJobExited:             ofJob(domain.JobEvent{Type: domain.EventJobExited, Job: domain.EventJob{Name: "migrate", Kind: domain.JobKindTask}, ExitCode: &zero}),
		domain.EventJobStopped:            ofJob(domain.JobEvent{Type: domain.EventJobStopped, CorrelationID: "popup-2", Job: domain.EventJob{Name: "web", Kind: domain.JobKindService}}),
		domain.EventJobCrashed:            ofJob(domain.JobEvent{Type: domain.EventJobCrashed, Job: domain.EventJob{Name: "web", Kind: domain.JobKindService}, ExitCode: &exitCode, LastLines: []string{"Error: boom"}}),
	}
	schema := schematest.Compile(t, schemas.Events)

	for _, typ := range domain.EventTypes {
		example, ok := examples[typ]
		if !ok {
			t.Errorf("%s has no example: add one here", typ)
			continue
		}
		if err := schematest.Validate(t, schema, example); err != nil {
			t.Errorf("%s does not match events.v1.json: %v\n%s", typ, err, example)
		}
	}
}
