package flow_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
)

func TestMergeContentKeepsWhatTheBuildDecides(t *testing.T) {
	step := flow.Step{Title: "static", Options: []flow.Option{{Value: "a"}}}
	built := flow.StepContent{
		Options:         []flow.Option{{Value: "main"}, {Value: "feat"}},
		Start:           "feat",
		Blockers:        []flow.Blocker{{Key: "dirty"}},
		ExcludeBranches: []string{"main"},
	}

	got := flow.MergeContent(step, built)
	if got.Title != "static" || len(got.Options) != 2 || got.Start != "feat" ||
		len(got.Blockers) != 1 || len(got.ExcludeBranches) != 1 {
		t.Errorf("content = %+v, want the static title and everything the build decided", got)
	}
}

func TestMergeContentCarriesEntries(t *testing.T) {
	merged := flow.MergeContent(flow.Step{Title: "t"}, flow.StepContent{Entries: []string{"a", "b"}})
	if len(merged.Entries) != 2 || merged.Entries[0] != "a" || merged.Entries[1] != "b" {
		t.Errorf("entries = %v, want the built pre-fill", merged.Entries)
	}
}
