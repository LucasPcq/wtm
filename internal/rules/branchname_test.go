package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBranchNameProblemFollowsGit(t *testing.T) {
	valid := []string{"main", "feat/login", "fix/a-b_c.d", "v1.2", "user@host", "a/b/c"}
	for _, name := range valid {
		if err := BranchNameProblem(name); err != nil {
			t.Errorf("%q refused: %v", name, err)
		}
	}
	invalid := []string{"bad..name", "-x", "/a", "a/", "a//b", "a.", "a@{1}", "@", "HEAD", "a b", "a~1", "a^", "a:b", "a?", "a*", "a[b", `a\b`, ".hidden", "a/.b", "x.lock", "a/x.lock/b", "tab\tname"}
	for _, name := range invalid {
		err := BranchNameProblem(name)
		if err == nil {
			t.Errorf("%q accepted, git refuses it", name)
			continue
		}
		if !errors.Is(err, domain.ErrUsage) {
			t.Errorf("%q: %v is not a usage error", name, err)
		}
	}
}
