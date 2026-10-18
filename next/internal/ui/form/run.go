package form

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

// Run asks the form of script, reading keys from in and drawing on out, then applies the request; when the plan turns stale as it applies, it starts the form again with the answers kept ([Wizard.Again]).
// A dry run with nothing to ask applies at once, without drawing the form. Run returns Apply's error, a failed read of the repository, or a [domain.KindCancelled] error when the user leaves.
func Run(ctx context.Context, script Script, in io.Reader, out *output.Stream) error {
	w := New(ctx, script)
	w.style(output.StylesOf(out))

	if script.DryRun {
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
		if script.DryRun || !script.Stale(err) {
			return err
		}
		w = w.Again()
	}
}

// Walk asks the steps of script on the terminal, reading keys from in and drawing on out, until the last one is answered: a read's picker, with no recap nor apply. It returns a failed read of the repository, or a [domain.KindCancelled] error when the user leaves.
func Walk(ctx context.Context, script Script, in io.Reader, out *output.Stream) error {
	w := New(ctx, script)
	w.style(output.StylesOf(out))
	return ask(ctx, w, in, out)
}

// ask runs wizard as a Bubble Tea program until it ends, and returns why it did not finish: the failed read that ended it, or an interruption.
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
