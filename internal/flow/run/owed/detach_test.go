package owed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func snapshotWith(up bool) Snapshot {
	job := domain.JobConfig{
		Name: "postgres", Kind: domain.JobKindService, Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: "true"},
	}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{job}}
	return Snapshot{
		Config:   cfg,
		Holdings: []domain.NamespaceHolding{{Branch: "feat", Env: map[string]string{domain.EnvWorktree: "feat"}, Config: cfg}},
		Up:       map[string]bool{"postgres": up},
	}
}

func dataStep(snapshot Snapshot, keepData bool) flow.Step {
	return DataStep(DataStepParams{
		Key:      "data",
		KeepData: keepData,
		Snapshot: func(flow.Answers) Snapshot { return snapshot },
	})
}

// A service that is up takes its data back without a question, and a removal
// that holds nothing has nothing to ask.
func TestDataStepIsOnlyAskedWhenAServiceIsDown(t *testing.T) {
	cases := map[string]struct {
		snapshot Snapshot
		keepData bool
		asked    bool
	}{
		"service down":        {snapshot: snapshotWith(false), asked: true},
		"service up":          {snapshot: snapshotWith(true)},
		"nothing held":        {snapshot: Snapshot{}},
		"--keep-data answers": {snapshot: snapshotWith(false), keepData: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			skip, reason := dataStep(c.snapshot, c.keepData).Skip(flow.NewAnswers(nil))
			if skip == c.asked {
				t.Errorf("skip = %v, want asked = %v", skip, c.asked)
			}
			if reason != "" {
				t.Errorf("reason = %q: a step with nothing to ask leaves no line behind", reason)
			}
		})
	}
}

// Starting a service nobody asked for is not a safe default.
func TestAnUnattendedRunKeepsTheDataOfAServiceDown(t *testing.T) {
	answer, err := dataStep(snapshotWith(false), false).Resolve(flow.NewAnswers(nil))
	if err != nil || answer.Value != DataDefer {
		t.Errorf("answer = %+v, %v — want %q", answer, err, DataDefer)
	}
}

func TestDataStepOffersToStartTheServicesDown(t *testing.T) {
	content, err := dataStep(snapshotWith(false), false).Build(flow.NewAnswers(nil))
	if err != nil {
		t.Fatal(err)
	}
	if content.Options[0].Value != DataStart || content.Options[0].Label != "Start postgres and drop it now" {
		t.Errorf("first option = %+v, want to start postgres", content.Options[0])
	}
}

// holdingFixture is the owed fixture's live worktree, recorded as holding a
// namespace in postgres.
func holdingFixture(t *testing.T) (flow.Context, string) {
	t.Helper()
	ctx, witness := fixture(t)
	metaDir := filepath.Join(ctx.StateDir, "worktrees", "feat-live")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte(`{"source_branch":"main","ordinal":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := worktree.RecordNamespaces(worktree.RecordNamespacesParams{StateDir: ctx.StateDir, Branch: "feat-live", Jobs: []string{"postgres"}}); err != nil {
		t.Fatal(err)
	}
	if err := runjobs.SettleRemovals(runjobs.SettleRemovalsParams{StateDir: ctx.StateDir, Refs: runjobs.LoadPendingRemovals(ctx.StateDir)}); err != nil {
		t.Fatal(err)
	}
	return ctx, witness
}

// removeWorktree takes feat-live away the way a clean does before its data is
// dropped, so a drop that still needed its directory would fail here.
func removeWorktree(t *testing.T, ctx flow.Context) {
	t.Helper()
	wt, err := worktree.FindByBranch(t.Context(), worktree.FindByBranchParams{ProjectDir: ctx.ProjectDir, Branch: "feat-live"})
	if err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, ctx.ProjectDir, "worktree", "remove", "--force", wt.Path)
}

func readHolding(t *testing.T, ctx flow.Context, up bool) Snapshot {
	t.Helper()
	snapshot := Read(t.Context(), ReadParams{Context: ctx, Branches: []string{"feat-live"}})
	if len(snapshot.Holdings) != 1 {
		t.Fatalf("holdings = %+v, want feat-live's", snapshot.Holdings)
	}
	snapshot.Up = map[string]bool{"postgres": up}
	return snapshot
}

func TestDropperDropsWhatAServiceUpHoldsOnceTheWorktreeIsGone(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := readHolding(t, ctx, true)
	removeWorktree(t, ctx)
	presenter := &flowtest.Recorder{}

	dropper := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot})
	outcomes := dropper.Drop(t.Context(), "feat-live")
	dropper.Close()

	if body, _ := os.ReadFile(witness); strings.TrimSpace(string(body)) != "app_feat-live" {
		t.Errorf("remove ran with %q, want app_feat-live", body)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDropped || outcomes[0].Branch != "feat-live" || outcomes[0].Job != "postgres" {
		t.Errorf("outcomes = %+v, want app_feat-live dropped from postgres", outcomes)
	}
	if left := runjobs.LoadPendingRemovals(ctx.StateDir); len(left) != 0 {
		t.Errorf("queue = %+v, want nothing owed", left)
	}
}

// Kept, the data is owed to the service's next start, and the run says so.
func TestDropperQueuesWhatAServiceDownHolds(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := readHolding(t, ctx, false)
	presenter := &flowtest.Recorder{}

	outcomes := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot}).Drop(t.Context(), "feat-live")

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q against a service down", body)
	}
	left := runjobs.LoadPendingRemovals(ctx.StateDir)
	if len(left) != 1 || left[0].Worktree != "feat-live" {
		t.Errorf("queue = %+v, want feat-live's namespace owed", left)
	}
	if len(presenter.Statuses) != 1 || !strings.Contains(presenter.Statuses[0].Text, "postgres is down") {
		t.Errorf("statuses = %+v, want the deferral said", presenter.Statuses)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDeferred {
		t.Errorf("outcomes = %+v, want it deferred", outcomes)
	}
}

// A service up that refused the drop is not down, and saying so sends the
// reader after the wrong cause.
func TestDropperNamesTheCauseOfADropRefusedByAServiceUp(t *testing.T) {
	ctx, _ := holdingFixture(t)
	snapshot := readHolding(t, ctx, true)
	snapshot.Holdings[0].Config.Jobs[0].Namespace.Remove = "echo 'database is being accessed by other users' >&2; exit 1"
	presenter := &flowtest.Recorder{}

	outcomes := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot}).Drop(t.Context(), "feat-live")

	if len(presenter.Statuses) != 1 {
		t.Fatalf("statuses = %+v, want one line", presenter.Statuses)
	}
	text := presenter.Statuses[0].Text
	if strings.Contains(text, "is down") || !strings.Contains(text, "being accessed by other users") {
		t.Errorf("status = %q, want the real cause", text)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDeferred || !strings.Contains(outcomes[0].Reason, "being accessed") {
		t.Errorf("outcomes = %+v, want it deferred with its cause", outcomes)
	}
	if left := runjobs.LoadPendingRemovals(ctx.StateDir); len(left) != 1 {
		t.Errorf("queue = %+v, want the namespace owed", left)
	}
}

// A debt left by an earlier attempt is paid by the drop that succeeds.
func TestDropperSettlesAnOlderDebtForTheNamespaceItDrops(t *testing.T) {
	ctx, _ := holdingFixture(t)
	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: ctx.StateDir, Refs: []domain.NamespaceRef{
		{Job: "postgres", Worktree: "feat-live", Ordinal: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	snapshot := readHolding(t, ctx, true)

	NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: &flowtest.Recorder{}, Snapshot: snapshot}).Drop(t.Context(), "feat-live")

	if left := runjobs.LoadPendingRemovals(ctx.StateDir); len(left) != 0 {
		t.Errorf("queue = %+v, want the old debt settled", left)
	}
}

func TestDropperWithKeepDataRunsNothing(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := readHolding(t, ctx, true)
	presenter := &flowtest.Recorder{}

	outcomes := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot, KeepData: true}).Drop(t.Context(), "feat-live")

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q under --keep-data", body)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceKept || outcomes[0].Name != "app_feat-live" {
		t.Errorf("outcomes = %+v, want app_feat-live kept", outcomes)
	}
	if len(presenter.Stages)+len(presenter.Statuses) != 0 {
		t.Errorf("presented %v %v, want nothing", presenter.Stages, presenter.Statuses)
	}
}

// feat/live and feat-live reduce to the same slug, so the namespace is both
// of theirs: cleaning one must not drop the other's database.
func TestDropperKeepsANamespaceAnotherWorktreeSharesByItsSlug(t *testing.T) {
	ctx, witness := holdingFixture(t)
	gittest.Git(t, ctx.ProjectDir, "worktree", "add", "-b", "feat/live", filepath.Join(t.TempDir(), "feat-slash-live"))
	snapshot := readHolding(t, ctx, true)
	if got := snapshot.Held()[0].SharedWith; got != "feat/live" {
		t.Fatalf("shared with = %q, want feat/live", got)
	}
	presenter := &flowtest.Recorder{}

	outcomes := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot}).Drop(t.Context(), "feat-live")

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q on a namespace feat/live still uses", body)
	}
	if len(presenter.Statuses) != 1 || !strings.Contains(presenter.Statuses[0].Text, "feat/live") {
		t.Errorf("statuses = %+v, want the collision named", presenter.Statuses)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceKept {
		t.Errorf("outcomes = %+v, want it kept", outcomes)
	}
	if left := runjobs.LoadPendingRemovals(ctx.StateDir); len(left) != 0 {
		t.Errorf("queue = %+v: a namespace still in use is owed to nobody", left)
	}
}

// A service down is started from the main checkout to take its data back, and
// let go once it has.
func TestDropperStartsAServiceDownAndLetsItGoAfterwards(t *testing.T) {
	ctx, witness := holdingFixture(t)
	daemon := processtest.Serve(t, nil)
	snapshot := readHolding(t, ctx, false)
	presenter := &flowtest.Recorder{}

	dropper := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot, StartDown: true})
	outcomes := dropper.Drop(t.Context(), "feat-live")
	dropper.Close()

	if body, _ := os.ReadFile(witness); strings.TrimSpace(string(body)) != "app_feat-live" {
		t.Errorf("remove ran with %q, want app_feat-live once postgres was started", body)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDropped {
		t.Errorf("outcomes = %+v, want it dropped", outcomes)
	}
	main, err := worktree.MainCheckout(t.Context(), worktree.MainCheckoutParams{ProjectDir: ctx.ProjectDir})
	if err != nil {
		t.Fatal(err)
	}
	if want := "start:@" + main + " stop:postgres@" + main; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want postgres started then let go in the main checkout", daemon.Actions())
	}
}

// A service that cannot be started leaves its data owed, and says why.
func TestDropperDefersWhatAServiceThatWouldNotStartHolds(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := readHolding(t, ctx, false)
	snapshot.Config = domain.RunConfig{}
	presenter := &flowtest.Recorder{}

	outcomes := NewDropper(t.Context(), DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot, StartDown: true}).Drop(t.Context(), "feat-live")

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q against a service that never started", body)
	}
	if len(presenter.Statuses) == 0 || !strings.Contains(presenter.Statuses[0].Text, domain.ErrJobNotFound.Error()) {
		t.Errorf("statuses = %+v, want the start failure said", presenter.Statuses)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDeferred {
		t.Errorf("outcomes = %+v, want it deferred", outcomes)
	}
}
