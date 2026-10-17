package form

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

type RunParams struct {
	Params
	In  io.Reader
	Out io.Writer
}

func Run(ctx context.Context, params RunParams) error {
	w := New(ctx, params.Params)
	script := params.Script
	if params.DryRun {
		if err := script.Observe(ctx); err != nil {
			return err
		}
		if !w.asks() {
			return script.Apply(ctx)
		}
	}
	for {
		if err := ask(ctx, asked{Wizard: w, In: params.In, Out: params.Out}); err != nil {
			return err
		}
		err := script.Apply(ctx)
		if params.DryRun || !script.Stale(err) {
			return err
		}
		w = w.Again()
	}
}

type asked struct {
	Wizard *Wizard
	In     io.Reader
	Out    io.Writer
}

func ask(ctx context.Context, params asked) error {
	final, err := tea.NewProgram(params.Wizard, tea.WithContext(ctx), tea.WithInput(params.In), tea.WithOutput(params.Out), tea.WithoutSignalHandler()).Run()
	if err != nil {
		return interrupted(err)
	}
	w, ok := final.(*Wizard)
	switch {
	case !ok:
		return interrupted(nil)
	case w.err != nil:
		return w.err
	case w.Ending() == Cancelled:
		return interrupted(nil)
	}
	return nil
}

func interrupted(cause error) *domain.Error {
	return &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted", Cause: cause}
}
