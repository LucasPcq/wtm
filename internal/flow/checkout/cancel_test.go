package checkout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

type interruptingRecorder struct {
	*recorder
	cancel context.CancelFunc
}

func (r interruptingRecorder) Stage(ctx context.Context, params flow.StageParams) error {
	if params.Message == fmt.Sprintf(domain.CreateLoadingFmt, "feat/thing") {
		r.cancel()
	}
	return r.recorder.Stage(ctx, params)
}

// Ctrl-C while the PR's worktree is being created lets it be created whole and
// runs nothing after it, its hooks included.
func TestAnInterruptedCheckoutKeepsTheWorktreeWhole(t *testing.T) {
	ctx := testContext(t)
	witness := t.TempDir() + "/hooked"
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "touch " + witness}}
	runCtx, cancel := context.WithCancel(t.Context())
	presenter := interruptingRecorder{recorder: newRecorder(), cancel: cancel}

	_, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Number: 42},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyRecap: confirmCheckout}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrCancelled) || !strings.Contains(err.Error(), "not set up") {
		t.Fatalf("err = %v, want the cancellation naming the worktree left unset", err)
	}
	if _, statErr := os.Stat(witness); !os.IsNotExist(statErr) {
		t.Error("the on_create hook ran after the interrupt")
	}
}

type cancelOnUpdate struct {
	*flowtest.Recorder
	cancel context.CancelFunc
}

func (p cancelOnUpdate) Publish(ctx context.Context, event domain.Event) {
	if event.Type == domain.EventWorktreeUpdated {
		p.cancel()
	}
	p.Recorder.Publish(ctx, event)
}

// The ports settle on the cancellable context, and with no on_create hook
// nothing came after them to notice an interrupt: the checkout read as a success.
func TestAnInterruptWhileThePortsSettleNamesTheWorktreeLeftBehind(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	if err := config.WriteRun(config.WriteRunParams{
		StateDir: ctx.StateDir,
		Force:    true,
		Config: domain.RunConfig{
			Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 3000}}},
			EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	presenter := newRecorder()
	runCtx, cancel := context.WithCancel(t.Context())
	ctx.Publisher = cancelOnUpdate{Recorder: presenter.Recorder, cancel: cancel}

	_, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Number: 42},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCheckout}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrLeftBehind) || !strings.Contains(err.Error(), "not set up") {
		t.Fatalf("err = %v, want the worktree named as created but not set up", err)
	}
	if presenter.checkedOut != nil {
		t.Errorf("checked out = %+v, want no success reported", presenter.checkedOut)
	}
	provisioned := presenter.Published[len(presenter.Published)-1]
	if provisioned.Type != domain.EventWorktreeProvisioned || provisioned.OK == nil || *provisioned.OK {
		t.Errorf("last event = %+v, want worktree.provisioned with ok false", provisioned)
	}
	for _, status := range presenter.Statuses {
		if strings.Contains(status.Text, "ports not settled") {
			t.Errorf("warned %q: the error already says the ports were not set up", status.Text)
		}
	}
}
