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
