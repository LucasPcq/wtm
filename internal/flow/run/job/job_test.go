package job_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func context(t *testing.T) flow.Context {
	t.Helper()
	return flow.Context{StateDir: t.TempDir()}
}

type recorder struct {
	flowtest.Recorder
	changed []jobflow.Outcome
}

func (r *recorder) Changed(outcome jobflow.Outcome) error {
	r.changed = append(r.changed, outcome)
	return nil
}

func runner() domain.JobConfig {
	off := false
	return domain.JobConfig{
		Name:        "dev",
		Kind:        domain.JobKindService,
		Cmd:         "turbo run dev",
		Runs:        []string{"api", "web"},
		BindsNoPort: true,
		// Probe has no flag at all: it is written programmatically, so an edit
		// that rebuilt the job from the form alone would re-arm the port probe
		// of a job whose reader opted out.
		Probe: &off,
	}
}

// A command with nothing to pre-fill it has no safe default: the run is refused
// naming the flag rather than writing a job nobody described.
func TestAddWithoutACommandIsRefusedNamingTheFlag(t *testing.T) {
	presenter := &recorder{}

	_, err := jobflow.Add(jobflow.AddParams{
		Context:   context(t),
		Request:   jobflow.AddRequest{Initial: domain.JobConfig{Name: "api"}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagCmd) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagCmd)
	}
}

// Every field a flag filled in is the answer of a run that cannot be asked, so
// the flags alone describe a whole job.
func TestAddResolvesEveryFieldFromWhatTheFlagsGave(t *testing.T) {
	ctx := context(t)
	presenter := &recorder{}

	outcome, err := jobflow.Add(jobflow.AddParams{
		Context: ctx,
		Request: jobflow.AddRequest{Initial: domain.JobConfig{
			Name:  "api",
			Cmd:   "pnpm dev",
			Kind:  domain.JobKindService,
			Ports: map[string]int{"PORT": 3000},
		}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if outcome.Name != "api" || outcome.Status != domain.JobActionAdded {
		t.Errorf("outcome = %+v, want api added", outcome)
	}

	saved := loadConfig(t, ctx.StateDir)
	if len(saved.Jobs) != 1 || saved.Jobs[0].Ports["PORT"] != 3000 {
		t.Errorf("saved = %+v, want the job with its declared port", saved.Jobs)
	}
}

// The form has no question for `probe`, and what a form never showed it must
// not drop: an edit used to unlink a runner from its children without a word.
func TestEditKeepsWhatTheFormDoesNotAskAbout(t *testing.T) {
	ctx := context(t)
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		runner(),
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm --filter api dev"},
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm --filter web dev"},
	}}
	presenter := &recorder{}

	_, err := jobflow.Edit(jobflow.EditParams{
		Context: ctx,
		Request: jobflow.EditRequest{Name: "dev", Config: cfg},
		Prompter: &flowtest.ScriptedPrompter{
			Answers: formAnswers(map[string]string{
				jobflow.KeyName:        "dev",
				jobflow.KeyCmd:         "turbo run dev --parallel",
				jobflow.KeyBindsNoPort: domain.RunJobBindsNoPortYes,
			}),
			Sets: map[string][]string{jobflow.KeyRuns: {"api", "web"}, jobflow.KeyTouches: nil},
		},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	saved := loadConfig(t, ctx.StateDir)
	if len(saved.Jobs) != 3 {
		t.Fatalf("saved %d jobs, want 3", len(saved.Jobs))
	}
	if got := saved.Jobs[0]; len(got.Runs) != 2 || !got.BindsNoPort {
		t.Errorf("runs = %v, binds_no_port = %v, want what the form answered", got.Runs, got.BindsNoPort)
	}
	if got := saved.Jobs[0].Probe; got == nil || *got {
		t.Errorf("probe = %v — the form re-armed a probe its reader had turned off", got)
	}
	if saved.Jobs[0].Cmd != "turbo run dev --parallel" {
		t.Errorf("cmd = %q, want the answered one", saved.Jobs[0].Cmd)
	}
}

// Answering the form as it opens writes the job back as it was: an edit that
// dropped a shared service's scope and namespace turned the database back into
// one per worktree, and left every [[env]] link naming a namespace that no
// longer existed.
func TestEditKeepsASharedServicesScopeAndNamespace(t *testing.T) {
	ctx := context(t)
	namespace := &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true"}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "pg", Kind: domain.JobKindService, Cmd: "docker compose up -d pg", Scope: domain.JobScopeShared, Namespace: namespace},
		{Name: "reset", Kind: domain.JobKindTask, Cmd: "pnpm reset", Touches: []string{"pg"}},
	}}

	for _, edit := range []struct {
		name, cmd, kind string
		touches         []string
	}{
		{"pg", "docker compose up -d --wait pg", string(domain.JobKindService), nil},
		{"reset", "pnpm db:reset", string(domain.JobKindTask), []string{"pg"}},
	} {
		if _, err := jobflow.Edit(jobflow.EditParams{
			Context: ctx,
			Request: jobflow.EditRequest{Name: edit.name, Config: cfg},
			Prompter: &flowtest.ScriptedPrompter{
				Answers: formAnswers(map[string]string{
					jobflow.KeyName: edit.name, jobflow.KeyCmd: edit.cmd, jobflow.KeyKind: edit.kind,
					jobflow.KeyScope: domain.ScopeValueShared, jobflow.KeyNamespaceName: "app_{worktree}", jobflow.KeyNamespaceCreate: "true",
				}),
				Sets: map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: edit.touches},
			},
			Presenter: &recorder{},
		}); err != nil {
			t.Fatalf("Edit %s: %v", edit.name, err)
		}
		cfg = loadConfig(t, ctx.StateDir)
	}

	if got := cfg.Jobs[0]; got.Scope != domain.JobScopeShared || got.Namespace == nil || got.Namespace.Name != "app_{worktree}" {
		t.Errorf("pg = %+v, want its scope and namespace kept", got)
	}
	if got := cfg.Jobs[1].Touches; len(got) != 1 || got[0] != "pg" {
		t.Errorf("reset touches = %v, want them kept", got)
	}
	if got := cfg.Jobs[1]; got.Scope != domain.JobScopePerWorktree || got.Namespace != nil {
		t.Errorf("reset = %+v, want a task left per worktree", got)
	}
}

// A patch is the non-interactive edit, and the form is the only other way to
// change something. A run that can open neither is refused rather than writing
// the job back untouched.
func TestEditWithNothingToChangeIsRefused(t *testing.T) {
	presenter := &recorder{}

	_, err := jobflow.Edit(jobflow.EditParams{
		Context:   context(t),
		Request:   jobflow.EditRequest{Name: "dev", Config: domain.RunConfig{Jobs: []domain.JobConfig{runner()}}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagCmd) {
		t.Fatalf("err = %v, want one naming the flags it could have taken", err)
	}
}

// A job several profiles start is refused until the caller says the references
// may go with it — the safety axis, which no amount of confirming replaces.
func TestRemoveRefusesAReferencedJobWithoutForce(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "api", Kind: domain.JobKindService, Cmd: "true"}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}}},
	}
	presenter := &recorder{}

	_, err := jobflow.Remove(jobflow.RemoveParams{
		Context:   context(t),
		Request:   jobflow.RemoveRequest{Name: "api", Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err == nil || !strings.Contains(err.Error(), "dev") {
		t.Fatalf("err = %v, want one naming the profile that starts it", err)
	}
}

func loadConfig(t *testing.T, stateDir string) domain.RunConfig {
	t.Helper()
	cfg, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load %s: %v", filepath.Base(stateDir), err)
	}
	return cfg
}

// The refusal is the safety axis, and --force is not its only key: a run with
// someone to ask lifts it by answering. That is what lets `run job list` remove
// a referenced job, since it has no --force of its own.
func TestRemoveLiftsTheReferenceRefusalByAsking(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "api", Kind: domain.JobKindService, Cmd: "true"}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}}},
	}

	prompter := &flowtest.ScriptedPrompter{Confirmed: true}
	outcome, err := jobflow.Remove(jobflow.RemoveParams{
		Context:   ctx,
		Request:   jobflow.RemoveRequest{Name: "api", Config: cfg},
		Prompter:  prompter,
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if prompter.Confirms != 1 {
		t.Errorf("asked %d times, want exactly one confirmation", prompter.Confirms)
	}
	if outcome.Status != domain.JobActionRemoved {
		t.Errorf("outcome = %+v, want the job removed", outcome)
	}
	if len(loadConfig(t, ctx.StateDir).Jobs) != 0 {
		t.Error("the job survived a lifted refusal")
	}
}

// Declining leaves the file alone: a refusal the reader kept is not a failure.
func TestRemoveKeepsTheJobWhenTheRefusalStands(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "api", Kind: domain.JobKindService, Cmd: "true"}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}}},
	}

	outcome, err := jobflow.Remove(jobflow.RemoveParams{
		Context:   ctx,
		Request:   jobflow.RemoveRequest{Name: "api", Config: cfg},
		Prompter:  &flowtest.ScriptedPrompter{Confirmed: false},
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !outcome.Aborted {
		t.Error("declining the removal did not abort it")
	}
	if _, err := runconfig.Load(ctx.StateDir); err != nil {
		t.Fatalf("load: %v", err)
	}
}

// A runner that starts the job is a reference too, and the refusal must name it
// — removing it silently is what made such a job unremovable.
func TestRemoveNamesTheRunnersInItsRefusal(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev", Runs: []string{"api"}},
		{Name: "api", Kind: domain.JobKindService, Cmd: "true"},
	}}

	_, err := jobflow.Remove(jobflow.RemoveParams{
		Context:   flow.Context{StateDir: t.TempDir()},
		Request:   jobflow.RemoveRequest{Name: "api", Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if err == nil || !strings.Contains(err.Error(), "dev") {
		t.Fatalf("err = %v, want one naming the runner that starts it", err)
	}
}
