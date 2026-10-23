package run

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestWhatAReadCouldNotAskIsAWarningUnderItsCommandsCodes(t *testing.T) {
	asked := Asked{JobsErr: ErrUnknown, PRsErr: errors.New("gh is not logged in")}

	warnings := asked.Warnings("list")

	assert.Equal(t, []domain.Warning{
		{Code: "list.jobs_unknown", Message: ErrUnknown.Error()},
		{Code: "list.prs_unavailable", Message: "gh is not logged in"},
	}, warnings)
}

func TestAnInterruptedReadWarnsOfNothingItCouldNotAsk(t *testing.T) {
	asked := Asked{JobsErr: ErrUnknown, PRsErr: errors.New("context canceled"), Interrupted: true}

	assert.Empty(t, asked.Warnings("list"))
}

func TestTheServicesOfAWorktreeAreUnknownWhenTheDaemonCannotTell(t *testing.T) {
	jobs := []Job{{Name: "web", WorkDir: "/a", Up: true}}

	assert.Equal(t, []string{"web"}, Asked{Jobs: jobs}.Services("/a"))
	assert.Nil(t, Asked{Jobs: jobs, JobsErr: ErrUnknown}.Services("/a"))
}
