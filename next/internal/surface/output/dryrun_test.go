package output

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

func TestADryRunShowsThePlanItsWarningsItsFingerprintAndHowToApplyIt(t *testing.T) {
	outcome := twoCreated(inTrees(t))
	outcome.Result = nil
	outcome.Plan.FastForward = []string{"main"}
	outcome.Plan.Remember = []string{"env_from"}
	outcome.Warnings = []wtm.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}

	golden(t, "dryrun", rendered(func(w *bytes.Buffer) {
		DryRun(w, DryRunParams{Outcome: outcome, Apply: "wtm create feat/a feat/b --env-from example --remember env_from --expect-fingerprint 0123456789abcdef01234567"})
	}))
}
