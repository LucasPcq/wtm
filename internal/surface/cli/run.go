package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

type route struct {
	Human       bool
	Interactive bool
	Options     dispatch.Options
}

func (b *binding) route(cmd *cobra.Command) route {
	flags := cmd.Flags()
	format, _ := flags.GetString(domain.FlagOutput)
	yes, _ := flags.GetBool(domain.FlagYes)
	dryRun, _ := flags.GetBool(domain.FlagDryRun)
	expected, _ := flags.GetString(flagExpectFingerprint)
	ask, _ := flags.GetBool(domain.FlagAsk)
	return route{
		Human:       rules.IsHumanFormat(format),
		Interactive: shared.Interactive(shared.UnattendedParams{TTY: shared.StdinIsTerminal(), Format: format, Yes: yes}),
		Options: dispatch.Options{
			DryRun:            dryRun,
			ExpectFingerprint: dispatch.Fingerprint(expected),
			IgnoreRemembered:  ask,
			Surface:           dispatch.SurfaceCLI,
			CorrelationID:     shared.CorrelationID(cmd),
		},
	}
}

func (b *binding) run(cmd *cobra.Command, args []string) error {
	way := b.route(cmd)
	request, invalid := b.request(cmd, args)
	if invalid != nil {
		return b.fail(cmd, failParams{Route: way, Err: invalid})
	}
	return b.execute(cmd, callParams{Route: way, Request: request})
}

func (b *binding) execute(cmd *cobra.Command, params callParams) error {
	reply, err := b.call(cmd, params)
	concluded := b.conclude(cmd, concludeParams{Route: params.Route, Request: params.Request, Reply: reply, Err: err})
	followed := b.follow(cmd, followParams{Route: params.Route, Reply: reply})
	if concluded != nil {
		return concluded
	}
	return followed
}

type callParams struct {
	Route   route
	Request json.RawMessage
}

func (b *binding) call(cmd *cobra.Command, params callParams) (dispatch.Reply, kernel.Error) {
	ctx := cmd.Context()
	dispatcher, err := b.engine.dispatcher(ctx)
	if err != nil {
		return nil, classified(err)
	}
	entry, found := b.engine.registry(dispatcher).Lookup(b.name)
	if !found {
		return nil, classified(fmt.Errorf(unknownEntryFmt, b.name))
	}
	view := render.NewProgressView(ctx, render.ProgressViewParams{W: cmd.ErrOrStderr(), Human: params.Route.Human, Catalog: b.engine.catalog, Live: render.IsTerminal(cmd.ErrOrStderr())})
	defer view.Close()
	fillable, isCommand := entry.Fillable()
	if params.Route.Interactive && b.engine.form != nil && isCommand {
		reply, err := b.engine.form(ctx, FormParams{Entry: fillable, Request: params.Request, Options: params.Route.Options, Names: b.spell, Detail: b.detail, Progress: view, Catalog: b.engine.catalog})
		return reply, classified(err)
	}
	reply, err := entry.Call(ctx, dispatch.Call{Request: params.Request, Options: params.Route.Options, Progress: view})
	return reply, classified(err)
}

func classified(err error) kernel.Error {
	if err == nil {
		return nil
	}
	return kernel.Classify(kernel.ClassifyParams{Err: err, Code: kernel.CodeInternal})
}
