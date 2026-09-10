package envports_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A project with no [[env_port]] link has nothing to settle: the step is not
// posed, and the pass writes and says nothing.
func TestSettleWithoutLinksReportsNothing(t *testing.T) {
	presenter := &flowtest.Recorder{}
	ctx := flow.Context{ProjectDir: t.TempDir(), StateDir: t.TempDir()}

	if envports.Linked(ctx) {
		t.Error("a project with no [[env_port]] link must not pose the step")
	}

	settlement, err := envports.Settle(envports.Params{
		Context:      ctx,
		Branch:       "feat/x",
		WorktreePath: t.TempDir(),
		Rewrite:      true,
		Presenter:    presenter,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settlement.Shifted != 0 {
		t.Errorf("settled %d value(s), want none", settlement.Shifted)
	}
	if len(presenter.Statuses) != 0 {
		t.Errorf("reported %+v, want nothing", presenter.Statuses)
	}
}

// settleFixture is a repository whose run.toml links a .env value to a job's
// port, plus a second worktree — the one being settled — holding the value it
// was copied with.
func settleFixture(t *testing.T) (flow.Context, string) {
	t.Helper()
	repo := gittest.InitRepo(t)
	stateDir := filepath.Join(repo, ".git", "wtm")
	second := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)

	if err := config.WriteRun(config.WriteRunParams{
		StateDir: stateDir,
		Force:    true,
		Config: domain.RunConfig{
			Jobs: []domain.JobConfig{{
				Name: "web", Kind: domain.JobKindService, Cmd: "true",
				Ports: map[string]int{"PORT": 3000},
			}},
			EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		},
	}); err != nil {
		t.Fatalf("write run config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(second, ".env"), []byte("WEB_PORT=3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := flow.Context{ProjectDir: repo, StateDir: stateDir}
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	return ctx, second
}

func TestSettleMovesTheCopiedPortsWhenTheRunSaidSo(t *testing.T) {
	ctx, worktreePath := settleFixture(t)
	if !envports.Linked(ctx) {
		t.Fatal("the fixture links a value to a port, so the step must be posed")
	}

	presenter := &flowtest.Recorder{}
	settlement, err := envports.Settle(envports.Params{
		Context:      ctx,
		Branch:       "feature",
		WorktreePath: worktreePath,
		Rewrite:      true,
		Presenter:    presenter,
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}

	body := readEnv(t, worktreePath)
	if strings.Contains(body, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want the copied port moved onto this worktree's", body)
	}
	// The pass reports a count to whoever concludes the run, and prints nothing
	// itself: the values it moved are in the .env beside it.
	if !settlement.Applied || settlement.Shifted != 1 {
		t.Errorf("settlement = %+v, want 1 value applied", settlement)
	}
	if len(presenter.Statuses) != 0 {
		t.Errorf("reported %+v, want nothing to print", presenter.Statuses)
	}
}

// The user answered "leave the copied values as they are" in the run that
// created the worktree. Nothing may rewrite them behind that answer.
func TestSettleLeavesTheValuesAloneWhenTheRunSaidSo(t *testing.T) {
	ctx, worktreePath := settleFixture(t)

	settlement, err := envports.Settle(envports.Params{
		Context:      ctx,
		Branch:       "feature",
		WorktreePath: worktreePath,
		Rewrite:      false,
		Presenter:    &flowtest.Recorder{},
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if settlement.Applied {
		t.Error("a declined pass must not report itself as applied")
	}

	if body := readEnv(t, worktreePath); !strings.Contains(body, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want the copied value untouched", body)
	}
}

func readEnv(t *testing.T, worktreePath string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(worktreePath, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	return string(body)
}
