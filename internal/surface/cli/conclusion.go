package cli

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

const (
	applyNote  = "apply this plan"
	replanNote = "the plan the facts give now"
)

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
