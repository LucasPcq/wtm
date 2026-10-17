package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestOccupiedPathProblem(t *testing.T) {
	cases := []struct {
		name     string
		occupant string
		want     error
	}{
		{"another branch's worktree", "feat-x", domain.ErrWorktreeNameTaken},
		{"no worktree registered", "", domain.ErrWorktreePathExists},
		{"the branch's own worktree", "feat/x", domain.ErrWorktreePathExists},
	}
	for _, tc := range cases {
		err := OccupiedPathProblem(OccupiedPathParams{Branch: "feat/x", Path: "/trees/feat-x", Occupant: tc.occupant})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
