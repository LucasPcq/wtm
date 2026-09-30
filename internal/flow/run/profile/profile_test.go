package profile_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

type recorder struct {
	flowtest.Recorder
	changed []profileflow.Outcome
}

func (r *recorder) Changed(outcome profileflow.Outcome) error {
	r.changed = append(r.changed, outcome)
	return nil
}

func declared() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm --filter api dev"},
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm --filter web dev"},
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "pnpm seed"},
	}}
}

// A profile with no job selected is not a profile: naming which jobs it starts
// is the whole request, so a run that cannot be asked is refused naming --jobs.
func TestAddWithoutJobsIsRefusedNamingTheFlag(t *testing.T) {
	_, err := profileflow.Add(profileflow.AddParams{
		Context:   flow.Context{StateDir: t.TempDir()},
		Request:   profileflow.AddRequest{Initial: domain.ProfileConfig{Name: "dev"}, Config: declared()},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagJobs) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagJobs)
	}
}

// The order of --jobs is the start order, so it is what the profile records —
// the order step resolves to the selection rather than to the catalogue.
func TestAddKeepsTheOrderTheFlagGave(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}

	_, err := profileflow.Add(profileflow.AddParams{
		Context: ctx,
		Request: profileflow.AddRequest{
			Initial: domain.ProfileConfig{Name: "dev", Jobs: []string{"seed", "api"}},
			Config:  declared(),
		},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	saved := load(t, ctx.StateDir)
	if len(saved.Profiles) != 1 {
		t.Fatalf("saved %d profiles, want 1", len(saved.Profiles))
	}
	if got := saved.Profiles[0].Jobs; len(got) != 2 || got[0] != "seed" || got[1] != "api" {
		t.Errorf("jobs = %v, want the order the flag gave", got)
	}
	if saved.Profiles[0].Default {
		t.Error("a profile nobody asked to make default became one")
	}
}

// The order question offers exactly what the step before selected, never the
// whole catalogue again.
func TestOrderStepOffersOnlyTheSelectedJobs(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			"run.profile.name":    "dev",
			"run.profile.default": domain.RunProfileNoValue,
		},
		Sets: map[string][]string{
			"run.profile.jobs":  {"api", "seed"},
			"run.profile.order": {"seed", "api"},
		},
	}

	ctx := flow.Context{StateDir: t.TempDir()}
	if _, err := profileflow.Add(profileflow.AddParams{
		Context:   ctx,
		Request:   profileflow.AddRequest{Config: declared()},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	options := prompter.Content["run.profile.order"].Options
	if len(options) != 2 {
		t.Fatalf("the order step offers %d rows, want the two jobs selected", len(options))
	}
	if load(t, ctx.StateDir).Profiles[0].Jobs[0] != "seed" {
		t.Error("the answered order was not what the profile recorded")
	}
}

// Only one profile can be the default, and taking it means saying whose it was.
func TestAddAsDefaultTakesItFromTheProfileThatHeldIt(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	cfg := declared()
	cfg.Profiles = []domain.ProfileConfig{{Name: "old", Jobs: []string{"api"}, Default: true}}

	if _, err := profileflow.Add(profileflow.AddParams{
		Context: ctx,
		Request: profileflow.AddRequest{
			Initial: domain.ProfileConfig{Name: "new", Jobs: []string{"web"}, Default: true},
			Config:  cfg,
		},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	saved := load(t, ctx.StateDir)
	for _, profile := range saved.Profiles {
		if profile.Name == "old" && profile.Default {
			t.Error("two profiles hold the default at once")
		}
		if profile.Name == "new" && !profile.Default {
			t.Error("the new profile did not take the default it asked for")
		}
	}
}

func load(t *testing.T, stateDir string) domain.RunConfig {
	t.Helper()
	cfg, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load run.toml: %v", err)
	}
	return cfg
}

// The order step is where a profile's job list comes from, so a surface that
// does not read it back writes a profile that starts nothing — which
// `ValidateRun` accepts. Both surfaces must answer it; this pins the flow's
// side, `flowui`'s answerOf pins the CLI's.
func TestOrderStepIsWhatTheProfileRecords(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			"run.profile.name":    "dev",
			"run.profile.default": domain.RunProfileNoValue,
		},
		Sets: map[string][]string{
			"run.profile.jobs":  {"api", "web", "seed"},
			"run.profile.order": {"seed", "web", "api"},
		},
	}

	ctx := flow.Context{StateDir: t.TempDir()}
	if _, err := profileflow.Add(profileflow.AddParams{
		Context:   ctx,
		Request:   profileflow.AddRequest{Config: declared()},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got := load(t, ctx.StateDir).Profiles[0].Jobs
	if len(got) != 3 {
		t.Fatalf("jobs = %v, want the three that were ordered — a profile that starts nothing was written", got)
	}
	if got[0] != "seed" || got[2] != "api" {
		t.Errorf("jobs = %v, want the answered order", got)
	}
}
