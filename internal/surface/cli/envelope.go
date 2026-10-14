package cli

import (
	"encoding/json"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

type envelopeParams struct {
	Command commandParams
	Failure kernel.Error
}

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
