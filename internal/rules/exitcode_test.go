package rules_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, domain.ExitCodeOK},
		{"generic", errors.New("boom"), domain.ExitCodeError},
		{"worktree path exists", domain.ErrWorktreePathExists, domain.ExitCodeWorktreeExists},
		{"worktree exists", domain.ErrWorktreeExists, domain.ExitCodeWorktreeExists},
		{"branch not found", domain.ErrBranchNotFound, domain.ExitCodeBranchNotFound},
		{"config not found", domain.ErrConfigNotFound, domain.ExitCodeConfigNotFound},
		{"job not found", domain.ErrJobNotFound, domain.ExitCodeNotDeclared},
		{"profile not found", fmt.Errorf(domain.RunProfileNotFoundFmt, domain.ErrProfileNotFound, "dev"), domain.ExitCodeNotDeclared},
		{"usage", fmt.Errorf("unknown flag: --bogus: %w", domain.ErrUsage), domain.ExitCodeUsage},
		{"wrapped", fmt.Errorf("context: %w", domain.ErrBranchNotFound), domain.ExitCodeBranchNotFound},
		{"env drift", fmt.Errorf("%w (%w)", domain.ErrEnvDrift, domain.ErrAborted), domain.ExitCodeEnvDrift},
		{"events schema newer", fmt.Errorf("event v2: %w", domain.ErrEventsSchemaNewer), domain.ExitCodeEventsSchemaNewer},
		{"not a git repository", fmt.Errorf(domain.NotGitRepoFmt, "/tmp/x", domain.ErrNotGitRepo), domain.ExitCodeNotGitRepo},
		{"context cancelled", fmt.Errorf("git fetch: %w", context.Canceled), domain.ExitCodeCancelled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestExitCodeUpgradeErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"from source", domain.ErrUpgradeFromSource, domain.ExitCodeUpgradeUnsupported},
		{"not writable", domain.ErrUpgradeNotWritable, domain.ExitCodeUpgradeUnsupported},
		{"wrapped not writable", fmt.Errorf("upgrade: %w", domain.ErrUpgradeNotWritable), domain.ExitCodeUpgradeUnsupported},
		{"checksum mismatch stays generic", domain.ErrChecksumMismatch, domain.ExitCodeError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ExitCode(tc.err); got != tc.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestInterrupted(t *testing.T) {
	failure := errors.New("git fetch: signal: interrupt")
	cases := []struct {
		name      string
		params    rules.InterruptedParams
		cancelled bool
		same      bool
	}{
		{"no error", rules.InterruptedParams{Signalled: true}, false, true},
		{"no signal", rules.InterruptedParams{Err: failure}, false, true},
		{"signalled failure", rules.InterruptedParams{Err: failure, Signalled: true}, true, false},
		{"already cancelled", rules.InterruptedParams{Err: domain.ErrCancelled, Signalled: true}, true, true},
		{"reported then signalled", rules.InterruptedParams{Err: domain.ErrAborted, Signalled: true}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rules.Interrupted(tc.params)
			if errors.Is(got, domain.ErrCancelled) != tc.cancelled {
				t.Fatalf("Interrupted(%+v) = %v, cancelled want %v", tc.params, got, tc.cancelled)
			}
			if tc.same && got != tc.params.Err {
				t.Fatalf("Interrupted(%+v) = %v, want the error untouched", tc.params, got)
			}
			if tc.params.Err != nil && !errors.Is(got, tc.params.Err) {
				t.Fatalf("Interrupted(%+v) = %v, lost the cause", tc.params, got)
			}
		})
	}
}

// A runner that returns ErrAborted after the user escaped its picker never
// reaches the hook that maps the mark to 19: the root reads the mark instead.
func TestARunTheUserBackedOutOfExitsCancelled(t *testing.T) {
	if got := rules.ExitCode(rules.BackedOut(rules.BackedOutParams{Err: domain.ErrAborted, Cancelled: true})); got != domain.ExitCodeCancelled {
		t.Errorf("exit code = %d, want %d", got, domain.ExitCodeCancelled)
	}
	if got := rules.ExitCode(rules.BackedOut(rules.BackedOutParams{Err: domain.ErrAborted})); got != domain.ExitCodeError {
		t.Errorf("an unmarked failure exits %d, want %d", got, domain.ExitCodeError)
	}
	if rules.BackedOut(rules.BackedOutParams{Cancelled: true}) != nil {
		t.Error("a clean run became an error")
	}
}
