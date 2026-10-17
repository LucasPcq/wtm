package output

import (
	"cmp"
	"encoding/json"
	"io"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const envelopeVersion = 2

type EnvelopeParams struct {
	Outcome app.CreateOutcome
	Err     *wtm.Error
}

type envelope struct {
	V           int               `json:"v"`
	Command     string            `json:"command"`
	OK          bool              `json:"ok"`
	Op          string            `json:"op,omitempty"`
	Items       []envelopeItem    `json:"items,omitempty"`
	Plan        *create.Planned   `json:"plan,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	FollowUps   []envelopeFollow  `json:"follow_ups"`
	Warnings    []envelopeWarning `json:"warnings"`
	Error       *envelopeError    `json:"error,omitempty"`
}

type envelopeItem struct {
	Subject  string            `json:"subject"`
	Status   wtm.Status        `json:"status"`
	Reason   wtm.Reason        `json:"reason,omitempty"`
	Error    *envelopeError    `json:"error,omitempty"`
	Warnings []envelopeWarning `json:"warnings,omitempty"`
	Detail   any               `json:"detail"`
}

type envelopeWarning struct {
	Code    string     `json:"code"`
	Params  wtm.Params `json:"params,omitempty"`
	Message string     `json:"message"`
}

type envelopeFollow struct {
	Command string          `json:"command"`
	Request json.RawMessage `json:"request,omitempty"`
	Code    string          `json:"code"`
	Params  wtm.Params      `json:"params,omitempty"`
}

type envelopeError struct {
	Kind     wtm.Kind        `json:"kind"`
	Code     string          `json:"code"`
	Message  string          `json:"message"`
	Params   wtm.Params      `json:"params,omitempty"`
	FollowUp *envelopeFollow `json:"follow_up,omitempty"`
	Fields   []envelopeField `json:"fields,omitempty"`
}

type envelopeField struct {
	Path     string     `json:"path"`
	Code     string     `json:"code"`
	Params   wtm.Params `json:"params,omitempty"`
	Accepted []string   `json:"accepted,omitempty"`
	With     []string   `json:"with,omitempty"`
	Flag     string     `json:"flag"`
	Message  string     `json:"message"`
}

func Envelope(w io.Writer, params EnvelopeParams) error {
	outcome := params.Outcome
	failure := params.Err
	doc := envelope{V: envelopeVersion, Command: commandName, Op: outcome.Op, FollowUps: []envelopeFollow{}}
	switch {
	case outcome.Result != nil:
		failure = cmp.Or(failure, outcome.Result.Failure())
		for _, item := range outcome.Result.Items {
			doc.Items = append(doc.Items, itemOf(item))
		}
		doc.Warnings = warningsOf(outcome.Warnings)
	case failure == nil:
		doc.Plan, doc.Fingerprint, doc.Warnings = &outcome.Plan, outcome.Fingerprint, warningsOf(outcome.Warnings)
	}
	if doc.Warnings == nil {
		doc.Warnings = []envelopeWarning{}
	}
	doc.OK, doc.Error = failure == nil, errorOf(failure)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(doc)
}

func itemOf(item create.Item) envelopeItem {
	var detail any = struct{}{}
	if item.Detail != nil {
		detail = item.Detail
	}
	doc := envelopeItem{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Error: errorOf(item.Err), Detail: detail}
	if len(item.Warnings) > 0 {
		doc.Warnings = warningsOf(item.Warnings)
	}
	return doc
}

func warningsOf(warnings []wtm.Warning) []envelopeWarning {
	out := make([]envelopeWarning, 0, len(warnings))
	for _, warned := range warnings {
		out = append(out, envelopeWarning{Code: warned.Code, Params: warned.Params, Message: warned.Message})
	}
	return out
}

func errorOf(err *wtm.Error) *envelopeError {
	if err == nil {
		return nil
	}
	doc := &envelopeError{Kind: err.Kind, Code: err.Code, Message: errorText(err), Params: err.Params}
	if follow := err.FollowUp; follow != nil {
		doc.FollowUp = &envelopeFollow{Command: follow.Command, Request: follow.Request, Code: follow.Code, Params: follow.Params}
	}
	for _, field := range err.Fields {
		flag := Flag(field.Path)
		doc.Fields = append(doc.Fields, envelopeField{Path: field.Path, Code: field.Code, Params: field.Params, Accepted: field.Accepted, With: field.With, Flag: flag, Message: field.Message(flag)})
	}
	return doc
}
