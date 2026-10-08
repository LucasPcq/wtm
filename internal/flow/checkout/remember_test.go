package checkout

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

func TestARememberedAnswerIsSettledAndTheRecapStillNamesIt(t *testing.T) {
	f := flowFor(t, recapInput{PR: domain.PRInfo{Number: 7, Branch: "feat/pr", BaseBranch: "main"}, IsolationApplies: true})
	f.ctx.Config.Project.Wizard.Remembered = map[string]string{
		domain.RememberEnvStrategy: string(domain.EnvStrategyMain),
		domain.RememberIsolation:   string(domain.IsolationVerbatim),
	}

	answers, err := (flow.Unattended{}).Ask(f.session(t.Context()))
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	recap := f.recap(answers)
	for _, line := range []string{
		domain.RecapFieldEnv + "main" + domain.RecapRememberedSuffix,
		domain.RecapFieldIsolation + domain.IsolationSummaryVerbatim + domain.RecapRememberedSuffix,
		domain.RecapRememberedHint,
	} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should contain %q", recap, line)
		}
	}
	if origins := f.origins(answers); origins[domain.RememberEnvStrategy] != domain.AnswerOriginRemembered || origins[domain.RememberIsolation] != domain.AnswerOriginRemembered {
		t.Errorf("origins = %v, want both remembered", origins)
	}
}

func TestCheckoutFlagsWinOverTheRememberedAnswers(t *testing.T) {
	f := flowFor(t, recapInput{PR: domain.PRInfo{Number: 7, Branch: "feat/pr", BaseBranch: "main"}, IsolationApplies: true, EnvFlag: "example", IsolationFlag: domain.IsolationIsolated})
	f.ctx.Config.Project.Wizard.Remembered = map[string]string{
		domain.RememberEnvStrategy: string(domain.EnvStrategyMain),
		domain.RememberIsolation:   string(domain.IsolationVerbatim),
	}

	answers, err := (flow.Unattended{}).Ask(f.session(t.Context()))
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answers.Value(KeyEnv) != "example" || answers.Value(KeyIsolation) != string(domain.IsolationIsolated) {
		t.Errorf("env = %q, isolation = %q, want the flags'", answers.Value(KeyEnv), answers.Value(KeyIsolation))
	}
}
