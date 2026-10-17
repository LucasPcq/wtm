package extract

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/create"
)

func TestTheEmbeddedQuestionsRecallTheirAnswer(t *testing.T) {
	in := recapInput{SourceArg: "feat/src", FilesFlag: []string{"a.go"}, CreateNew: true, Branch: "feat/new", From: "main"}
	f := recapFlowFor(t, in)
	f.ctx.Config.Project.Wizard.Remembered = map[string]string{domain.RememberIsolation: string(domain.IsolationVerbatim)}
	f.create = f.embed(t.Context())

	var recalled string
	for _, step := range f.session(t.Context()).Steps {
		if step.Key == create.KeyIsolation {
			recalled = step.Memory.Value
		}
	}
	if recalled != string(domain.IsolationVerbatim) {
		t.Errorf("isolation step recalls %q, want the remembered verbatim", recalled)
	}

	answers := recapAnswersFor(in).With(create.KeyIsolation, flow.Answer{Value: string(domain.IsolationVerbatim), Recalled: true})
	content, err := f.recapStep(t.Context()).Build(answers)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		domain.RecapFieldIsolation + domain.IsolationSummaryVerbatim + domain.RecapRememberedSuffix,
		domain.RecapRememberedHint,
	} {
		if !strings.Contains(content.Description, line) {
			t.Errorf("recap %q should contain %q", content.Description, line)
		}
	}
}
