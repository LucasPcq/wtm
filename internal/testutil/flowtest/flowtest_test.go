package flowtest

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
)

func TestScriptedListIsValidatedEntryByEntry(t *testing.T) {
	step := flow.Step{Kind: flow.StepTextList, Key: "k", ValidateEntry: func(check flow.EntryCheck) error {
		for _, e := range check.Entries {
			if e == check.Entry {
				return errors.New("twice")
			}
		}
		return nil
	}}

	ok := &ScriptedPrompter{Sets: map[string][]string{"k": {"a", "b"}}}
	answers, err := ok.Ask(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got := answers.Values("k"); len(got) != 2 {
		t.Errorf("values = %v", got)
	}

	dup := &ScriptedPrompter{Sets: map[string][]string{"k": {"a", "a"}}}
	if _, err := dup.Ask(flow.Session{Steps: []flow.Step{step}}); err == nil {
		t.Error("a duplicate entry should be refused as the real host would")
	}
}

// The wizard builds every step before its first question: a Build that needs an
// earlier answer to succeed aborts the real session, so the double refuses it.
func TestABuildThatNeedsAnEarlierAnswerIsRefused(t *testing.T) {
	first := flow.Step{Kind: flow.StepSelect, Key: "first"}
	second := flow.Step{Kind: flow.StepSelect, Key: "second", Build: func(answers flow.Answers) (flow.StepContent, error) {
		if answers.Value("first") == "" {
			return flow.StepContent{}, errors.New("first not answered")
		}
		return flow.StepContent{}, nil
	}}

	prompter := &ScriptedPrompter{Answers: map[string]string{"first": "a", "second": "b"}}
	if _, err := prompter.Ask(flow.Session{Steps: []flow.Step{first, second}}); err == nil {
		t.Error("a Build failing before the session opens should be refused as the real host would")
	}
}
