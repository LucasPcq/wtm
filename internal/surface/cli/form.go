package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/surface/tui/form"
)

// interactiveForm is the production Form: the terminal's wizard, its recap
// drawn by the command's own detail.
func interactiveForm(ctx context.Context, params FormParams) (dispatch.Reply, error) {
	return form.Run(ctx, form.Params{
		Entry:    params.Entry,
		Request:  params.Request,
		Options:  params.Options,
		Names:    params.Names,
		Plan:     params.Detail.Plan,
		Progress: params.Progress,
		Catalog:  params.Catalog,
	})
}

func confirmFollowUp(params ConfirmParams) (bool, error) {
	return form.FollowUp(form.FollowUpParams{Command: params.Command, Note: params.Note})
}

type followParams struct {
	Route route
	Reply dispatch.Reply
}

// follow offers each follow-up of an interactive run that applied: a yes runs
// it through the form again, its request filled in, and concludes it in turn.
// A follow-up this command line does not serve stays the line it printed.
func (b *binding) follow(cmd *cobra.Command, params followParams) error {
	reply, applied := params.Reply.(dispatch.CommandReply)
	if !params.Route.Interactive || b.engine.form == nil || b.engine.confirm == nil || !applied || reply.Outcome == nil {
		return nil
	}
	for _, followUp := range reply.Outcome.FollowUps {
		target, served := b.engine.bindings[followUp.Command]
		if !served {
			continue
		}
		yes, err := b.engine.confirm(ConfirmParams{
			Command: b.engine.commandLine(lineParams{Target: followUp.Command, Request: followUp.Request}),
			Note:    b.engine.catalog.Message(followUp.Code, followUp.Params),
		})
		if err != nil {
			return err
		}
		if !yes {
			continue
		}
		if err := target.execute(cmd, callParams{Route: freshRoute(params.Route), Request: followUp.Request}); err != nil {
			return err
		}
	}
	return nil
}

// freshRoute is the route of a follow-up: as interactive as the run that
// proposed it, without the options only that run was given.
func freshRoute(from route) route {
	return route{Human: from.Human, Interactive: from.Interactive, Options: dispatch.Options{Surface: from.Options.Surface, CorrelationID: from.Options.CorrelationID}}
}
