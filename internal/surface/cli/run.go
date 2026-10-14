package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

const (
	applyNote  = "apply this plan"
	replanNote = "the plan the facts give now"
)

// route is how a run goes, read once from the command line.
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

// run is the same for every command of the registry: the request from the
// flags, the call through dispatch, then the conclusion or the envelope.
func (b *binding) run(cmd *cobra.Command, args []string) error {
	way := b.route(cmd)
	request, invalid := b.request(cmd, args)
	if invalid != nil {
		return b.fail(cmd, failParams{Route: way, Err: invalid})
	}
	return b.execute(cmd, callParams{Route: way, Request: request})
}

// execute calls, concludes, then offers the follow-ups of an interactive run
// one by one: the failure of the run itself is the one that exits.
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

type concludeParams struct {
	Route   route
	Request json.RawMessage
	Reply   dispatch.Reply
	Err     kernel.Error
}

func (b *binding) conclude(cmd *cobra.Command, params concludeParams) error {
	switch reply := params.Reply.(type) {
	case dispatch.CommandReply:
		return b.concludeCommand(cmd, commandParams{concludeParams: params, Result: reply.Result})
	case dispatch.QueryReply:
		return b.concludeQuery(cmd, queryParams{Route: params.Route, View: reply.View})
	}
	return b.fail(cmd, failParams{Route: params.Route, Err: params.Err, Request: params.Request})
}

type commandParams struct {
	concludeParams
	Result dispatch.Result[json.RawMessage, json.RawMessage]
}

func (b *binding) concludeCommand(cmd *cobra.Command, params commandParams) error {
	failure := firstFailure(params)
	if !params.Route.Human {
		if err := render.WriteEnvelope(cmd.OutOrStdout(), b.envelope(envelopeParams{Command: params, Failure: failure})); err != nil {
			return err
		}
		return b.reported(failure)
	}
	outcome := params.Result.Outcome
	switch {
	case outcome != nil:
		next, err := b.followUps(outcome.FollowUps)
		if err != nil {
			return b.fail(cmd, failParams{Route: params.Route, Err: classified(err)})
		}
		render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			render.Outcome(w, render.OutcomeParams{Command: b.name, Items: outcome.Items, Warnings: outcome.Warnings, FollowUps: next, Detail: b.detail, Catalog: b.engine.catalog})
		})
	case params.Err != nil:
		return b.fail(cmd, failParams{Route: params.Route, Err: params.Err, Request: params.Request})
	default:
		apply, err := b.rerun(rerunParams{Request: params.Request, Flag: flagExpectFingerprint, Value: string(params.Result.Fingerprint)})
		if err != nil {
			return b.fail(cmd, failParams{Route: params.Route, Err: classified(err)})
		}
		render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			render.Recap(w, render.RecapParams{
				Command:     b.name,
				Plan:        params.Result.Plan.Detail,
				Warnings:    params.Result.Plan.Warnings,
				Fingerprint: string(params.Result.Fingerprint),
				Next:        []render.NextStepParams{{Command: apply, Note: applyNote}},
				Detail:      b.detail,
				Catalog:     b.engine.catalog,
			})
		})
	}
	return b.reported(failure)
}

func firstFailure(params commandParams) kernel.Error {
	if params.Err != nil {
		return params.Err
	}
	if params.Result.Outcome == nil {
		return nil
	}
	return params.Result.Outcome.Failure()
}

type envelopeParams struct {
	Command commandParams
	Failure kernel.Error
}

// envelope fills the keys the run reached: the items of an applied run, the
// plan and its fingerprint of a dry run, the error alone otherwise.
func (b *binding) envelope(params envelopeParams) render.EnvelopeParams {
	result := params.Command.Result
	reach := reachedBy(params.Command)
	return render.EnvelopeParams{
		Command:     b.name,
		OK:          params.Failure == nil,
		Op:          result.Op,
		Items:       reach.Items,
		Plan:        reach.Plan,
		Fingerprint: reach.Fingerprint,
		FollowUps:   reach.FollowUps,
		Warnings:    reach.Warnings,
		Err:         params.Failure,
		Wording:     b.wording(),
	}
}

type reached struct {
	Items       []kernel.Item[json.RawMessage]
	FollowUps   []kernel.FollowUp
	Warnings    []kernel.Warning
	Plan        json.RawMessage
	Fingerprint string
}

func reachedBy(params commandParams) reached {
	result := params.Result
	if result.Outcome != nil {
		return reached{Items: result.Outcome.Items, FollowUps: result.Outcome.FollowUps, Warnings: result.Outcome.Warnings}
	}
	if params.Err != nil {
		return reached{}
	}
	return reached{Plan: result.Plan.Detail, Fingerprint: string(result.Fingerprint), Warnings: result.Plan.Warnings}
}

type queryParams struct {
	Route route
	View  json.RawMessage
}

func (b *binding) concludeQuery(cmd *cobra.Command, params queryParams) error {
	if !params.Route.Human {
		return render.WriteEnvelope(cmd.OutOrStdout(), render.EnvelopeParams{Command: b.name, OK: true, View: params.View, Wording: b.wording()})
	}
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) { render.View(w, b.name, params.View) })
	return nil
}

type failParams struct {
	Route   route
	Err     kernel.Error
	Request json.RawMessage
}

// fail reports a run that applied nothing: the envelope on stdout, or the
// failure on stderr.
func (b *binding) fail(cmd *cobra.Command, params failParams) error {
	if !params.Route.Human {
		if err := render.WriteEnvelope(cmd.OutOrStdout(), render.EnvelopeParams{Command: b.name, Err: params.Err, Wording: b.wording()}); err != nil {
			return err
		}
		return b.reported(params.Err)
	}
	next, err := b.failureNext(params)
	if err != nil {
		return b.fail(cmd, failParams{Route: params.Route, Err: kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: errors.Join(err, params.Err)})})
	}
	render.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		render.Failure(w, render.FailureParams{Err: params.Err, Wording: b.wording(), Next: next})
	})
	return b.reported(params.Err)
}

// failureNext is what to run next: the follow-up the error carries, or a new
// preview when the facts changed under the one the run expected.
func (b *binding) failureNext(params failParams) ([]render.NextStepParams, error) {
	base := params.Err.Base()
	if base.Code == kernel.CodePlanStale {
		line, err := b.rerun(rerunParams{Request: params.Request, Flag: domain.FlagDryRun})
		return []render.NextStepParams{{Command: line, Note: replanNote}}, err
	}
	if base.FollowUp == nil {
		return nil, nil
	}
	return b.followUps([]kernel.FollowUp{*base.FollowUp})
}

func (b *binding) flagError(cmd *cobra.Command, err error) error {
	invalid := kernel.Invalid(nil)
	invalid.Cause = err
	return b.fail(cmd, failParams{Route: b.route(cmd), Err: invalid})
}

func (b *binding) reported(failure kernel.Error) error {
	if failure == nil {
		return nil
	}
	return &reported{failure: failure, line: b.wording().FailureLine(failure), exitCodes: b.exitCodes}
}

func (b *binding) wording() render.Wording {
	return render.Wording{Catalog: b.engine.catalog, Names: b.spell}
}
