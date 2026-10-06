package rules

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// sharedDatabasesConfig is the shape the step was written for: shared databases named
// after the app they serve, and tasks named after the same app.
func sharedDatabasesConfig() domain.RunConfig {
	shared := func(name string) domain.JobConfig {
		return domain.JobConfig{Name: name, Kind: domain.JobKindService, Cmd: "docker compose up -d " + name, Scope: domain.JobScopeShared}
	}
	task := func(name string) domain.JobConfig {
		return domain.JobConfig{Name: name, Kind: domain.JobKindTask, Cmd: "pnpm run " + name}
	}
	return domain.RunConfig{Jobs: []domain.JobConfig{
		shared("postgres-billing"), shared("postgres-orders"), shared("keycloak"),
		{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		task("build:shared"), task("orm:billing:reset"), task("orm:orders:init"),
	}}
}

func TestTouchChoicesProposeWhatTheNameSays(t *testing.T) {
	choices := TouchChoices(TouchChoicesParams{Config: sharedDatabasesConfig()})

	got := map[string][]string{}
	for _, choice := range choices {
		got[choice.Job] = choice.Touches
		if choice.Options[0] != "" || !slices.Contains(choice.Options, "keycloak") || slices.Contains(choice.Options, "dev") {
			t.Errorf("%s options = %v, want none first then every data service, dev excluded", choice.Job, choice.Options)
		}
	}
	want := map[string][]string{
		"build:shared":      nil,
		"orm:billing:reset": {"postgres-billing"},
		"orm:orders:init":   {"postgres-orders"},
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want one per task", got)
	}
	for job, touches := range want {
		if !slices.Equal(got[job], touches) {
			t.Errorf("%s touches = %v, want %v", job, got[job], touches)
		}
	}
}

// What run.toml already says outranks what the name proposes.
func TestTouchChoicesKeepWhatTheConfigSays(t *testing.T) {
	cfg := sharedDatabasesConfig()
	cfg.Jobs[5].Touches = []string{"keycloak", "postgres-billing"}
	for _, choice := range TouchChoices(TouchChoicesParams{Config: cfg, Existing: cfg}) {
		if choice.Job == "orm:billing:reset" && !slices.Equal(choice.Touches, []string{"keycloak", "postgres-billing"}) {
			t.Errorf("touches = %v, want the config's kept whole", choice.Touches)
		}
	}
}

func TestTouchChoicesNeedAServiceHoldingData(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		{Name: "db:reset", Kind: domain.JobKindTask, Cmd: "pnpm db:reset"},
	}}
	if choices := TouchChoices(TouchChoicesParams{Config: cfg}); choices != nil {
		t.Errorf("choices = %+v, want the step skipped", choices)
	}
}

// Two candidates is a guess: nothing is proposed.
func TestProposedTouchesStaysSilentWhenAmbiguous(t *testing.T) {
	got := ProposedTouches(ProposedTouchesParams{Task: "billing:reset", Services: []string{"postgres-billing", "redis-billing"}})
	if got != nil {
		t.Errorf("proposed %v, want nothing", got)
	}
}

func TestApplyTouchChoices(t *testing.T) {
	cfg := sharedDatabasesConfig()
	cfg.Jobs[4].Touches = []string{"keycloak"}
	cfg.Jobs[6].Touches = []string{"postgres-orders"}
	out := ApplyTouchChoices(ApplyTouchChoicesParams{Config: cfg, Choices: []domain.JobTouchChoice{
		{Job: "build:shared", Touches: nil},
		{Job: "orm:billing:reset", Touches: []string{"postgres-billing", "gone"}},
	}})

	byName := map[string][]string{}
	for _, job := range out.Jobs {
		byName[job.Name] = job.Touches
	}
	if byName["build:shared"] != nil {
		t.Errorf("build:shared = %v, want none: the step asked and none is an answer", byName["build:shared"])
	}
	if !slices.Equal(byName["orm:billing:reset"], []string{"postgres-billing"}) {
		t.Errorf("orm:billing:reset = %v, want the answer without the service that no longer exists", byName["orm:billing:reset"])
	}
	if !slices.Equal(byName["orm:orders:init"], []string{"postgres-orders"}) {
		t.Errorf("orm:orders:init = %v, want a task the step did not list left alone", byName["orm:orders:init"])
	}
	if cfg.Jobs[4].Touches == nil {
		t.Error("the config given was written through")
	}
}
