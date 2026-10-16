package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBranchCandidateExists(t *testing.T) {
	candidates := []domain.BranchCandidate{
		{Name: "main"},
		{Name: "origin/feature", IsRemote: true},
	}

	if !BranchCandidateExists(candidates, "main") {
		t.Error("expected local branch to be accepted")
	}
	if !BranchCandidateExists(candidates, "origin/feature") {
		t.Error("expected remote ref to be accepted")
	}
	if BranchCandidateExists(candidates, "nope") {
		t.Error("unknown ref must be rejected")
	}
}

func TestShouldOfferFastForward(t *testing.T) {
	cases := []struct {
		name  string
		state domain.DivergenceState
		want  bool
	}{
		{"behind is fast-forwardable", domain.DivergenceBehind, true},
		{"diverged is not", domain.DivergenceDiverged, false},
		{"ahead is not", domain.DivergenceAhead, false},
		{"up to date is not", domain.DivergenceUpToDate, false},
		{"unknown is not", domain.DivergenceUnknown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ShouldOfferFastForward(c.state); got != c.want {
				t.Errorf("ShouldOfferFastForward(%v) = %v, want %v", c.state, got, c.want)
			}
		})
	}
}

func TestSourceIsStartPoint(t *testing.T) {
	cases := []struct {
		name   string
		target domain.BranchTarget
		want   bool
	}{
		{"a new branch starts from the source", domain.BranchTarget{State: domain.BranchTargetNew}, true},
		{"a branch only on origin starts from origin", domain.BranchTarget{State: domain.BranchTargetNew, Upstream: "origin/feat"}, false},
		{"a reused branch carries its own history", domain.BranchTarget{State: domain.BranchTargetExisting}, false},
		{"a taken branch never starts from the source", domain.BranchTarget{State: domain.BranchTargetCheckedOut}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SourceIsStartPoint(c.target); got != c.want {
				t.Errorf("SourceIsStartPoint(%+v) = %v, want %v", c.target, got, c.want)
			}
		})
	}
}

func TestMergeBranchCandidates(t *testing.T) {
	got := MergeBranchCandidates(MergeBranchCandidatesParams{
		Local:  []string{"main", "feature"},
		Remote: []string{"origin/feature", "origin/hotfix", "origin/release"},
	})

	want := []domain.BranchCandidate{
		{Name: "feature", IsRemote: false},
		{Name: "main", IsRemote: false},
		{Name: "origin/hotfix", IsRemote: true},
		{Name: "origin/release", IsRemote: true},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d: expected %+v, got %+v", i, want[i], got[i])
		}
	}
}

func TestMergeBranchCandidatesDropsRemoteWithLocalCounterpart(t *testing.T) {
	got := MergeBranchCandidates(MergeBranchCandidatesParams{
		Local:  []string{"feature"},
		Remote: []string{"origin/feature"},
	})

	if len(got) != 1 {
		t.Fatalf("expected the remote with a local counterpart to be dropped, got %+v", got)
	}
	if got[0].Name != "feature" || got[0].IsRemote {
		t.Errorf("expected local feature to win, got %+v", got[0])
	}
}

func TestMergeBranchCandidatesEmptyRemote(t *testing.T) {
	got := MergeBranchCandidates(MergeBranchCandidatesParams{Local: []string{"b", "a"}})

	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("expected locals sorted with no remotes, got %+v", got)
	}
}

func TestMergeBranchCandidatesFillsDivergence(t *testing.T) {
	got := MergeBranchCandidates(MergeBranchCandidatesParams{
		Local:  []string{"main", "feature", "solo"},
		Remote: []string{"origin/main", "origin/feature", "origin/other"},
		Divergence: map[string]domain.AheadBehind{
			"main":    {Ahead: 0, Behind: 5},
			"feature": {Ahead: 2, Behind: 3},
		},
	})

	byName := make(map[string]domain.BranchCandidate, len(got))
	for _, c := range got {
		byName[c.Name] = c
	}

	if c := byName["main"]; c.State != domain.DivergenceBehind || c.Behind != 5 || c.Ahead != 0 {
		t.Errorf("main: expected behind/5/0, got %+v", c)
	}
	if c := byName["feature"]; c.State != domain.DivergenceDiverged || c.Ahead != 2 || c.Behind != 3 {
		t.Errorf("feature: expected diverged/2/3, got %+v", c)
	}
	// solo has a remote-only sibling ("origin/other") but no same-name origin
	// counterpart, so no divergence data and Unknown state.
	if c := byName["solo"]; c.State != domain.DivergenceUnknown || c.Ahead != 0 || c.Behind != 0 {
		t.Errorf("solo: expected unknown/0/0, got %+v", c)
	}
}
