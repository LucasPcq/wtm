package create

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type brokenRunCase struct {
	name  string
	setup func(t *testing.T, ctx *flow.Context)
	// cause is what the warning must name, empty when nothing is worth saying.
	cause string
}

func writeRunToml(t *testing.T, ctx flow.Context, body string) {
	t.Helper()
	if err := os.MkdirAll(ctx.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx.StateDir, domain.RunFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func linkedRunConfig(t *testing.T, ctx flow.Context, file string) {
	t.Helper()
	if err := config.WriteRun(config.WriteRunParams{
		StateDir: ctx.StateDir,
		Force:    true,
		Config: domain.RunConfig{
			Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 3000}}},
			EnvPorts: []domain.EnvPortLink{{File: file, Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		},
	}); err != nil {
		t.Fatalf("write run config: %v", err)
	}
}

func corruptNeighbour(t *testing.T, ctx flow.Context) {
	t.Helper()
	gittest.Git(t, ctx.ProjectDir, "worktree", "add", "-b", "feat/other", filepath.Join(t.TempDir(), "other"))
	metaDir := rules.WorktreeMetaDir(ctx.StateDir, "feat/other")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// G1: run.toml belongs to the run module. Whatever state it is in, a create
// ends like it did before the module existed — the worktree provisioned and
// its hooks run — and the port pass it could not do is said, not failed on.
func TestRunCreatesWhateverStateRunTomlIsIn(t *testing.T) {
	cases := []brokenRunCase{
		{
			name:  "unknown key",
			setup: func(t *testing.T, ctx *flow.Context) { writeRunToml(t, *ctx, "bogus_key = 1\n") },
			cause: "bogus_key",
		},
		{
			name: "link on a file config.toml does not provision",
			setup: func(t *testing.T, ctx *flow.Context) {
				ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
				linkedRunConfig(t, *ctx, "apps/gone/.env")
			},
			cause: "apps/gone/.env",
		},
		{
			name: "neighbour meta unreadable, links declared",
			setup: func(t *testing.T, ctx *flow.Context) {
				ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
				linkedRunConfig(t, *ctx, ".env")
				corruptNeighbour(t, *ctx)
			},
			cause: "feat/other",
		},
		{
			name:  "neighbour meta unreadable, no run.toml",
			setup: func(t *testing.T, ctx *flow.Context) { corruptNeighbour(t, *ctx) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testContext(t)
			ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "touch hook-ran"}}
			tc.setup(t, &ctx)
			presenter := newRecorder()

			outcome, err := Run(Params{
				Context:   ctx,
				Request:   Request{Branch: "feat/broken", From: "main", EnvFrom: "example"},
				Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCreate}},
				Presenter: presenter,
			})
			if err != nil {
				t.Fatalf("create must not fail over the run module: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(outcome.Result.Path, "hook-ran")); statErr != nil {
				t.Errorf("on_create hooks did not run: %v", statErr)
			}
			if presenter.created == nil {
				t.Fatal("the run must conclude like any create")
			}

			warnings := outcome.Result.Warnings
			if tc.cause == "" {
				if len(warnings) != 0 {
					t.Errorf("warnings = %v, want none", warnings)
				}
				return
			}
			if len(warnings) != 1 || !strings.Contains(warnings[0], tc.cause) || !strings.Contains(warnings[0], "wtm env feat/broken") {
				t.Errorf("warnings = %v, want one naming %q and the command that settles it", warnings, tc.cause)
			}
			if !recordedWarning(presenter.Statuses, tc.cause) {
				t.Errorf("statuses = %+v, want a warning naming %q", presenter.Statuses, tc.cause)
			}
		})
	}
}

// An invalid run.toml is read before the worktree exists: nothing it would
// have written — an ordinal — is left behind in the worktree's record.
func TestRunAllocatesNoOrdinalAtCreate(t *testing.T) {
	ctx := testContext(t)
	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/lazy", From: "main", EnvFrom: "example"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmCreate}},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Result.Metadata.Ordinal != 0 {
		t.Errorf("ordinal = %d, want none allocated before the run module asks", outcome.Result.Metadata.Ordinal)
	}
}

func recordedWarning(notices []flow.Notice, cause string) bool {
	for _, notice := range notices {
		if notice.Kind == flow.NoticeWarning && strings.Contains(notice.Text+strings.Join(notice.Lines, " "), cause) {
			return true
		}
	}
	return false
}

func portContext(t *testing.T) flow.Context {
	t.Helper()
	ctx := testContext(t)
	ctx.Config.Project.Env.Strategy = domain.EnvStrategyMain
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	linkedRunConfig(t, ctx, ".env")
	if err := os.WriteFile(filepath.Join(ctx.ProjectDir, ".env"), []byte("WEB_PORT=3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ctx
}

// The outcome carries what the JSON reports: the worktree's isolation and the
// port pass in `wtm env`'s shape.
func TestRunReportsTheIsolationAndThePortPass(t *testing.T) {
	outcome, err := Run(Params{
		Context:   portContext(t),
		Request:   Request{Branch: "feat/ports", From: "main", EnvFrom: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCreate}},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Result.Isolation != domain.IsolationIsolated {
		t.Errorf("isolation = %q, want isolated", outcome.Result.Isolation)
	}
	ports := outcome.Result.EnvPorts
	if !ports.Applied || len(ports.Entries) != 1 || ports.Entries[0].Status != domain.EnvPortStatusRewrite {
		t.Errorf("env_ports = %+v, want the one link settled", ports)
	}
}

// --isolation answers a creation. A worktree --if-not-exists found already
// there keeps what it is, and the flag is not dropped without a word.
func TestRunWarnsAnIsolationTheExistingWorktreeIgnores(t *testing.T) {
	ctx := portContext(t)
	if _, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/there", From: "main", EnvFrom: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCreate}},
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	presenter := newRecorder()
	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/there", From: "main", IfNotExists: true, Isolation: domain.IsolationVerbatim},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if !outcome.Result.AlreadyExists || outcome.Result.Isolation != domain.IsolationIsolated {
		t.Fatalf("result = %+v, want the existing isolated worktree", outcome.Result)
	}
	if len(outcome.Result.Warnings) != 1 || !strings.Contains(outcome.Result.Warnings[0], "--isolation verbatim ignored") {
		t.Errorf("warnings = %v, want the ignored flag named", outcome.Result.Warnings)
	}
	if !recordedWarning(presenter.Statuses, "ignored") {
		t.Errorf("statuses = %+v, want the warning shown", presenter.Statuses)
	}
}
