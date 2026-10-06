package flowui

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	listflow "github.com/LucasPcq/wtm/internal/flow/run/list"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// capturing hands a flow's session to the test instead of asking it.
type capturing struct {
	flow.Unattended
	session flow.Session
}

func (c *capturing) Interactive() bool { return true }

func (c *capturing) Ask(session flow.Session) (flow.Answers, error) {
	c.session = session
	return flow.Answers{}, domain.ErrUserAborted
}

// LUC-265: the action step was built before any entry was picked and answered
// that with an abort, so `wtm run list` closed before drawing its picker.
func TestRunListOpensOnItsPicker(t *testing.T) {
	prompter := &capturing{}
	if _, err := listflow.Run(listflow.Params{
		Request: listflow.Request{Config: domain.RunConfig{
			Jobs:     []domain.JobConfig{{Name: "api", Kind: domain.JobKindService}},
			Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}}},
		}},
		Prompter:  prompter,
		Presenter: &flowtest.Recorder{},
	}); err != nil {
		t.Fatalf("list: %v", err)
	}

	plan, err := build(prompter.session)

	if err != nil {
		t.Fatalf("the wizard could not open: %v", err)
	}
	want := []string{domain.RunListEntryStepName, domain.RunCRUDActionStepName}
	if got := names(plan.steps); !slices.Equal(got, want) {
		t.Errorf("steps = %v, want %v", got, want)
	}
}
