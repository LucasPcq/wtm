package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"

	"github.com/stretchr/testify/assert"
)

func TestThePickerAsksAProfileOrAJobThenWhatToDoWithIt(t *testing.T) {
	d := picking(t, world{})
	d.looksLike("1_entry")

	d.Keys("down", "enter")
	d.looksLike("2_action")

	d.Keys("down", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, runlist.CodeDown, d.p.action.Code)
	assert.Equal(t, domain.CommandRunDown, d.p.action.Command)
}

func TestAJobPickedOffersToStartStopAndReadIt(t *testing.T) {
	d := picking(t, world{})

	d.Keys("down", "down", "enter")
	d.looksLike("job_action")

	d.Keys("down", "down", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}, {Path: "job", Values: []string{"web"}}}, d.p.action.Answers)
	assert.Equal(t, domain.CommandRunLogs, d.p.action.Command)
}

func TestLeavingThePickerCancels(t *testing.T) {
	d := picking(t, world{})

	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}
