package addressing

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
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

const portsEnv = "VITE_API_URL=http://localhost:4001\n"

// withFeature adds a linked worktree holding the .env it was copied with — main's
// port, which is not its own.
func (r repo) withFeature(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, r.dir, "worktree", "add", "-b", "feature", path)
	if err := os.WriteFile(filepath.Join(path, ".env"), []byte(portsEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// alignMain is `wtm env main` under names: the one way main's .env is moved
// onto names.
func (r repo) alignMain(t *testing.T) {
	t.Helper()
	if _, err := envports.Settle(envports.Params{
		Context:      r.params().Context,
		Branch:       "main",
		WorktreePath: r.dir,
		Presenter:    &flowtest.Recorder{},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.env(t), ".localhost") {
		t.Fatalf("main .env = %q, want it on names", r.env(t))
	}
}

func (r repo) env(t *testing.T) string {
	t.Helper()
	return readFile(t, filepath.Join(r.dir, ".env"))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
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

func TestSwitchToNamesSettlesTheWorktreesButNotMain(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
	feature := r.withFeature(t)
	params, presenter := r.switchParams(SwitchRequest{Mode: domain.AddressingNames}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if got := r.addressing(t); got != domain.AddressingNames {
		t.Errorf("run.toml addressing = %q, want names", got)
	}
	if body := readFile(t, filepath.Join(feature, ".env")); !strings.Contains(body, ".localhost") {
		t.Errorf("feature .env = %q, want the value moved onto the published name", body)
	}
	if r.env(t) != portsEnv {
		t.Errorf("main .env = %q, want it untouched: only `wtm env main` moves it onto names", r.env(t))
	}
	if !outcome.Changed || !slices.Equal(outcome.Settled, []string{"feature"}) || outcome.MainLeft != "main" {
		t.Errorf("outcome = %+v, want feature settled and main left as is", outcome)
	}
	if len(presenter.outcomes) != 1 {
		t.Errorf("concluded %d time(s), want once", len(presenter.outcomes))
	}
}

// A verbatim worktree keeps the .env it was copied with, whichever way the
// project spells its addresses: the switch does not count it, nor write it.
func TestSwitchLeavesAVerbatimWorktreeAsCopied(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
	feature := r.withFeature(t)
	if err := worktree.SetIsolation(worktree.SetIsolationParams{
		Ref:       worktree.WorktreeRef{ProjectDir: r.dir, StateDir: r.stateDir, Branch: "feature"},
		Isolation: domain.IsolationVerbatim,
	}); err != nil {
		t.Fatalf("SetIsolation: %v", err)
	}
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingNames}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if body := readFile(t, filepath.Join(feature, ".env")); body != portsEnv {
		t.Errorf("feature .env = %q, want it as copied", body)
	}
	if len(outcome.Settled) != 0 || len(outcome.Pending) != 0 {
		t.Errorf("outcome = %+v, want the verbatim worktree neither settled nor pending", outcome)
	}
}

// Back to ports is the inverse of a `wtm env main`: main returns to the state it
// has without wtm, so the pass takes it along.
func TestSwitchToPortsBringsMainBack(t *testing.T) {
	r := newRepo(t, namedConfig, portsEnv)
	r.alignMain(t)
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingPorts}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if r.env(t) != portsEnv {
		t.Errorf("main .env = %q, want its port back", r.env(t))
	}
	if !slices.Equal(outcome.Settled, []string{"main"}) || outcome.MainLeft != "" {
		t.Errorf("outcome = %+v, want main settled", outcome)
	}
}

func TestSwitchKeepEnvLeavesTheFilesOutOfStep(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
	feature := r.withFeature(t)
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingNames, KeepEnv: true}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if got := r.addressing(t); got != domain.AddressingNames {
		t.Errorf("run.toml addressing = %q, want names", got)
	}
	if body := readFile(t, filepath.Join(feature, ".env")); body != portsEnv {
		t.Errorf("feature .env = %q, want it untouched", body)
	}
	if len(outcome.Settled) != 0 || !slices.Equal(outcome.Pending, []string{"feature"}) {
		t.Errorf("outcome = %+v, want feature left pending", outcome)
	}
}

// The mode already in place is not a no-op while a worktree still spells the
// other one: that is the drift a previous --keep-env left behind.
func TestSwitchToTheCurrentModeStillSettlesTheDrift(t *testing.T) {
	r := newRepo(t, namedConfig, portsEnv)
	r.withFeature(t)
	params, _ := r.switchParams(SwitchRequest{Mode: domain.AddressingNames}, flow.Unattended{})

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if outcome.Changed {
		t.Error("run.toml already said names, nothing to write")
	}
	if !slices.Equal(outcome.Settled, []string{"feature"}) {
		t.Errorf("settled = %v, want feature", outcome.Settled)
	}
}

func TestSwitchDeclinedLeavesTheFilesPending(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
	feature := r.withFeature(t)
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
	description := prompter.Content[stepSettle].Description
	if !strings.Contains(description, "1 worktree") || !strings.Contains(description, "main is left out") {
		t.Errorf("settle description = %q, want the count and main named as left out", description)
	}
	if body := readFile(t, filepath.Join(feature, ".env")); body != portsEnv {
		t.Errorf("feature .env = %q, want it untouched", body)
	}
	if !outcome.Changed || !slices.Equal(outcome.Pending, []string{"feature"}) {
		t.Errorf("outcome = %+v, want the switch written and feature pending", outcome)
	}
}

func TestSwitchUnattendedNeedsTheMode(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
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
	r := newRepo(t, portsConfig, portsEnv)
	params, _ := r.switchParams(SwitchRequest{Mode: "hosts"}, flow.Unattended{})

	if _, err := Switch(params); err == nil || !strings.Contains(err.Error(), `"hosts"`) {
		t.Fatalf("err = %v, want the unknown mode named", err)
	}
}

// Nothing the pass may move means nothing to ask: main alone out of step under
// names is its own decision, not a yes with no consequence.
func TestSwitchSkipsTheSettleStepWhenOnlyMainIsOutOfStep(t *testing.T) {
	r := newRepo(t, portsConfig, portsEnv)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{stepMode: string(domain.AddressingNames)}}
	params, _ := r.switchParams(SwitchRequest{}, prompter)

	outcome, err := Switch(params)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if prompter.AskedKeys() != stepMode {
		t.Errorf("asked %q, want only the mode", prompter.AskedKeys())
	}
	if len(outcome.Settled)+len(outcome.Pending) != 0 || outcome.MainLeft != "main" {
		t.Errorf("outcome = %+v, want nothing settled and main left", outcome)
	}
}
