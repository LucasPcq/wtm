package env

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

type recorder struct {
	flowtest.Recorder
	outcome    Outcome
	reconciled bool
}

func (r *recorder) Reconciled(outcome Outcome) error {
	r.outcome, r.reconciled = outcome, true
	return nil
}

func testContext(t *testing.T) flow.Context {
	t.Helper()
	globaldir.Isolate(t)
	dir, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := domain.Config{}
	cfg.Project.Worktrees.BasePath = "../.trees"
	cfg.Project.Worktrees.BaseBranch = "main"
	cfg.Project.Env.Strategy = domain.EnvStrategyMain
	cfg.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	write(t, filepath.Join(dir, ".env"), "SHARED=main\n")
	return flow.Context{ProjectDir: dir, StateDir: stateDir, Config: cfg}
}

// withPorts declares a linked port, which is what makes a worktree recorded
// without an isolation one that still has to adopt it.
func withPorts(t *testing.T, ctx flow.Context) {
	t.Helper()
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nWEB_PORT=3000\n")
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
	}}); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// makeWorktree creates a worktree with no isolation recorded, as one created
// before the choice existed.
func makeWorktree(t *testing.T, ctx flow.Context, branch string) string {
	t.Helper()
	result, err := worktree.Create(domain.CreateParams{
		ProjectDir:   ctx.ProjectDir,
		StateDir:     ctx.StateDir,
		Branch:       branch,
		FromBranch:   "main",
		SourceBranch: "main",
		Config:       ctx.Config,
		SkipHooks:    true,
	})
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	return result.Path
}

// forgetIsolation leaves the worktree as one created before the isolation
// choice existed: a meta.json that never recorded one.
func forgetIsolation(t *testing.T, ctx flow.Context, branch string) {
	t.Helper()
	legacy := `{"source_branch": "main", "created_at": "2026-01-01T00:00:00Z", "env_strategy": "main"}`
	write(t, filepath.Join(rules.WorktreeMetaDir(ctx.StateDir, branch), domain.MetaFileName), legacy)
}

func run(ctx flow.Context, request Request, prompter flow.Prompter) (Outcome, *recorder, error) {
	if request.Mode == "" {
		request.Mode = domain.EnvModeAdd
	}
	if request.OnConflict == "" {
		request.OnConflict = domain.EnvDecisionKeep
	}
	presenter := &recorder{}
	outcome, err := Run(Params{Context: ctx, Request: request, Prompter: prompter, Presenter: presenter})
	return outcome, presenter, err
}

func TestRunAppliesTheWizardsDecisions(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/a")
	write(t, filepath.Join(path, ".env"), "SHARED=mine\nORPHAN=1\n")

	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeyWorktree: "feat/a", KeyRecap: domain.EnvApplyValue},
		EnvDecisions: map[string][]domain.EnvFileDecision{KeyResolve: {{
			Target:    ".env",
			Decisions: map[string]domain.EnvConflictDecision{"SHARED": domain.EnvDecisionOverwrite},
			PruneKeys: []string{"ORPHAN"},
		}}},
	}
	outcome, presenter, err := run(ctx, Request{Mode: domain.EnvModeRefresh}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := read(t, filepath.Join(path, ".env")); got != "SHARED=main\n" {
		t.Errorf(".env = %q, want the conflict overwritten and the orphan pruned", got)
	}
	if !presenter.reconciled || outcome.IsolationChanged {
		t.Errorf("outcome = %+v, want a reconciliation that changed no isolation", outcome)
	}
	if prompter.AskedKeys() != "env.worktree,env.resolve,env.recap" {
		t.Errorf("asked %s", prompter.AskedKeys())
	}
	if len(presenter.Stages) != 1 || presenter.Stages[0] != domain.EnvScanLoading {
		t.Errorf("stages = %v, want the one pre-scan", presenter.Stages)
	}

	recap := prompter.Content[KeyRecap].Description
	for _, want := range []string{"Worktree:  feat/a", "Mode:      refresh", "Env:       main", `SHARED  overwrite → "main"`, `ORPHAN  remove "1"`} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap lacks %q:\n%s", want, recap)
		}
	}
	badges := prompter.Content[KeyWorktree].Options
	if len(badges) == 0 {
		t.Fatal("the picker offers no worktree")
	}
}

func TestRunAsksNothingOfAWorktreeInSync(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")

	prompter := &flowtest.ScriptedPrompter{}
	_, presenter, err := run(ctx, Request{Worktree: "feat/a"}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prompter.Asked) != 0 || !presenter.reconciled {
		t.Errorf("asked %v, reconciled %v; want no question and the report", prompter.Asked, presenter.reconciled)
	}
}

func TestRunSkipsTheResolverWhenOnlyPortsMove(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	makeWorktree(t, ctx, "feat/a")

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: domain.EnvApplyValue}}
	if _, _, err := run(ctx, Request{Worktree: "feat/a"}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prompter.AskedKeys() != "env.recap" {
		t.Errorf("asked %s, want the recap alone", prompter.AskedKeys())
	}
	recap := prompter.Content[KeyRecap]
	if !strings.Contains(recap.Description, domain.EnvRecapSafeOnly) || !strings.Contains(recap.Description, "WEB_PORT") || len(recap.Options) != 2 {
		t.Errorf("recap = %+v, want the port move announced and the verbatim action offered", recap)
	}
}

func TestRunAbortWritesNothing(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/a")
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nNEW=1\n")

	outcome, presenter, err := run(ctx, Request{Worktree: "feat/a"}, &flowtest.ScriptedPrompter{Abort: true})
	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v; want an abort", outcome, err)
	}
	if len(presenter.Notices) != 1 || !presenter.Notices[0].IsAbort() || presenter.reconciled {
		t.Errorf("notices = %v, reconciled = %v", presenter.Notices, presenter.reconciled)
	}
	if got := read(t, filepath.Join(path, ".env")); strings.Contains(got, "NEW") {
		t.Errorf(".env = %q, want it untouched", got)
	}
}

func TestRunAdoptingRecordsTheIsolation(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	makeWorktree(t, ctx, "feat/a")
	forgetIsolation(t, ctx, "feat/a")
	ref := worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: "feat/a"}

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyAdopt: domain.IsolationAdoptValue, KeyRecap: domain.EnvApplyValue}}
	outcome, _, err := run(ctx, Request{Worktree: "feat/a"}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.IsolationChanged || worktree.RecordedIsolation(ref) != domain.IsolationIsolated {
		t.Errorf("outcome = %+v, recorded = %q; want isolated, and the change reported", outcome, worktree.RecordedIsolation(ref))
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.RecapFieldIsolation+domain.IsolationSummaryIsolated) || !strings.Contains(recap, domain.EnvRecapAdoptPorts) {
		t.Errorf("recap = %q, want the adoption and its port move named", recap)
	}
}

func TestRunReportsNoChangeForTheIsolationAlreadyRecorded(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	makeWorktree(t, ctx, "feat/a")
	forgetIsolation(t, ctx, "feat/a")

	first, _, err := run(ctx, Request{Worktree: "feat/a", Isolation: domain.IsolationIsolated}, flow.Unattended{})
	if err != nil || !first.IsolationChanged {
		t.Fatalf("first run: %+v, %v; want the isolation recorded", first, err)
	}
	second, _, err := run(ctx, Request{Worktree: "feat/a", Isolation: domain.IsolationIsolated}, flow.Unattended{})
	if err != nil || second.IsolationChanged {
		t.Errorf("second run: %+v, %v; want nothing new recorded", second, err)
	}
}

func TestRunUnattendedNeedsAWorktree(t *testing.T) {
	ctx := testContext(t)
	for _, request := range []Request{{}, {Check: true}} {
		if _, _, err := run(ctx, request, flow.Unattended{}); !errors.Is(err, domain.ErrEnvWorktreeRequired) {
			t.Errorf("request %+v: err = %v, want ErrEnvWorktreeRequired", request, err)
		}
	}
}

func TestRunCheckAsksNothingAndWritesNothing(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/a")
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nNEW=1\n")

	prompter := &flowtest.ScriptedPrompter{}
	outcome, _, err := run(ctx, Request{Worktree: "feat/a", Check: true}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prompter.Asked) != 0 || !outcome.Result.Check {
		t.Errorf("asked %v, result %+v", prompter.Asked, outcome.Result)
	}
	if got := read(t, filepath.Join(path, ".env")); strings.Contains(got, "NEW") {
		t.Errorf(".env = %q, want it untouched", got)
	}
}

func TestRunOpensTheResolverOnWhatTheFlagsDecided(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/a")
	write(t, filepath.Join(path, ".env"), "SHARED=mine\nORPHAN=1\n")

	prompter := &flowtest.ScriptedPrompter{
		Answers:      map[string]string{KeyRecap: domain.EnvApplyValue},
		EnvDecisions: map[string][]domain.EnvFileDecision{KeyResolve: nil},
	}
	request := Request{Worktree: "feat/a", Mode: domain.EnvModeRefresh, OnConflict: domain.EnvDecisionOverwrite, Prune: true}
	if _, _, err := run(ctx, request, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.Content[KeyResolve].EnvDefaults; !got.Overwrite || !got.Prune {
		t.Errorf("defaults = %+v, want --on-conflict overwrite and --prune carried to the resolver", got)
	}
}

func TestPickerBadgesPendingAdditionsAndDisablesARefusedIsolation(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nNEW=1\n")

	prompter := &flowtest.ScriptedPrompter{
		Answers:      map[string]string{KeyWorktree: "feat/a", KeyRecap: domain.EnvApplyValue},
		EnvDecisions: map[string][]domain.EnvFileDecision{KeyResolve: nil},
	}
	if _, _, err := run(ctx, Request{Isolation: domain.IsolationVerbatim}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	options := map[string]flow.Option{}
	for _, option := range prompter.Content[KeyWorktree].Options {
		options[option.Value] = option
	}
	if main := options["main"]; !main.Disabled {
		t.Errorf("main = %+v, want it disabled: it refuses verbatim", main)
	}
	if badges := options["feat/a"].Badges; len(badges) == 0 || badges[len(badges)-1].Text != "1 change(s)" {
		t.Errorf("feat/a badges = %+v, want the pending addition counted", badges)
	}
}
