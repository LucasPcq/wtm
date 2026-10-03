package rules_test

import (
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
