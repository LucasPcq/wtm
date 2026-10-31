package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// B4: legacy showed a fork whose branch a worktree holds as checked out.
func TestThePickerListsTheOpenPullRequestsWithWhatSetsEachApart(t *testing.T) {
	d := wizard(t, checkout.Request{})

	formtest.Golden(t, "wizard_1_pull_request", d.Screen())
}

func TestAPullRequestPickedIsAskedItsParentThenConfirmed(t *testing.T) {
	d := wizard(t, checkout.Request{})

	d.Keys("enter")
	formtest.Golden(t, "wizard_2_parent", d.Screen())

	d.Keys("enter", "enter")
	formtest.Golden(t, "wizard_3_recap", d.Screen())

	d.Keys("enter")
	require.Equal(t, form.Finished, d.Wizard.Ending())
}

func TestAPullRequestThatCannotBeCheckedOutIsRefusedAtTheRecap(t *testing.T) {
	d := wizard(t, checkout.Request{})

	d.Keys("down", "down", "down", "enter", "enter", "enter")

	assert.Contains(t, d.Screen(), "branch feat/held of pull request #4 is checked out at /elsewhere/feat-held")
	assert.Contains(t, d.Screen(), "→ wtm go feat/held   jump in")
}
