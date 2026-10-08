package start

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/concurrency"
)

// A job started alone gets the port check `run up` gives it, on the same budget.
func TestTheStartedJobIsProbedOnRunTomlsBudget(t *testing.T) {
	f := &startFlow{runCtx: t.Context(), request: Request{Config: domain.RunConfig{PortProbeTimeout: 3}}}

	params := f.seamParams("/wt/here")
	if params.ProbeBudget != 3*time.Second || params.NoProbe {
		t.Errorf("probe = (%v, noProbe %v), want run.toml's 3s budget", params.ProbeBudget, params.NoProbe)
	}
}

func TestNoProbeSkipsThePortCheck(t *testing.T) {
	f := &startFlow{runCtx: t.Context(), request: Request{NoProbe: true}}

	if !f.seamParams("/wt/here").NoProbe {
		t.Error("--no-probe did not reach the seam")
	}
}

func TestStartAsksAboutTheOtherWorktrees(t *testing.T) {
	f := &startFlow{runCtx: t.Context(), request: Request{Cwd: "/wt/here"}}
	f.concurrency = f.question()

	for _, step := range f.session().Steps {
		if step.Key == concurrency.Key {
			return
		}
	}
	t.Error("the session has no concurrency step")
}
