package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestRemoveJobRetireLeJobEtSesReferences(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService},
			{Name: "api", Kind: domain.JobKindService},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"web", "api"}},
			{Name: "front", Jobs: []string{"web"}},
		},
		EnvPorts: []domain.EnvPortLink{
			{File: ".env", Key: "PORT", Job: "web", Port: domain.PortNameDefault},
			{File: ".env", Key: "API_PORT", Job: "api", Port: "API_PORT"},
		},
	}

	got, effect := rules.RemoveJob(cfg, "web")

	if len(got.Jobs) != 1 || got.Jobs[0].Name != "api" {
		t.Errorf("jobs = %+v, want seulement api", got.Jobs)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "dev" {
		t.Errorf("profils = %+v, want seulement dev", got.Profiles)
	}
	if len(got.Profiles[0].Jobs) != 1 || got.Profiles[0].Jobs[0] != "api" {
		t.Errorf("jobs de dev = %v, want [api]", got.Profiles[0].Jobs)
	}
	if len(got.EnvPorts) != 1 || got.EnvPorts[0].Key != "API_PORT" {
		t.Errorf("env_port = %+v, want seulement API_PORT", got.EnvPorts)
	}
	if !effect.Removed {
		t.Error("Removed = false, want true")
	}
	if len(effect.EmptiedProfiles) != 1 || effect.EmptiedProfiles[0] != "front" {
		t.Errorf("profils vidés = %v, want [front]", effect.EmptiedProfiles)
	}
	if len(effect.EnvPorts) != 1 || effect.EnvPorts[0] != "PORT" {
		t.Errorf("liens retirés = %v, want [PORT]", effect.EnvPorts)
	}
}

// Retirer un job absent ne touche à rien : l'appelant doit pouvoir le dire
// sans avoir comparé les deux configs.
func TestRemoveJobSurUnJobAbsentNeChangeRien(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	got, effect := rules.RemoveJob(cfg, "absent")

	if effect.Removed {
		t.Error("Removed = true, want false")
	}
	if len(got.Jobs) != 1 {
		t.Errorf("jobs = %+v, want inchangés", got.Jobs)
	}
}

// La config d'entrée n'est jamais modifiée en place : l'init recalcule la
// sienne à chaque affichage de step, et partagerait sinon des slices amputées.
func TestRemoveJobNeModifiePasLaConfigDentree(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web"}, {Name: "api"}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"web", "api"}}},
	}

	rules.RemoveJob(cfg, "web")

	if len(cfg.Jobs) != 2 {
		t.Errorf("jobs d'entrée = %+v, want inchangés", cfg.Jobs)
	}
	if len(cfg.Profiles[0].Jobs) != 2 {
		t.Errorf("jobs du profil d'entrée = %v, want inchangés", cfg.Profiles[0].Jobs)
	}
}

// A job is named in three places, and a runner's `runs` is the third. Leaving it
// behind made the removal fail the very validation it had just invalidated, so
// a job any runner started could not be removed at all.
func TestRemoveJobStripsTheRunnersThatStartedIt(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev", Runs: []string{"api", "web"}},
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm --filter api dev"},
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm --filter web dev"},
	}}

	out, effect := rules.RemoveJob(cfg, "api")

	if len(effect.Runners) != 1 || effect.Runners[0] != "dev" {
		t.Fatalf("runners = %v, want the runner that named it", effect.Runners)
	}
	runner, ok := rules.FindJob(out, "dev")
	if !ok {
		t.Fatal("the runner itself was removed")
	}
	if len(runner.Runs) != 1 || runner.Runs[0] != "web" {
		t.Errorf("runs = %v, want the removed job stripped and the rest kept", runner.Runs)
	}
	if _, errs := rules.ValidateRun(out); len(errs) > 0 {
		t.Errorf("the config a removal produced does not validate: %v", errs)
	}
}

// A runner left with nothing to start keeps its own declaration: it is a job in
// its own right, unlike a profile that starts nothing.
func TestRemoveJobLeavesARunnerWithNoChildrenStanding(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev", Runs: []string{"api"}},
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm --filter api dev"},
	}}

	out, _ := rules.RemoveJob(cfg, "api")

	runner, ok := rules.FindJob(out, "dev")
	if !ok {
		t.Fatal("the runner was removed along with its only child")
	}
	if runner.Runs != nil {
		t.Errorf("runs = %v, want it dropped rather than left empty", runner.Runs)
	}
	if _, errs := rules.ValidateRun(out); len(errs) > 0 {
		t.Errorf("validation: %v", errs)
	}
}
