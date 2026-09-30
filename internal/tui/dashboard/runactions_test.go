package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

// A row is a worktree. Asking again which one, right after it was picked, is
// the question the positional-subject rule exists to not ask.
func TestARunStartedFromARowNamesItsWorktree(t *testing.T) {
	selected := domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}

	named, err := target.Named(target.ResolveParams{
		ProjectDir: t.TempDir(),
		Query:      runWorktree(selected),
	})

	// The repository is empty, so the name resolves to nothing — but it must
	// have been looked up as a name. A path is refused before that, which is the
	// regression this guards.
	if err == nil {
		t.Fatalf("named = %+v, want the empty repository to answer nothing", named)
	}
	if !strings.Contains(err.Error(), selected.Branch) {
		t.Fatalf("error = %v, want the branch quoted: the positional is resolved by name", err)
	}
}

// runModel is a dashboard over a project that has a run module, which is what
// every run action reads before it starts anything.
func runModel(t *testing.T, running ...string) Model {
	t.Helper()
	stateDir := t.TempDir()
	if err := runconfig.Save(runconfig.SaveParams{
		StateDir: stateDir,
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true"}}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	model := New(RunParams{StateDir: stateDir, Cwd: "/tmp/a"})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a", "b"), parents: map[string]string{}})
	for _, branch := range running {
		model.jobs = append(model.jobs, domain.JobInfo{
			Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/" + branch,
		})
	}
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	return model.withBoard()
}

// The batch gesture designates no row: which worktrees it acts on is what the
// run asks, and the operation holds none until the picker answers.
func TestTheBatchStartHoldsNoWorktreeUntilItIsAnswered(t *testing.T) {
	model := runModel(t)

	started, cmd := model.startRunUpAll()

	if cmd == nil {
		t.Fatal("the batch entry must start the run")
	}
	if len(started.ops.running) != 1 || len(started.ops.running[0].targets) != 0 {
		t.Fatalf("running = %+v, want a run holding no worktree yet", started.ops.running)
	}
}

// A stop is about what is standing, so the picker opens on the worktrees that
// have something up — where a start is about where you are, which the step
// already ticks on its own.
func TestTheBatchStopArrivesWithWhatIsRunningTicked(t *testing.T) {
	model := runModel(t, "b")

	if got := model.runningWorktrees(); len(got) != 1 || got[0] != "/tmp/b" {
		t.Fatalf("precheck = %v, want the worktree with something up", got)
	}

	started, cmd := model.startRunDownAll()
	if cmd == nil {
		t.Fatal("the batch stop must start the run")
	}
	if len(started.ops.running) != 1 {
		t.Fatalf("running = %+v, want one run in flight", started.ops.running)
	}
}

// From a row there is nothing to ask — stopping everything there is the safe
// default — and from the global menu there is no row to answer for it.
func TestOnlyTheBatchStopInstallsTheModal(t *testing.T) {
	model := runModel(t)

	if _, asks := model.runPrompter(runPrompterParams{}).(prompter); asks {
		t.Error("the row stop installed the modal, want it unattended: the row already answered")
	}
	if _, asks := model.runPrompter(runPrompterParams{Ask: true}).(prompter); !asks {
		t.Error("the batch stop must ask which worktrees it empties")
	}
}

// A run started with no row still needs run.toml, and refuses the same way a
// row does when the project has no run module.
func TestABatchRunRefusesAProjectWithNoRunModule(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	refused, cmd := model.startRunUpAll()

	if cmd != nil {
		t.Fatal("a project with no run module must start nothing")
	}
	if last := lastOutput(refused); last != domain.DashboardRunNotConfigured {
		t.Errorf("refusal = %q, want %q", last, domain.DashboardRunNotConfigured)
	}
}
