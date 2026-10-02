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
	snapshot, err := snapshotEvent(snapshotParams{ProjectDir: dir, StateDir: stateDir, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	about := func(event domain.Event) domain.Event {
		event.Worktree = &identity
		return stamp(stampParams{Event: event, Repo: repo})
	}
	examples := map[domain.EventType]domain.Event{
		domain.EventSnapshot:           snapshot,
		domain.EventReady:              readyEvent(),
		domain.EventWorktreeCreated:    about(domain.Event{Type: domain.EventWorktreeCreated}),
		domain.EventWorktreeUpdated:    about(domain.Event{Type: domain.EventWorktreeUpdated, Changed: []domain.IdentityField{domain.IdentityOrdinal}}),
		domain.EventWorktreeRelocated:  about(domain.Event{Type: domain.EventWorktreeRelocated, FromPath: "/old/feat-a"}),
		domain.EventWorktreeReparented: about(domain.Event{Type: domain.EventWorktreeReparented, FromParent: "main"}),
		domain.EventWorktreeRemoved:    about(domain.Event{Type: domain.EventWorktreeRemoved}),
	}
	schema := schematest.Compile(t, schemas.Events)

	for _, typ := range domain.EventTypes {
		example, ok := examples[typ]
		if !ok {
			t.Errorf("%s has no example: add one here", typ)
			continue
		}
		doc, err := json.Marshal(example)
		if err != nil {
			t.Fatal(err)
		}
		if err := schematest.Validate(t, schema, doc); err != nil {
			t.Errorf("%s does not match events.v1.json: %v\n%s", typ, err, doc)
		}
	}
}
