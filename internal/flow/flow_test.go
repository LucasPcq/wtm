package flow

import (
	"errors"
	"testing"
)

func TestConfirmDescriptionFoldsTheWarningIn(t *testing.T) {
	got := ConfirmDescription(ConfirmParams{Description: "review this", Warning: "cannot be undone"})
	if want := "review this\ncannot be undone"; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}

func TestConfirmDescriptionWithoutAWarningStaysThePlainDescription(t *testing.T) {
	if got := ConfirmDescription(ConfirmParams{Description: "review this"}); got != "review this" {
		t.Errorf("description = %q, want the description unchanged", got)
	}
}

func TestConfirmDescriptionWithOnlyAWarningIsTheWarning(t *testing.T) {
	if got := ConfirmDescription(ConfirmParams{Warning: "cannot be undone"}); got != "cannot be undone" {
		t.Errorf("description = %q, want the warning alone", got)
	}
}

func TestCheckEntryTrimsAndValidatesAgainstTheList(t *testing.T) {
	var seen EntryCheck
	step := Step{Kind: StepTextList, ValidateEntry: func(check EntryCheck) error {
		seen = check
		if check.Entry == "taken" {
			return errors.New("taken")
		}
		return nil
	}}

	got, err := CheckEntry(step, EntryCheck{Entry: "  feat/a ", Entries: []string{"feat/b"}})
	if err != nil || got != "feat/a" {
		t.Fatalf("CheckEntry = %q, %v; want the trimmed entry accepted", got, err)
	}
	if seen.Entry != "feat/a" || len(seen.Entries) != 1 {
		t.Errorf("validator saw %+v, want the trimmed entry and the list so far", seen)
	}
	if _, err := CheckEntry(step, EntryCheck{Entry: "taken"}); err == nil {
		t.Error("the step's validator should refuse")
	}
	if _, err := CheckEntry(step, EntryCheck{Entry: "   "}); err == nil {
		t.Error("a blank entry should be refused before the validator runs")
	}
}

func TestCheckEntryWithoutValidatorOnlyRefusesBlank(t *testing.T) {
	if got, err := CheckEntry(Step{Kind: StepTextList}, EntryCheck{Entry: "x"}); err != nil || got != "x" {
		t.Errorf("CheckEntry = %q, %v", got, err)
	}
}
