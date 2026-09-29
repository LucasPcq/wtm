package foreigndata_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/foreigndata"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

var resetConfig = domain.RunConfig{Jobs: []domain.JobConfig{
	{Name: "pg", Kind: domain.JobKindService, Cmd: "docker compose up -d pg", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true"}},
	{Name: "reset", Kind: domain.JobKindTask, Cmd: "pnpm reset", Touches: []string{"pg"}},
}}

// verbatimWorktree is a repository with one linked worktree created verbatim:
// its .env names main's database, so a reset run there resets main's.
func verbatimWorktree(t *testing.T) (flow.Context, string) {
	t.Helper()
	t.Setenv(domain.EnvComposeProjectName, "")
	repo := gittest.InitRepo(t)
	stateDir := filepath.Join(repo, ".git", "wtm")
	path := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", path)
	if err := worktree.SetIsolation(worktree.SetIsolationParams{
		Ref:       worktree.WorktreeRef{ProjectDir: repo, StateDir: stateDir, Branch: "feature"},
		Isolation: domain.IsolationVerbatim,
	}); err != nil {
		t.Fatal(err)
	}
	return flow.Context{ProjectDir: repo, StateDir: stateDir}, path
}

func params(ctx flow.Context, dir string, prompter flow.Prompter) foreigndata.Params {
	return foreigndata.Params{Context: ctx, Config: resetConfig, Jobs: resetConfig.Jobs, WorkDirs: []string{dir}, Prompter: prompter}
}

func TestAResetOnTheSourcesDataIsRefusedWithNobodyToAsk(t *testing.T) {
	ctx, dir := verbatimWorktree(t)

	proceed, err := foreigndata.Allow(params(ctx, dir, flow.Unattended{}))
	if proceed || err == nil {
		t.Fatalf("Allow = (%v, %v), want a refusal", proceed, err)
	}
	for _, want := range []string{"reset changes pg", "--" + domain.FlagForce} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name %q", err, want)
		}
	}
}

func TestAResetOnTheSourcesDataIsAskedOnATerminal(t *testing.T) {
	ctx, dir := verbatimWorktree(t)

	declined := &flowtest.ScriptedPrompter{}
	if proceed, err := foreigndata.Allow(params(ctx, dir, declined)); proceed || err != nil || declined.Confirms != 1 {
		t.Errorf("declined: Allow = (%v, %v) after %d confirm(s), want one question answered no", proceed, err, declined.Confirms)
	}
	accepted := &flowtest.ScriptedPrompter{Confirmed: true}
	if proceed, err := foreigndata.Allow(params(ctx, dir, accepted)); !proceed || err != nil {
		t.Errorf("accepted: Allow = (%v, %v), want the run to go on", proceed, err)
	}
}

// --force is the safety axis: it lifts the refusal and asks nothing.
func TestForceLiftsTheRefusal(t *testing.T) {
	ctx, dir := verbatimWorktree(t)
	forced := params(ctx, dir, flow.Unattended{})
	forced.Force = true

	if proceed, err := foreigndata.Allow(forced); !proceed || err != nil {
		t.Errorf("Allow = (%v, %v), want --force to let the run through", proceed, err)
	}
}

// Main owns its data: a reset there is its own business.
func TestMainIsNeverStopped(t *testing.T) {
	ctx, _ := verbatimWorktree(t)
	if proceed, err := foreigndata.Allow(params(ctx, ctx.ProjectDir, flow.Unattended{})); !proceed || err != nil {
		t.Errorf("Allow = (%v, %v), want main let through", proceed, err)
	}
}
