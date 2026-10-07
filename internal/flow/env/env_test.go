package env

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	for _, want := range []string{"Worktree:  feat/a", "Mode:      refresh", "Env:       main", `SHARED  overwrite → "main"`, `ORPHAN  prune "1"`} {
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

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: domain.EnvKeepValue, KeyRecap: domain.EnvApplyValue}}
	if _, _, err := run(ctx, Request{Worktree: "feat/a"}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prompter.AskedKeys() != "env.isolation,env.recap" {
		t.Errorf("asked %s, want the isolation and the recap alone", prompter.AskedKeys())
	}
	recap := prompter.Content[KeyRecap]
	if !strings.Contains(recap.Description, domain.EnvRecapSafeOnly) || !strings.Contains(recap.Description, "WEB_PORT") || len(recap.Options) != 1 {
		t.Errorf("recap = %+v, want the port move announced and a single confirmation", recap)
	}
	if want := domain.RecapFieldIsolation + domain.IsolationSummaryIsolated + domain.EnvRecapUnchanged; !strings.Contains(recap.Description, want) {
		t.Errorf("recap lacks %q:\n%s", want, recap.Description)
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

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: string(domain.IsolationIsolated), KeyRecap: domain.EnvApplyValue}}
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
	outcome, presenter, err := run(ctx, Request{Worktree: "feat/a", Check: true}, prompter)
	if !errors.Is(err, domain.ErrEnvDrift) || !errors.Is(err, domain.ErrAborted) || !presenter.reconciled {
		t.Fatalf("err = %v, reconciled = %v; want the report, then the drift exit", err, presenter.reconciled)
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

func TestRunCheckOfAWorktreeInSyncSucceeds(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")
	if _, _, err := run(ctx, Request{Worktree: "feat/a", Check: true}, flow.Unattended{}); err != nil {
		t.Errorf("err = %v, want a clean check to exit 0", err)
	}
}

func TestAnIsolationChangeIsPublishedAndAnUnchangedOneIsNot(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	makeWorktree(t, ctx, "feat/a")
	forgetIsolation(t, ctx, "feat/a")
	events := &flowtest.Recorder{}
	ctx.Publisher = events

	for range 2 {
		if _, _, err := run(ctx, Request{Worktree: "feat/a", Isolation: domain.IsolationIsolated}, flow.Unattended{}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	var updates []domain.Event
	for _, event := range events.Published {
		if event.Type == domain.EventWorktreeUpdated && slices.Contains(event.Changed, domain.IdentityIsolation) {
			updates = append(updates, event)
		}
	}
	if len(updates) != 1 || updates[0].Worktree.Isolation != domain.IsolationIsolated {
		t.Fatalf("isolation updates = %+v, want exactly one, to isolated", updates)
	}
}

func TestRunSettlesTheConfiguredEnvDespiteAnOrphanLink(t *testing.T) {
	ctx := testContext(t)
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nWEB_PORT=3000\n")
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}},
		EnvPorts: []domain.EnvPortLink{
			{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"},
			{File: "services/x/.env", Key: "X_PORT", Job: "web", Port: "PORT"},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	path := makeWorktree(t, ctx, "feat/a")

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: domain.EnvKeepValue, KeyRecap: domain.EnvApplyValue}}
	outcome, _, err := run(ctx, Request{Worktree: "feat/a"}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := read(t, filepath.Join(path, ".env")); strings.Contains(got, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want WEB_PORT moved off main's port", got)
	}
	warnings := outcome.Result.Warnings
	if len(warnings) != 1 || !strings.Contains(warnings[0], "X_PORT") || !strings.Contains(warnings[0], "services/x/.env") || strings.Contains(warnings[0], "not settled") {
		t.Errorf("warnings = %v, want the orphan link named alone, the pass settled", warnings)
	}
}

const namedAPI = `
[[job]]
name = "api"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 4001 }
url = { port = "PORT" }

[[env_port]]
file = ".env"
key = "API_URL"
job = "api"
port = "PORT"
`

// withNamedAPI publishes a job by name and links a URL-shaped value to it,
// with main's .env still spelling the port, as a checkout predating wtm does.
func withNamedAPI(t *testing.T, ctx flow.Context, addressing domain.Addressing) {
	t.Helper()
	write(t, filepath.Join(ctx.StateDir, domain.RunFileName), "addressing = \""+string(addressing)+"\"\n"+namedAPI)
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nAPI_URL=http://localhost:4001\n")
}

func setIsolation(t *testing.T, ctx flow.Context, branch string, isolation domain.Isolation) {
	t.Helper()
	ref := worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch}
	if err := worktree.SetIsolation(worktree.SetIsolationParams{Ref: ref, Isolation: isolation}); err != nil {
		t.Fatal(err)
	}
}

func recorded(ctx flow.Context, branch string) domain.Isolation {
	return worktree.RecordedIsolation(worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch})
}

func TestTheWizardSwitchesAnIsolatedWorktreeToVerbatim(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	path := makeWorktree(t, ctx, "feat/a")
	if _, _, err := run(ctx, Request{Worktree: "feat/a"}, flow.Unattended{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := read(t, filepath.Join(path, ".env")); strings.Contains(got, "WEB_PORT=3000") {
		t.Fatalf(".env = %q, want its own port before the switch", got)
	}

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyWorktree:  "feat/a",
		KeyIsolation: string(domain.IsolationVerbatim),
		KeyRecap:     domain.EnvApplyValue,
	}}
	outcome, _, err := run(ctx, Request{}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if options := prompter.Content[KeyIsolation].Options; len(options) != 2 || options[0].Value != domain.EnvKeepValue || options[0].Label != domain.EnvIsolationKeepIsolated {
		t.Errorf("options = %+v, want keeping isolated first, then the switch", options)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.EnvRestoreRecapTitle) || !strings.Contains(recap, domain.RecapFieldIsolation+domain.IsolationSummaryVerbatim) {
		t.Errorf("recap = %q, want the switch and what it puts back", recap)
	}
	if recap := prompter.Content[KeyRecap].Description; strings.Contains(recap, domain.EnvRecapFieldAddressing) {
		t.Errorf("recap = %q, want no addressing line on a linked worktree", recap)
	}
	if !outcome.IsolationChanged || recorded(ctx, "feat/a") != domain.IsolationVerbatim {
		t.Errorf("outcome = %+v, recorded = %q; want verbatim recorded", outcome, recorded(ctx, "feat/a"))
	}
	if got := read(t, filepath.Join(path, ".env")); !strings.Contains(got, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want the source's port back", got)
	}
}

func TestTheWizardMovesAVerbatimWorktreeOntoIsolation(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	path := makeWorktree(t, ctx, "feat/a")
	setIsolation(t, ctx, "feat/a", domain.IsolationVerbatim)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyIsolation: string(domain.IsolationIsolated), KeyRecap: domain.EnvApplyValue}}
	outcome, _, err := run(ctx, Request{Worktree: "feat/a"}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if options := prompter.Content[KeyIsolation].Options; len(options) != 2 || options[0].Label != domain.EnvIsolationKeepVerbatim {
		t.Errorf("options = %+v, want keeping verbatim first", options)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.EnvRecapAdoptPorts) {
		t.Errorf("recap = %q, want the port move announced", recap)
	}
	if !outcome.IsolationChanged || recorded(ctx, "feat/a") != domain.IsolationIsolated {
		t.Errorf("outcome = %+v, recorded = %q; want isolated recorded", outcome, recorded(ctx, "feat/a"))
	}
	if got := read(t, filepath.Join(path, ".env")); strings.Contains(got, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want its own port", got)
	}
}

func TestAnUnattendedRunKeepsAVerbatimWorktreeAsItIs(t *testing.T) {
	ctx := testContext(t)
	withPorts(t, ctx)
	path := makeWorktree(t, ctx, "feat/a")
	setIsolation(t, ctx, "feat/a", domain.IsolationVerbatim)

	outcome, _, err := run(ctx, Request{Worktree: "feat/a"}, flow.Unattended{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.IsolationChanged || recorded(ctx, "feat/a") != domain.IsolationVerbatim {
		t.Errorf("outcome = %+v, recorded = %q; want it left verbatim", outcome, recorded(ctx, "feat/a"))
	}
	if got := read(t, filepath.Join(path, ".env")); !strings.Contains(got, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want the source's port kept", got)
	}
}

func TestMainKeepsWhatItsEnvSpellsUnlessAsked(t *testing.T) {
	ctx := testContext(t)
	withNamedAPI(t, ctx, domain.AddressingNames)
	env := filepath.Join(ctx.ProjectDir, ".env")

	if _, _, err := run(ctx, Request{Worktree: "main"}, flow.Unattended{}); err != nil {
		t.Fatalf("unattended: %v", err)
	}
	if got := read(t, env); !strings.Contains(got, "API_URL=http://localhost:4001") {
		t.Fatalf(".env = %q, want main left on ports", got)
	}
	if _, _, err := run(ctx, Request{Worktree: "main", Check: true}, flow.Unattended{}); err != nil {
		t.Errorf("check: %v, want main on ports to read as in sync", err)
	}

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyAddressing: domain.EnvKeepValue}}
	if _, _, err := run(ctx, Request{Worktree: "main"}, prompter); err != nil {
		t.Fatalf("wizard: %v", err)
	}
	if prompter.AskedKeys() != "env.addressing" {
		t.Errorf("asked %s, want the addressing alone", prompter.AskedKeys())
	}
	if options := prompter.Content[KeyAddressing].Options; len(options) != 2 || options[0].Label != domain.EnvAddressingKeepPorts {
		t.Errorf("options = %+v, want keeping ports first", options)
	}

	if _, _, err := run(ctx, Request{Worktree: "main", Addressing: domain.AddressingNames}, flow.Unattended{}); err != nil {
		t.Fatalf("to names: %v", err)
	}
	if got := read(t, env); !strings.Contains(got, ".localhost") || strings.Contains(got, "localhost:4001") {
		t.Fatalf(".env = %q, want main moved onto names", got)
	}

	prompter = &flowtest.ScriptedPrompter{Answers: map[string]string{KeyAddressing: string(domain.AddressingPorts), KeyRecap: domain.EnvApplyValue}}
	if _, _, err := run(ctx, Request{Worktree: "main"}, prompter); err != nil {
		t.Fatalf("back to ports: %v", err)
	}
	if options := prompter.Content[KeyAddressing].Options; options[0].Label != domain.EnvAddressingKeepNames {
		t.Errorf("options = %+v, want keeping names first once main is on them", options)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.EnvRecapFieldAddressing+string(domain.AddressingPorts)) {
		t.Errorf("recap = %q, want the move back to ports named", recap)
	}
	if got := read(t, env); !strings.Contains(got, "API_URL=http://localhost:4001") {
		t.Errorf(".env = %q, want main back on ports", got)
	}
}

func TestAddressingFlagsAWorktreeCannotTakeAreRefused(t *testing.T) {
	ctx := testContext(t)
	withNamedAPI(t, ctx, domain.AddressingNames)
	makeWorktree(t, ctx, "feat/a")

	if _, _, err := run(ctx, Request{Worktree: "feat/a", Addressing: domain.AddressingPorts}, flow.Unattended{}); !errors.Is(err, domain.ErrEnvAddressingMainOnly) {
		t.Errorf("linked worktree on ports: err = %v, want ErrEnvAddressingMainOnly", err)
	}
	if _, _, err := run(ctx, Request{Worktree: "feat/a", Addressing: domain.AddressingNames}, flow.Unattended{}); err != nil {
		t.Errorf("linked worktree on the project's mode: err = %v, want it accepted", err)
	}

	withNamedAPI(t, ctx, domain.AddressingPorts)
	if _, _, err := run(ctx, Request{Worktree: "main", Addressing: domain.AddressingNames}, flow.Unattended{}); !errors.Is(err, domain.ErrEnvAddressingProjectPorts) {
		t.Errorf("main on names in a ports project: err = %v, want ErrEnvAddressingProjectPorts", err)
	}
}

func TestMainGetsBackTheEnvValuesWtmStampedIntoIt(t *testing.T) {
	ctx := testContext(t)
	write(t, filepath.Join(ctx.StateDir, domain.RunFileName), `
[[job]]
name = "kc"
kind = "service"
cmd = "run-kc"

[[env]]
file = ".env"
key = "KEYCLOAK_REALM"
job = "kc"
value = "acme-{worktree}"
`)
	write(t, filepath.Join(ctx.ProjectDir, ".env.example"), "SHARED=\nKEYCLOAK_REALM=acme\n")
	env := filepath.Join(ctx.ProjectDir, ".env")
	write(t, env, "SHARED=main\nKEYCLOAK_REALM=acme-main\n")

	if _, _, err := run(ctx, Request{Worktree: "main", Check: true}, flow.Unattended{}); !errors.Is(err, domain.ErrEnvDrift) {
		t.Errorf("check: err = %v, want the stamped value counted as drift", err)
	}
	if _, _, err := run(ctx, Request{Worktree: "main"}, flow.Unattended{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := read(t, env); got != "SHARED=main\nKEYCLOAK_REALM=acme\n" {
		t.Errorf(".env = %q, want the template's realm back", got)
	}

	write(t, env, "SHARED=main\nKEYCLOAK_REALM=my-realm\n")
	if _, _, err := run(ctx, Request{Worktree: "main"}, flow.Unattended{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := read(t, env); got != "SHARED=main\nKEYCLOAK_REALM=my-realm\n" {
		t.Errorf(".env = %q, want a value the user chose left alone", got)
	}
}

// LUC-265: every step after the picker read the worktree before it was picked,
// so `wtm env` with no argument failed with "worktree not found:" instead of
// asking which one.
func TestWithNoWorktreeGivenThePickerIsAskedFirst(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyWorktree:   "feat/a",
		KeyIsolation:  domain.EnvKeepValue,
		KeyAddressing: domain.EnvKeepValue,
		KeyRecap:      domain.EnvApplyValue,
	}}

	if _, _, err := run(ctx, Request{Mode: domain.EnvModeRefresh}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prompter.Asked) == 0 || prompter.Asked[0] != KeyWorktree {
		t.Errorf("asked = %s, want the worktree picker first", prompter.AskedKeys())
	}
}

// LUC-274: overwriting a port-linked conflict takes the source's value and the
// port pass then shifts it onto this worktree's port, but the resolver and the
// recap previewed the source's port — a value the file never gets.
func TestTheResolverPreviewsALinkedOverwriteOnTheWorktreesPort(t *testing.T) {
	ctx := testContext(t)
	write(t, filepath.Join(ctx.ProjectDir, ".env"), "SHARED=main\nDB_URL=postgres://app:main@localhost:3000/db\n")
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "DB_URL", Job: "web", Port: "PORT"}},
	}}); err != nil {
		t.Fatal(err)
	}
	path := makeWorktree(t, ctx, "feat/a")
	if _, _, err := run(ctx, Request{Worktree: "feat/a"}, flow.Unattended{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	settled := read(t, filepath.Join(path, ".env"))
	if strings.Contains(settled, "localhost:3000") {
		t.Fatalf("setup: .env = %q, want its own port", settled)
	}
	write(t, filepath.Join(path, ".env"), strings.Replace(settled, "app:main@", "app:mine@", 1))

	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeyWorktree: "feat/a", KeyIsolation: domain.EnvKeepValue, KeyRecap: domain.EnvApplyValue},
		EnvDecisions: map[string][]domain.EnvFileDecision{KeyResolve: {{
			Target:    ".env",
			Decisions: map[string]domain.EnvConflictDecision{"DB_URL": domain.EnvDecisionOverwrite},
		}}},
	}
	if _, _, err := run(ctx, Request{Mode: domain.EnvModeRefresh}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}

	written := read(t, filepath.Join(path, ".env"))
	want := ""
	for _, line := range strings.Split(written, "\n") {
		if value, ok := strings.CutPrefix(line, "DB_URL="); ok {
			want = value
		}
	}
	if !strings.Contains(want, "app:main@") || strings.Contains(want, "localhost:3000") {
		t.Fatalf(".env = %q, want the source's value on the worktree's port", written)
	}
	var preview string
	for _, file := range prompter.Content[KeyResolve].EnvFiles {
		for _, entry := range file.Diff.Entries {
			if entry.Key == "DB_URL" {
				preview = entry.ResolvedValue
			}
		}
	}
	if preview != want {
		t.Errorf("resolver previews %q, want %q — what the overwrite writes", preview, want)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, want) {
		t.Errorf("recap lacks %q:\n%s", want, recap)
	}
}
