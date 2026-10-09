package create

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// rememberingContext is a project on disk whose isolation question applies, so
// both rememberable questions of create are put, with remembered answers.
func rememberingContext(t *testing.T, remembered map[string]string) flow.Context {
	t.Helper()
	ctx := testContext(t)
	ctx.Config.Project.Wizard.Remembered = remembered
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: portedConfig("")}); err != nil {
		t.Fatalf("write run config: %v", err)
	}
	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: ctx.StateDir, Config: ctx.Config.Project}); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	return ctx
}

func rememberedOnDisk(t *testing.T, ctx flow.Context) map[string]string {
	t.Helper()
	project, err := config.LoadProjectRaw(ctx.StateDir)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	return project.Wizard.Remembered
}

func TestARememberedAnswerIsNotAskedAndTheRecapStillNamesIt(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{
		domain.RememberEnvStrategy: string(domain.EnvStrategyMain),
		domain.RememberIsolation:   string(domain.IsolationVerbatim),
	})
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyBranch: "feat/x", KeySource: "main", KeyRecap: confirmCreate}}

	outcome, err := Run(t.Context(), Params{Context: ctx, Prompter: prompter, Presenter: newRecorder()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if want := strings.Join([]string{KeyBranch, KeySource, KeyRecap}, ","); prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	recap := prompter.Content[KeyRecap].Description
	for _, line := range []string{
		"Env:       main" + domain.RecapRememberedSuffix,
		"Isolation: " + domain.IsolationSummaryVerbatim + domain.RecapRememberedSuffix,
		domain.RecapRememberedHint,
	} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should contain %q", recap, line)
		}
	}
	result := outcome.Results[0]
	if result.Metadata.EnvStrategy != domain.EnvStrategyMain || result.Isolation != domain.IsolationVerbatim {
		t.Errorf("env = %q, isolation = %q, want the remembered main and verbatim", result.Metadata.EnvStrategy, result.Isolation)
	}
}

func TestAFlagOverridesTheRememberedAnswer(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{
		domain.RememberEnvStrategy: string(domain.EnvStrategyMain),
		domain.RememberIsolation:   string(domain.IsolationVerbatim),
	})
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmCreate}}

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main", EnvFrom: string(domain.EnvStrategyExample), Isolation: domain.IsolationIsolated},
		Prompter:  prompter,
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	result := outcome.Results[0]
	if result.Metadata.EnvStrategy != domain.EnvStrategyExample || result.Isolation != domain.IsolationIsolated {
		t.Errorf("env = %q, isolation = %q, want the flags' example and isolated", result.Metadata.EnvStrategy, result.Isolation)
	}
	if recap := prompter.Content[KeyRecap].Description; strings.Contains(recap, domain.RecapRememberedSuffix) {
		t.Errorf("recap %q marks a flag's answer as remembered", recap)
	}
	if result.Origins[domain.RememberEnvStrategy] != domain.AnswerOriginFlag || result.Origins[domain.RememberIsolation] != domain.AnswerOriginFlag {
		t.Errorf("origins = %v, want both from a flag", result.Origins)
	}
}

func TestTickingAlwaysRemembersTheAnswerOnceConfirmed(t *testing.T) {
	ctx := rememberingContext(t, nil)
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			KeyBranch:    "feat/x",
			KeySource:    "main",
			KeyEnv:       string(domain.EnvStrategyParent),
			KeyIsolation: string(domain.IsolationIsolated),
			KeyRecap:     confirmCreate,
		},
		Remember: map[string]bool{KeyEnv: true},
	}

	if _, err := Run(t.Context(), Params{Context: ctx, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, "Env:       parent"+domain.RecapWillRememberSuffix) {
		t.Errorf("recap %q should say the env strategy will be remembered", recap)
	}
	remembered := rememberedOnDisk(t, ctx)
	if len(remembered) != 1 || remembered[domain.RememberEnvStrategy] != string(domain.EnvStrategyParent) {
		t.Errorf("remembered = %v, want env_strategy=parent alone", remembered)
	}
}

func TestAnAbortedSessionRemembersNothing(t *testing.T) {
	ctx := rememberingContext(t, nil)
	prompter := &flowtest.ScriptedPrompter{Abort: true, Remember: map[string]bool{KeyEnv: true}}

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Branches: []string{"feat/x"}}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if remembered := rememberedOnDisk(t, ctx); len(remembered) != 0 {
		t.Errorf("remembered = %v after an abort, want nothing", remembered)
	}
}

func TestAskAsksAgainAndKeepsWhatStaysTicked(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{
		domain.RememberEnvStrategy: string(domain.EnvStrategyMain),
		domain.RememberIsolation:   string(domain.IsolationVerbatim),
	})
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			KeyEnv:       string(domain.EnvStrategyParent),
			KeyIsolation: string(domain.IsolationIsolated),
			KeyRecap:     confirmCreate,
		},
		Remember: map[string]bool{KeyIsolation: false},
	}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main", Ask: true},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if want := strings.Join([]string{KeyEnv, KeyIsolation, KeyRecap}, ","); prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	remembered := rememberedOnDisk(t, ctx)
	if len(remembered) != 1 || remembered[domain.RememberEnvStrategy] != string(domain.EnvStrategyParent) {
		t.Errorf("remembered = %v, want the env strategy changed to parent and the isolation forgotten", remembered)
	}
}

func TestAnUnattendedRunTakesTheRememberedAnswerAndSaysSo(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{domain.RememberEnvStrategy: string(domain.EnvStrategyMain)})

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main"},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	result := outcome.Results[0]
	if result.Metadata.EnvStrategy != domain.EnvStrategyMain {
		t.Errorf("env = %q, want the remembered main", result.Metadata.EnvStrategy)
	}
	want := map[string]domain.AnswerOrigin{
		domain.RememberEnvStrategy: domain.AnswerOriginRemembered,
		domain.RememberIsolation:   domain.AnswerOriginDefault,
	}
	if len(result.Origins) != len(want) || result.Origins[domain.RememberEnvStrategy] != want[domain.RememberEnvStrategy] || result.Origins[domain.RememberIsolation] != want[domain.RememberIsolation] {
		t.Errorf("origins = %v, want %v", result.Origins, want)
	}
}

func TestAskUnattendedIgnoresTheMemory(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{domain.RememberEnvStrategy: string(domain.EnvStrategyMain)})

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main", Ask: true},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := outcome.Results[0].Origins[domain.RememberEnvStrategy]; got != domain.AnswerOriginConfig {
		t.Errorf("env origin = %q, want the config's under --ask", got)
	}
	if remembered := rememberedOnDisk(t, ctx); remembered[domain.RememberEnvStrategy] != string(domain.EnvStrategyMain) {
		t.Errorf("remembered = %v, want the memory untouched by a run that asked nothing", remembered)
	}
}

// A flag always wins: under --ff a remembered "keep" never settles the step.
func TestFastForwardFlagWinsOverARememberedKeep(t *testing.T) {
	f := newFlow(t, Request{FastForward: true}, nil)
	session := flow.Recall(flow.RecallParams{
		Session:    f.session(),
		Remembered: map[string]string{domain.RememberSourceUpdate: domain.SourceUpdateKeep},
	})

	for _, step := range session.Steps {
		if step.Key != KeySourceUpdate {
			continue
		}
		step.Skip = nil
		answer, _ := flow.Settle(step, flow.Answers{})
		if answer.Value != updateFastForward || !answer.Given || answer.Recalled {
			t.Errorf("answer = %+v, want --ff to settle the step over the memory", answer)
		}
	}
}

// The config-default row answers "", which no memory holds: ticking it must not
// promise in the recap what the run then does the opposite of.
func TestTickingTheConfigDefaultSaysItForgetsInsteadOfRemembering(t *testing.T) {
	ctx := rememberingContext(t, map[string]string{domain.RememberEnvStrategy: string(domain.EnvStrategyParent)})
	prompter := &flowtest.ScriptedPrompter{
		Answers:  map[string]string{KeyEnv: "", KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCreate},
		Remember: map[string]bool{KeyEnv: true},
	}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main", Ask: true},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	recap := prompter.Content[KeyRecap].Description
	if !strings.Contains(recap, "Env:       "+domain.EnvSummaryConfigDefault+domain.RecapWillForgetSuffix) {
		t.Errorf("recap %q should say the env strategy will be forgotten", recap)
	}
	if strings.Contains(recap, domain.RecapWillRememberSuffix) {
		t.Errorf("recap %q promises to remember an answer no memory can hold", recap)
	}
	if remembered := rememberedOnDisk(t, ctx); len(remembered) != 0 {
		t.Errorf("remembered = %v, want the env strategy forgotten", remembered)
	}
}
