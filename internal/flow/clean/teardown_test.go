package clean

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// dataFixture is a worktree holding a namespace in a shared postgres that is
// up, whose remove writes to the witness what it dropped — or "present" when
// the worktree's directory was still there, which a drop must never see.
type dataFixture struct {
	ctx     flow.Context
	branch  string
	path    string
	witness string
	daemon  *processtest.Daemon
}

func newDataFixture(t *testing.T) dataFixture {
	t.Helper()
	globaldir.Isolate(t)
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/data")
	wt, err := worktree.FindByBranch(t.Context(), worktree.FindByBranchParams{ProjectDir: ctx.ProjectDir, Branch: "feat/data"})
	if err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(t.TempDir(), "witness")
	remove := `if [ -d "` + wt.Path + `" ]; then echo present; else echo "$WTM_NAMESPACE"; fi >> ` + witness
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "postgres", Kind: domain.JobKindService, Cmd: "true", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: remove},
	}}}}); err != nil {
		t.Fatal(err)
	}
	if err := worktree.RecordNamespaces(worktree.RecordNamespacesParams{StateDir: ctx.StateDir, Branch: "feat/data", Jobs: []string{"postgres"}}); err != nil {
		t.Fatal(err)
	}
	up := []domain.JobInfo{{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: ctx.ProjectDir}}
	return dataFixture{ctx: ctx, branch: "feat/data", path: wt.Path, witness: witness, daemon: processtest.Serve(t, up)}
}

func (d dataFixture) run(t *testing.T, request Request) (Outcome, *recorder, error) {
	t.Helper()
	request.Branches = []string{d.branch}
	request.BaseBranch = "main"
	presenter := newRecorder()
	outcome, err := Run(t.Context(), Params{
		Context:   d.ctx,
		Request:   request,
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: presenter,
	})
	return outcome, presenter, err
}

func (d dataFixture) dropped(t *testing.T) string {
	t.Helper()
	body, _ := os.ReadFile(d.witness)
	return strings.TrimSpace(string(body))
}

func (d dataFixture) worktreeExists() bool {
	_, err := os.Stat(d.path)
	return err == nil
}

// The data goes last: a hook that fails leaves the worktree, and a worktree
// left behind must still find its database — the next `run up` would otherwise
// recreate it empty.
func TestAFailingHookLeavesTheDataIntact(t *testing.T) {
	d := newDataFixture(t)
	d.ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "exit 3"}}

	_, _, err := d.run(t, Request{})

	if err == nil {
		t.Fatal("a failing on_clean hook must fail the clean")
	}
	if !d.worktreeExists() {
		t.Error("the worktree was removed despite its hook failing")
	}
	if got := d.dropped(t); got != "" {
		t.Errorf("remove ran (%q) before the worktree was gone", got)
	}
	if left := runjobs.LoadPendingRemovals(d.ctx.StateDir); len(left) != 0 {
		t.Errorf("queue = %+v: a worktree still there owes nothing", left)
	}
}

func TestTheDataIsDroppedOnceTheWorktreeIsGone(t *testing.T) {
	d := newDataFixture(t)

	outcome, _, err := d.run(t, Request{})
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if got := d.dropped(t); got != "app_feat-data" {
		t.Errorf("remove saw %q, want app_feat-data dropped after the worktree went", got)
	}
	want := domain.NamespaceOutcome{Branch: "feat/data", Job: "postgres", Name: "app_feat-data", Status: domain.NamespaceDropped}
	if len(outcome.Namespaces) != 1 || outcome.Namespaces[0] != want {
		t.Errorf("namespaces = %+v, want %+v", outcome.Namespaces, want)
	}
}

// The API is stopped before the drop, so the drop never meets its session; its
// claim on postgres goes last, so the service is still up to take the drop.
func TestTheJobsStopFirstAndTheClaimsGoLast(t *testing.T) {
	d := newDataFixture(t)
	d.daemon = processtest.Serve(t, []domain.JobInfo{
		{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: d.ctx.ProjectDir},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: d.path},
		{Name: "postgres", Status: domain.JobStatusJoined, WorkDir: d.path},
	})

	if _, _, err := d.run(t, Request{}); err != nil {
		t.Fatalf("clean: %v", err)
	}
	want := "stop:api@" + d.path + " stop_all:@" + d.path
	if got := strings.Join(d.daemon.Actions(), " "); got != want {
		t.Errorf("requests = %q, want %q", got, want)
	}
	if got := d.dropped(t); got != "app_feat-data" {
		t.Errorf("remove saw %q, want the drop between the two", got)
	}
}

// A job that would not stop refuses the removal, naming why, and touches
// nothing: the worktree, its data and its hooks are all left alone.
func TestAJobThatWouldNotStopRefusesTheRemoval(t *testing.T) {
	d := newDataFixture(t)
	d.daemon = processtest.Serve(t, []domain.JobInfo{
		{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: d.ctx.ProjectDir},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: d.path},
	})
	d.daemon.StopError = "the daemon holding the socket is v0.27.0 — run 'wtm run daemon restart'"
	d.ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "true"}}

	_, presenter, err := d.run(t, Request{})

	if err == nil || !strings.Contains(err.Error(), "wtm run daemon restart") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v, want the cause and the way past it", err)
	}
	if !d.worktreeExists() || d.dropped(t) != "" || len(presenter.Hooks) != 0 {
		t.Errorf("exists=%v dropped=%q hooks=%v, want nothing touched", d.worktreeExists(), d.dropped(t), presenter.Hooks)
	}
}

func TestForceRemovesAWorktreeWhoseJobsWouldNotStop(t *testing.T) {
	d := newDataFixture(t)
	d.daemon = processtest.Serve(t, []domain.JobInfo{
		{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: d.ctx.ProjectDir},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: d.path},
	})
	d.daemon.Survive = true

	_, presenter, err := d.run(t, Request{Force: true})

	if err != nil {
		t.Fatalf("clean --force: %v", err)
	}
	if d.worktreeExists() {
		t.Error("--force must remove the worktree anyway")
	}
	if !hasStatus(presenter, "--force") {
		t.Errorf("statuses = %+v, want the forced stop said", presenter.Statuses)
	}
}

// Another live worktree reduces to the same slug: the namespace is its too.
func TestASlugCollisionKeepsTheNamespace(t *testing.T) {
	d := newDataFixture(t)
	gittest.Git(t, d.ctx.ProjectDir, "worktree", "add", "-b", "feat.data", filepath.Join(t.TempDir(), "feat.data"))

	outcome, presenter, err := d.run(t, Request{})

	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if got := d.dropped(t); got != "" {
		t.Errorf("remove ran (%q) on a namespace feat.data still uses", got)
	}
	if !hasStatus(presenter, "feat.data") {
		t.Errorf("statuses = %+v, want the collision named", presenter.Statuses)
	}
	if len(outcome.Namespaces) != 1 || outcome.Namespaces[0].Status != domain.NamespaceKept {
		t.Errorf("namespaces = %+v, want it kept", outcome.Namespaces)
	}
}

// git drops its entry even when files it could not delete are left: the rest
// of the removal follows, and the leftover directory is named.
func TestAHalfRemovedWorktreeIsFinishedAndItsLeftoverNamed(t *testing.T) {
	d := newDataFixture(t)
	locked := filepath.Join(d.path, "root-owned")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "pgdata"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	outcome, presenter, err := d.run(t, Request{Force: true})

	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if worktree.StillTracked(t.Context(), worktree.FindByBranchParams{ProjectDir: d.ctx.ProjectDir, Branch: d.branch}) {
		t.Error("git still tracks the worktree")
	}
	if out, _ := exec.Command("git", "-C", d.ctx.ProjectDir, "branch", "--list", d.branch).Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("branch %s kept after its worktree went", d.branch)
	}
	if !hasStatus(presenter, "sudo rm -rf "+d.path) {
		t.Errorf("statuses = %+v, want the leftover named with the way to delete it", presenter.Statuses)
	}
	if len(outcome.Namespaces) != 1 || outcome.Namespaces[0].Status != domain.NamespaceDropped {
		t.Errorf("namespaces = %+v, want the data dropped with the worktree", outcome.Namespaces)
	}
}

// A locked worktree is refused by git before anything is deleted: that failure
// removed nothing, so the data stays.
func TestARemovalGitRefusedKeepsTheData(t *testing.T) {
	d := newDataFixture(t)
	gittest.Git(t, d.ctx.ProjectDir, "worktree", "lock", d.path)

	_, _, err := d.run(t, Request{})

	if !errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		t.Fatalf("err = %v, want the removal failure", err)
	}
	if !d.worktreeExists() || d.dropped(t) != "" {
		t.Errorf("exists=%v dropped=%q, want both untouched", d.worktreeExists(), d.dropped(t))
	}
}

func hasStatus(presenter *recorder, fragment string) bool {
	for _, status := range presenter.Statuses {
		if strings.Contains(status.Text, fragment) {
			return true
		}
	}
	return false
}
