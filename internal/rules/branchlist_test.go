package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBranchEntryProblem(t *testing.T) {
	cases := []struct {
		name     string
		params   BranchEntryParams
		wantErr  bool
		wantTake bool
	}{
		{name: "first entry", params: BranchEntryParams{Entry: "feat/a"}},
		{name: "distinct entries", params: BranchEntryParams{Entry: "feat/b", Entries: []string{"feat/a"}, DerivedNames: true}},
		{name: "listed twice", params: BranchEntryParams{Entry: "feat/a", Entries: []string{"feat/a"}}, wantErr: true},
		{name: "derived clash with jobs", params: BranchEntryParams{Entry: "feat.x", Entries: []string{"feat/x"}, DerivedNames: true}, wantErr: true, wantTake: true},
		{name: "derived clash without jobs", params: BranchEntryParams{Entry: "feat.x", Entries: []string{"feat/x"}}},
		{name: "host-label clash", params: BranchEntryParams{Entry: "feat/a_b", Entries: []string{"feat/a-b"}, DerivedNames: true}, wantErr: true, wantTake: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := BranchEntryProblem(tc.params)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantTake && !errors.Is(err, domain.ErrWorktreeNameTaken) {
				t.Errorf("err = %v, want it to wrap ErrWorktreeNameTaken (exit 10)", err)
			}
		})
	}
}
