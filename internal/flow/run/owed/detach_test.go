package owed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
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

func TestDetachDropsWhatAServiceUpHolds(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := Read(ReadParams{Context: ctx, Branches: []string{"feat-live"}})
	if len(snapshot.Holdings) != 1 {
		t.Fatalf("holdings = %+v, want feat-live's", snapshot.Holdings)
	}
	snapshot.Up = map[string]bool{"postgres": true}
	presenter := &flowtest.Recorder{}

	Detach(DetachParams{Context: ctx, Presenter: presenter, Snapshot: snapshot})

	if body, _ := os.ReadFile(witness); strings.TrimSpace(string(body)) != "app_feat-live" {
		t.Errorf("remove ran with %q, want app_feat-live", body)
	}
	if len(presenter.Stages) != 1 {
		t.Errorf("stages = %v, want the drop under one", presenter.Stages)
	}
	if left := runjobs.LoadPendingRemovals(ctx.StateDir); len(left) != 0 {
		t.Errorf("queue = %+v, want nothing owed", left)
	}
}

// Kept, the data is owed to the service's next start, and the run says so.
func TestDetachQueuesWhatAServiceDownHolds(t *testing.T) {
	ctx, witness := holdingFixture(t)
	snapshot := Read(ReadParams{Context: ctx, Branches: []string{"feat-live"}})
	snapshot.Up = map[string]bool{}
	presenter := &flowtest.Recorder{}

	Detach(DetachParams{Context: ctx, Presenter: presenter, Snapshot: snapshot})

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q against a service down", body)
	}
	left := runjobs.LoadPendingRemovals(ctx.StateDir)
	if len(left) != 1 || left[0].Worktree != "feat-live" {
		t.Errorf("queue = %+v, want feat-live's namespace owed", left)
	}
	if len(presenter.Statuses) != 1 || presenter.Statuses[0].Kind != flow.NoticeWarning {
		t.Errorf("statuses = %+v, want the deferral said", presenter.Statuses)
	}
}
