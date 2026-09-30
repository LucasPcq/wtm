package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
)

// dispatchParams is an action `run list`'s picker chose, run in this process. It
// used to re-exec the binary for it (os.Executable + exec), which is what having
// no flow layer forced it into: the action lived in another command's runner and
// there was no other way to reach it.
type dispatchParams struct {
	Cmd *cobra.Command
	// WorkDir is the worktree the action acts on, loaded rather than assumed: the
	// config comes from it and not from the current directory.
	WorkDir string
	Job     string
	Profile string
	Format  string
}

// Every dispatch installs a prompter that answers nothing: the picker already
// chose, so re-asking would let the reader undo the action they just picked. A
// start still puts its safety question (foreign data) to the reader, who is at
// a terminal — refusing it for want of one would be false.
//
// target is the configuration of the repository the picked entry belongs to.
// The guard is not re-run: the listing this came from already passed it.
func (p dispatchParams) target() (runctx.Context, error) {
	return runctx.Open(runctx.OpenParams{Cmd: p.Cmd, Dir: p.WorkDir, SkipGuard: true})
}

func (p dispatchParams) dispatchStop() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	_, err = stopflow.Run(stopflow.Params{
		Context:   t.FlowContext(),
		Request:   stopflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.Run},
		Prompter:  t.Prompter(false),
		Presenter: stopPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	return err
}

func (p dispatchParams) dispatchStart() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := startflow.Run(startflow.Params{
		Context:   t.FlowContext(),
		Request:   startflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.Run},
		Prompter:  confirming(t),
		Presenter: startPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}

func (p dispatchParams) dispatchLogs() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	_, err = logsflow.Run(logsflow.Params{
		Context:   t.FlowContext(),
		Request:   logsflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.Run},
		Prompter:  t.Prompter(false),
		Presenter: logsPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	return err
}

func (p dispatchParams) dispatchUp() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := upflow.Run(upflow.Params{
		Context: t.FlowContext(),
		Request: upflow.Request{Cwd: p.WorkDir, Profile: p.Profile, Config: t.Run},
		// The picker asked its question already; the concurrency one it did not,
		// so it resolves to leaving the other worktrees alone.
		Prompter:  confirming(t),
		Presenter: upPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	return concluded(outcome)
}

func (p dispatchParams) dispatchDown(all bool) error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := downflow.Run(downflow.Params{
		Context:   t.FlowContext(),
		Request:   downflow.Request{Cwd: p.WorkDir, Profile: p.Profile, All: all, Config: t.Run},
		Prompter:  t.Prompter(false),
		Presenter: downPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	if outcome.Failed() {
		return domain.ErrAborted
	}
	return nil
}

// confirmingPrompter answers every step unattended and asks every confirmation.
type confirmingPrompter struct {
	answers  flow.Prompter
	confirms flow.Prompter
}

func confirming(t runctx.Context) flow.Prompter {
	return confirmingPrompter{answers: t.Prompter(false), confirms: t.Prompter(true)}
}

func (p confirmingPrompter) Ask(session flow.Session) (flow.Answers, error) {
	return p.answers.Ask(session)
}

func (p confirmingPrompter) Confirm(params flow.ConfirmParams) (bool, error) {
	return p.confirms.Confirm(params)
}

func (p confirmingPrompter) Interactive() bool { return true }
