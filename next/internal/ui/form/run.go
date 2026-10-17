package form

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, params Params, in io.Reader, out *output.Stream) error {
	w := New(ctx, params)
	w.style(output.StylesOf(out))
	script := params.Script
	if params.DryRun {
		apply, err := script.Observe(ctx)
		apply()
		if err != nil {
			return err
		}
		if !w.asks() {
			return script.Apply(ctx)
		}
	}
	for {
		if err := ask(ctx, w, in, out); err != nil {
			return err
		}
		err := script.Apply(ctx)
		if params.DryRun || !script.Stale(err) {
			return err
		}
		w = w.Again()
	}
}

func ask(ctx context.Context, wizard *Wizard, in io.Reader, out *output.Stream) error {
	final, err := tea.NewProgram(wizard, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out.Writer), tea.WithoutSignalHandler()).Run()
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
