package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"

	"github.com/stretchr/testify/assert"
)

func TestThePickerAsksAWorktreeThenWhatToDoWithIt(t *testing.T) {
	d := picking(t, world{}, list.Request{})
	d.looksLike("1_worktree")

	d.Keys("down", "down", "enter")
	d.looksLike("2_action")

	d.Keys("down", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []domain.FollowUp{{Argv: []string{"wtm", "open_pr", "feat/b"}, Code: "list.open_pr", Message: "Open the pull request"}}, d.p.chosen().FollowUps)
}

func TestAMissingWorktreeOffersOnlyToCleanIt(t *testing.T) {
	d := picking(t, world{}, list.Request{})

	d.Keys("down", "down", "down", "down", "enter")

	assert.Contains(t, d.Screen(), "What to do with feat/gone")
	assert.Contains(t, d.Screen(), "Clean (delete the worktree)")
	assert.NotContains(t, d.Screen(), "Go to the worktree")
}

func TestLeavingThePickerCancels(t *testing.T) {
	d := picking(t, world{}, list.Request{})

	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}
