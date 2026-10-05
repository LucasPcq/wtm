package probes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/probes"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func config() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000},
	}}}
}

// web answered on its base port and not on the one it was given: it never
// reads the variable, and will warn the same way at every run.
func baseBound() runlogs.Outcomes {
	return runlogs.Outcomes{{Steps: 1, Probes: []domain.PortProbe{{
		Job: "web", Name: "PORT", Port: 3010, Status: domain.PortSilent, BaseListening: 3000,
	}}}}
}

type fixture struct {
	stateDir  string
	prompter  *flowtest.ScriptedPrompter
	presenter *flowtest.Recorder
}

func offer(t *testing.T, fx fixture, results runlogs.Outcomes) domain.RunConfig {
	t.Helper()
	cfg, err := probes.OfferToSilence(t.Context(), probes.Params{
		Context:   flow.Context{StateDir: fx.stateDir},
		Prompter:  fx.prompter,
		Presenter: fx.presenter,
		Config:    config(),
		Results:   results,
	})
	if err != nil {
		t.Fatalf("OfferToSilence: %v", err)
	}
	return cfg
}

func newFixture(t *testing.T, confirmed bool) fixture {
	return fixture{
		stateDir:  t.TempDir(),
		prompter:  &flowtest.ScriptedPrompter{Confirmed: confirmed},
		presenter: &flowtest.Recorder{},
	}
}

func TestAnAcceptedOfferWritesProbeFalseAndSaysSo(t *testing.T) {
	fx := newFixture(t, true)

	cfg := offer(t, fx, baseBound())

	if fx.prompter.Confirms != 1 {
		t.Fatalf("confirms = %d, want the offer made once", fx.prompter.Confirms)
	}
	if probe := cfg.Jobs[0].Probe; probe == nil || *probe {
		t.Errorf("returned config probe = %v, want false", probe)
	}
	saved, err := runconfig.Load(fx.stateDir)
	if err != nil {
		t.Fatalf("load run.toml: %v", err)
	}
	if probe := saved.Jobs[0].Probe; probe == nil || *probe {
		t.Errorf("run.toml probe = %v, want false", probe)
	}
	if len(fx.presenter.Statuses) != 1 {
		t.Errorf("statuses = %+v, want the change announced", fx.presenter.Statuses)
	}
}

func TestADeclinedOfferChangesNothing(t *testing.T) {
	fx := newFixture(t, false)

	cfg := offer(t, fx, baseBound())

	if cfg.Jobs[0].Probe != nil {
		t.Errorf("probe = %v, want it left unset", *cfg.Jobs[0].Probe)
	}
	if _, err := os.Stat(filepath.Join(fx.stateDir, domain.RunFileName)); !os.IsNotExist(err) {
		t.Error("run.toml written after a refusal")
	}
}

// After an abort the question is why the run stopped, not whether to hear less.
func TestAnAbortedRunIsNotOffered(t *testing.T) {
	fx := newFixture(t, true)
	results := baseBound()
	results[0].Failed = "web"

	offer(t, fx, results)

	if fx.prompter.Confirms != 0 {
		t.Errorf("confirms = %d, want nothing asked after an abort", fx.prompter.Confirms)
	}
}

func TestNobodyToAskIsNotOffered(t *testing.T) {
	cfg, err := probes.OfferToSilence(t.Context(), probes.Params{
		Context:   flow.Context{StateDir: t.TempDir()},
		Prompter:  flow.Unattended{},
		Presenter: &flowtest.Recorder{},
		Config:    config(),
		Results:   baseBound(),
	})
	if err != nil {
		t.Fatalf("OfferToSilence: %v", err)
	}
	if cfg.Jobs[0].Probe != nil {
		t.Error("an unattended run silenced a probe")
	}
}
