package rules

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestDistinctNamesTrimsAndKeepsTheOrder(t *testing.T) {
	names, err := DistinctNames(DistinctNamesParams{Names: []string{" feat/a", "feat/b "}, Blank: "blank"})
	if err != nil || !reflect.DeepEqual(names, []string{"feat/a", "feat/b"}) {
		t.Errorf("names = %v, err = %v", names, err)
	}
}

func TestDistinctNamesRefusesABlankOrARepeat(t *testing.T) {
	for name, names := range map[string][]string{
		"blank":  {"feat/a", "  "},
		"repeat": {"feat/a", " feat/a"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DistinctNames(DistinctNamesParams{Names: names, Blank: "blank"}); !errors.Is(err, domain.ErrUsage) {
				t.Errorf("err = %v, want ErrUsage", err)
			}
		})
	}
}

func TestBatchFailureOfKeepsTheExitCodeAndFlagsAPrivilegedRemoval(t *testing.T) {
	removal := BatchFailureOf(BatchFailureOfParams{Branch: "feat/a", Path: "/a", Err: fmt.Errorf("%w: busy", domain.ErrWorktreeRemoveFailed)})
	if !removal.Privileged || removal.Branch != "feat/a" || removal.Path != "/a" {
		t.Errorf("failure = %+v, want a privileged removal of feat/a", removal)
	}
	exists := BatchFailureOf(BatchFailureOfParams{Branch: "feat/b", Err: domain.ErrWorktreeExists})
	if exists.ExitCode != domain.ExitCodeWorktreeExists || exists.Privileged {
		t.Errorf("failure = %+v, want the worktree-exists exit code", exists)
	}
}
