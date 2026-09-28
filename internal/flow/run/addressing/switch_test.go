package addressing

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

var portsConfig = strings.Replace(namedConfig, `addressing = "names"`, `addressing = "ports"`, 1)

type switchRecorder struct {
	flowtest.Recorder
	outcomes []SwitchOutcome
}

func (r *switchRecorder) Switched(outcome SwitchOutcome) error {
	r.outcomes = append(r.outcomes, outcome)
	return nil
}

func (r repo) switchParams(request SwitchRequest, prompter flow.Prompter) (SwitchParams, *switchRecorder) {
	cfg, err := runconfig.Load(r.stateDir)
	if err != nil {
		panic(err)
	}
	request.Config = cfg
	presenter := &switchRecorder{}
	return SwitchParams{
		Context:   r.params().Context,
		Request:   request,
		Prompter:  prompter,
		Presenter: presenter,
	}, presenter
}

func (r repo) env(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(r.dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func (r repo) addressing(t *testing.T) domain.Addressing {
	t.Helper()
	cfg, err := runconfig.Load(r.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Addressing
}

func TestSwitchWritesTheModeAndSettlesTheWorktrees(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	params, presenter := r.switchParams(SwitchRequest{Mode: domain.AddressingNames}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if got := r.addressing(t); got != domain.AddressingNames {
		t.Errorf("run.toml addressing = %q, want names", got)
	}
	if !strings.Contains(r.env(t), ".localhost") {
		t.Errorf(".env = %q, want the value moved onto the published name", r.env(t))
	}
	if !outcome.Changed || outcome.Previous != domain.AddressingPorts || !slices.Equal(outcome.Settled, []string{"main"}) {
		t.Errorf("outcome = %+v, want ports → names with main settled", outcome)
	}
	if len(presenter.outcomes) != 1 {
		t.Errorf("concluded %d time(s), want once", len(presenter.outcomes))
	}
}

func TestSwitchKeepEnvLeavesTheFilesOutOfStep(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingNames, KeepEnv: true}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if got := r.addressing(t); got != domain.AddressingNames {
		t.Errorf("run.toml addressing = %q, want names", got)
	}
	if r.env(t) != "VITE_API_URL=http://localhost:4001\n" {
		t.Errorf(".env = %q, want it untouched", r.env(t))
	}
	if len(outcome.Settled) != 0 || !slices.Equal(outcome.Pending, []string{"main"}) {
		t.Errorf("outcome = %+v, want main left pending", outcome)
	}
}

// The mode already in place is not a no-op while a worktree still spells the
// other one: that is the drift a previous --keep-env left behind.
func TestSwitchToTheCurrentModeStillSettlesTheDrift(t *testing.T) {
	r := newRepo(t, namedConfig, "VITE_API_URL=http://localhost:4001\n")
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingNames}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if outcome.Changed {
		t.Error("run.toml already said names, nothing to write")
	}
	if !slices.Equal(outcome.Settled, []string{"main"}) {
		t.Errorf("settled = %v, want main", outcome.Settled)
	}
}

func TestSwitchDeclinedLeavesTheFilesPending(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		stepMode:   string(domain.AddressingNames),
		stepSettle: settleNo,
	}}
	params, _ := r.switchParams(SwitchRequest{}, prompter)

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if prompter.AskedKeys() != stepMode+","+stepSettle {
		t.Errorf("asked %q, want the mode then the settle question, as steps of one session", prompter.AskedKeys())
	}
	if !strings.Contains(prompter.Content[stepSettle].Title, "1 worktree") {
		t.Errorf("settle title = %q, want the count read under the mode just picked", prompter.Content[stepSettle].Title)
	}
	if r.env(t) != "VITE_API_URL=http://localhost:4001\n" {
		t.Errorf(".env = %q, want it untouched", r.env(t))
	}
	if !outcome.Changed || !slices.Equal(outcome.Pending, []string{"main"}) {
		t.Errorf("outcome = %+v, want the switch written and main pending", outcome)
	}
}

func TestSwitchUnattendedNeedsTheMode(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	params, _ := r.switchParams(SwitchRequest{}, flow.Unattended{})

	_, err := Switch(params)
	if err == nil || !strings.Contains(err.Error(), "argument") {
		t.Fatalf("err = %v, want the refusal naming the argument", err)
	}
	if got := r.addressing(t); got != domain.AddressingPorts {
		t.Errorf("run.toml addressing = %q, want it untouched", got)
	}
}

func TestSwitchRefusesAnUnknownMode(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	params, _ := r.switchParams(SwitchRequest{Mode: "hosts"}, flow.Unattended{})

	if _, err := Switch(params); err == nil || !strings.Contains(err.Error(), `"hosts"`) {
		t.Fatalf("err = %v, want the unknown mode named", err)
	}
}

// Nothing to move means nothing to ask: the question would be a yes with no
// consequence.
func TestSwitchSkipsTheSettleStepWhenEveryWorktreeIsInStep(t *testing.T) {
	r := newRepo(t, portsConfig, "VITE_API_URL=http://localhost:4001\n")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{stepMode: string(domain.AddressingPorts)}}
	params, _ := r.switchParams(SwitchRequest{}, prompter)

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if prompter.AskedKeys() != stepMode {
		t.Errorf("asked %q, want only the mode", prompter.AskedKeys())
	}
	if outcome.Changed || len(outcome.Settled)+len(outcome.Pending) != 0 {
		t.Errorf("outcome = %+v, want nothing done", outcome)
	}
}
