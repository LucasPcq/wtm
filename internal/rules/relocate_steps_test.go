package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestRelocateStepStartResolvesTheAdoptionParent(t *testing.T) {
	step := domain.RelocateStep{Branch: "feat/x", Status: domain.RelocateStatusAdopt, Adopt: true}
	cases := []struct {
		name    string
		step    domain.RelocateStep
		parents map[string]string
		want    string
	}{
		{name: "chosen", step: step, parents: map[string]string{"feat/x": "develop"}, want: "develop"},
		{name: "planned", step: domain.RelocateStep{Branch: "feat/x", Adopt: true, Parent: "release"}, want: "release"},
		{name: "base", step: step, parents: map[string]string{"feat/x": ""}, want: "main"},
		{name: "not adopted", step: domain.RelocateStep{Branch: "feat/x", Status: domain.RelocateStatusMove}, parents: map[string]string{"feat/x": "develop"}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rules.RelocateStepStart(rules.RelocateStepStartParams{Step: tc.step, Parents: tc.parents, BaseBranch: "main"})
			if got.Parent != tc.want {
				t.Errorf("parent = %q, want %q", got.Parent, tc.want)
			}
		})
	}
}

func TestRelocateStepStartExplainsAnUninspectableWorktree(t *testing.T) {
	got := rules.RelocateStepStart(rules.RelocateStepStartParams{Step: domain.RelocateStep{Branch: "feat/x", Status: domain.RelocateStatusError}})
	if got.Status != domain.RelocateStatusError || got.Detail != domain.RelocateUninspectableDetail {
		t.Errorf("got %+v", got)
	}
}

func TestRelocateStepDone(t *testing.T) {
	cases := map[domain.RelocateStatus]domain.RelocateStep{
		domain.RelocateStatusMoved:        {Status: domain.RelocateStatusMove},
		domain.RelocateStatusMovedAdopted: {Status: domain.RelocateStatusMove, Adopt: true},
		domain.RelocateStatusAdopted:      {Status: domain.RelocateStatusAdopt, Adopt: true},
	}
	for want, step := range cases {
		if got := rules.RelocateStepDone(step); got != want {
			t.Errorf("RelocateStepDone(%+v) = %q, want %q", step, got, want)
		}
	}
}

func TestRelocateStepFailed(t *testing.T) {
	got := rules.RelocateStepFailed(domain.RelocateStepResult{Branch: "feat/x", Status: domain.RelocateStatusMove}, errors.New("boom"))
	if got.Status != domain.RelocateStatusError || got.Detail != "boom" {
		t.Errorf("got %+v", got)
	}
}
