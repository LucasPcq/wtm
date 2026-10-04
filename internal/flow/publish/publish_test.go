package publish_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type fixture struct {
	ctx  flow.Context
	rec  *flowtest.Recorder
	path string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := gittest.InitRepo(t)
	path := filepath.Join(t.TempDir(), "feat-a")
	gittest.Git(t, dir, "worktree", "add", "-b", "feat/a", path)
	rec := &flowtest.Recorder{}
	return fixture{
		ctx:  flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Publisher: rec},
		rec:  rec,
		path: path,
	}
}

func TestCreatedCarriesTheWorktreeAsItNowIs(t *testing.T) {
	f := newFixture(t)

	publish.Created(f.ctx, "feat/a")

	if !slices.Equal(f.rec.PublishedTypes(), []domain.EventType{domain.EventWorktreeCreated}) {
		t.Fatalf("published %v", f.rec.PublishedTypes())
	}
	got := f.rec.Published[0]
	if got.Worktree == nil || got.Worktree.Branch != "feat/a" {
		t.Fatalf("worktree = %+v", got.Worktree)
	}
	if got.Repo != nil || got.TS != "" || got.V != 0 {
		t.Fatalf("a flow publishes intent, the publisher stamps it: %+v", got)
	}
}

func TestEachEventCarriesTheFieldItIsAbout(t *testing.T) {
	f := newFixture(t)

	publish.Updated(publish.UpdatedParams{Context: f.ctx, Branch: "feat/a", Changed: []domain.IdentityField{domain.IdentityIsolation}})
	publish.Relocated(publish.RelocatedParams{Context: f.ctx, Branch: "feat/a", FromPath: "/old"})
	publish.Reparented(publish.ReparentedParams{Context: f.ctx, Branch: "feat/a", FromParent: "feat/x"})

	got := f.rec.Published
	if len(got) != 3 {
		t.Fatalf("published %v", f.rec.PublishedTypes())
	}
	if !slices.Equal(got[0].Changed, []domain.IdentityField{domain.IdentityIsolation}) || got[1].FromPath != "/old" || got[2].FromParent != "feat/x" {
		t.Fatalf("published %+v", got)
	}
}

func TestRemovedCarriesTheStateCapturedBeforeTheRemoval(t *testing.T) {
	f := newFixture(t)

	last, ok := publish.Capture(f.ctx, "feat/a")
	if !ok {
		t.Fatal("capture failed")
	}
	gittest.Git(t, f.ctx.ProjectDir, "worktree", "remove", f.path)
	publish.Removed(f.ctx, last)

	if len(f.rec.Published) != 1 || f.rec.Published[0].Type != domain.EventWorktreeRemoved || f.rec.Published[0].Worktree.Branch != "feat/a" {
		t.Fatalf("published %+v", f.rec.Published)
	}
}

func TestAWorktreeThatCannotBeReadPublishesNothing(t *testing.T) {
	f := newFixture(t)

	publish.Created(f.ctx, "nope")
	if _, ok := publish.Capture(f.ctx, "nope"); ok {
		t.Fatal("captured a worktree that does not exist")
	}

	if len(f.rec.Published) != 0 {
		t.Fatalf("published %v", f.rec.PublishedTypes())
	}
}

func TestNoPublisherReadsNothing(t *testing.T) {
	f := newFixture(t)
	f.ctx.Publisher = nil

	publish.Created(f.ctx, "feat/a")
	if _, ok := publish.Capture(f.ctx, "feat/a"); ok {
		t.Fatal("captured with nobody to publish to")
	}
}

func TestNobodyListeningReadsNothing(t *testing.T) {
	f := newFixture(t)
	f.rec.Unheard = true

	publish.Created(f.ctx, "feat/a")
	if _, ok := publish.Capture(f.ctx, "feat/a"); ok {
		t.Fatal("captured with nobody listening")
	}
	if len(f.rec.Published) != 0 {
		t.Fatalf("published %v with nobody listening", f.rec.PublishedTypes())
	}
}
