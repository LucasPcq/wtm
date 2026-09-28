package runlogs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var sharedPostgres = domain.JobConfig{
	Name:  "postgres",
	Kind:  domain.JobKindService,
	Cmd:   "docker compose up postgres",
	Scope: domain.JobScopeShared,
	Namespace: &domain.JobNamespaceConfig{
		Name:   "app_{worktree}",
		Create: "createdb $WTM_NAMESPACE",
	},
}

func runShared(t *testing.T, service *runlogstest.Service) (*runlogstest.Sink, runlogs.Outcome) {
	t.Helper()
	sink := &runlogstest.Sink{}
	outcome, err := runlogs.Run(t.Context(), runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    []domain.JobConfig{sharedPostgres},
		WorkDir: "/trees/feat-x",
		Env:     map[string]string{domain.EnvWorktree: "feat_x", domain.EnvOrdinal: "1"},
		Shared:  &domain.SharedJobContext{WorkDir: "/main"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return sink, outcome
}

// The one thing a clean will drop is the slice a start carved out, so the start
// says it made it — by name, and apart from the service it lives in.
func TestRunNamesTheNamespaceASharedStartCarved(t *testing.T) {
	sink, _ := runShared(t, &runlogstest.Service{})

	started, found := sink.Last(runlogs.PhaseStarted)
	if !found {
		t.Fatal("no started event")
	}
	if !started.Attached || started.Namespace != "app_feat_x" {
		t.Errorf("started = %+v, want postgres attached with app_feat_x carved", started)
	}
}

// A worktree already holding the service ran no create this time: naming the
// namespace would claim work that was not done.
func TestRunNamesNoNamespaceWhenTheServiceWasAlreadyHeld(t *testing.T) {
	sink, _ := runShared(t, &runlogstest.Service{
		Refusals: map[string]string{"postgres": "job postgres " + domain.JobAlreadyRunningSuffix},
	})

	started, found := sink.Last(runlogs.PhaseStarted)
	if !found {
		t.Fatal("no started event")
	}
	if started.Namespace != "" {
		t.Errorf("namespace = %q, want none for a start that ran no create", started.Namespace)
	}
}

// A linked worktree holding main's service is told where it runs.
func TestRunSaysWhereAnAttachedServiceRuns(t *testing.T) {
	sink := &runlogstest.Sink{}
	if _, err := runlogs.Run(t.Context(), runlogs.RunParams{
		Service:        &runlogstest.Service{},
		Sink:           sink,
		Jobs:           []domain.JobConfig{sharedPostgres},
		WorkDir:        "/trees/feat-x",
		Worktree:       "feat/x",
		Env:            map[string]string{domain.EnvWorktree: "feat_x", domain.EnvOrdinal: "1"},
		Shared:         &domain.SharedJobContext{WorkDir: "/main"},
		SharedWorktree: "main",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	started, _ := sink.Last(runlogs.PhaseStarted)
	if started.SharedIn != "main" {
		t.Errorf("shared in = %q, want main", started.SharedIn)
	}
}
