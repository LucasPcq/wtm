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
