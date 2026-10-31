package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfCheckoutIsTheEnginesWordForWord(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "checkout_help", got.Stdout)
}

func TestCheckoutConcludesWithItsWorktreeAndTheStepIntoIt(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "1", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "outcome_resumed", got.Stdout)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "1", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestTheEnvelopeCarriesThePullRequestAndHowItsBranchIsCheckedOut(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "1", "--from", "origin/feat/draft", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
	formtest.Golden(t, "envelope_success", got.Stdout)
	envelope := decoded(t, got.Stdout)
	assert.Equal(t, 1, envelope.Plan.PullRequest.Number)
	assert.Equal(t, "resume", envelope.Plan.Action)
	assert.Equal(t, "origin/feat/draft", envelope.Plan.From)
}

// B6, B8: legacy dumped gh's command line, and printed no JSON on a refusal.
func TestEachPullRequestThatCannotBeCheckedOutIsAnInvalidRequestNamingWhy(t *testing.T) {
	tests := []struct {
		name string
		args []string
		says string
	}{
		{"unknown", []string{"99"}, "<number>: no pull request #99"},
		{"fork", []string{"3"}, "pull request #3 is from a fork: wtm checks out only this repository's branches, `gh pr checkout 3` checks it out"},
		{"held", []string{"4"}, "branch feat/held of pull request #4 is checked out at /elsewhere/feat-held\n  → wtm go feat/held   jump in"},
		{"self_parent", []string{"1", "--from", "feat/remote"}, "feat/remote cannot be its own parent: pass another value to --from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCheckout(t, newWorld(seed{}), append(tt.args, "--yes")...)

			assert.Equal(t, uicli.ExitUsage, got.Code)
			assert.Contains(t, got.Stderr, tt.says)
		})
	}
}

func TestARefusalIsTheEnvelopeWithItsField(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "4", "--output", "json")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "envelope_invalid", got.Stdout)
}

// B7: legacy exited 1, naming no argument.
func TestNoPullRequestUnattendedIsRequiredByItsArgument(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "<number> is required")
}

func TestAnArgumentThatIsNoPullRequestNumberIsAUsageError(t *testing.T) {
	for _, arg := range []string{"abc", "0"} {
		got := runCheckout(t, newWorld(seed{}), arg)

		assert.Equal(t, uicli.ExitUsage, got.Code, arg)
		assert.Contains(t, got.Stderr, "<number> must be a pull request number, not "+arg)
	}
}

// B9: legacy took --review alone, silently.
func TestMineAndReviewTogetherAreRefused(t *testing.T) {
	got := runCheckout(t, newWorld(seed{}), "1", "--mine", "--review", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "--review cannot be given with the filter of your own pull requests")
}

func TestGitHubThatCannotBeAskedIsAPrecondition(t *testing.T) {
	got := runCheckout(t, newWorld(seed{GitHub: worktree.ErrPullRequestsUnavailable}), "1", "--yes")

	assert.Equal(t, uicli.ExitPrecondition, got.Code)
	assert.Contains(t, got.Stderr, "pull requests unavailable")
}

func TestAHookThatFailsEndsOnItsWorktreeAndExitsOne(t *testing.T) {
	got := runCheckout(t, newWorld(seed{OnCreate: []project.Hook{{Cmd: "false"}}}), "1", "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Contains(t, got.Stdout, "→ wtm go feat/remote")
}
