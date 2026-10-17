package output

import (
	"encoding/json"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const envelopeVersion = 2

type EnvelopeParams struct {
	Command     string
	Op          string
	Items       []Item
	Plan        any
	Fingerprint string
	Warnings    []domain.Warning
	Err         *domain.Error
	Flags       Flags
}

type Item struct {
	Subject  string
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
	Detail   any
}

type envelope struct {
	V           int              `json:"v"`
	Command     string           `json:"command"`
	OK          bool             `json:"ok"`
	Op          string           `json:"op,omitempty"`
	Items       []envelopeItem   `json:"items,omitempty"`
	Plan        any              `json:"plan,omitempty"`
	Fingerprint string           `json:"fingerprint,omitempty"`
	FollowUps   []envelopeFollow `json:"follow_ups"`
	Warnings    []domain.Warning `json:"warnings"`
	Error       *envelopeError   `json:"error,omitempty"`
}

type envelopeItem struct {
	Subject  string           `json:"subject"`
	Status   domain.Status    `json:"status"`
	Reason   domain.Reason    `json:"reason,omitempty"`
	Error    *envelopeError   `json:"error,omitempty"`
	Warnings []domain.Warning `json:"warnings,omitempty"`
	Detail   any              `json:"detail"`
}

type envelopeFollow struct {
	Command string          `json:"command"`
	Request json.RawMessage `json:"request,omitempty"`
	Code    string          `json:"code"`
	Params  domain.Params   `json:"params,omitempty"`
}

type envelopeError struct {
	Kind     domain.Kind     `json:"kind"`
	Code     string          `json:"code"`
	Message  string          `json:"message"`
	Params   domain.Params   `json:"params,omitempty"`
	FollowUp *envelopeFollow `json:"follow_up,omitempty"`
	Fields   []envelopeField `json:"fields,omitempty"`
}

type envelopeField struct {
	Path     string        `json:"path"`
	Code     string        `json:"code"`
	Params   domain.Params `json:"params,omitempty"`
	Accepted []string      `json:"accepted,omitempty"`
	With     []string      `json:"with,omitempty"`
	Flag     string        `json:"flag"`
	Message  string        `json:"message"`
}

func Envelope(w io.Writer, params EnvelopeParams) error {
	doc := envelope{V: envelopeVersion, Command: params.Command, OK: params.Err == nil, Op: params.Op, Plan: params.Plan, Fingerprint: params.Fingerprint, FollowUps: []envelopeFollow{}, Warnings: params.Warnings, Error: errorOf(FailureParams{Err: params.Err, Flags: params.Flags})}
	for _, item := range params.Items {
		doc.Items = append(doc.Items, itemOf(item, params.Flags))
	}
	if doc.Warnings == nil {
		doc.Warnings = []domain.Warning{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(doc)
}

func itemOf(item Item, flags Flags) envelopeItem {
	detail := item.Detail
	if detail == nil {
		detail = struct{}{}
	}
	return envelopeItem{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Error: errorOf(FailureParams{Err: item.Err, Flags: flags}), Warnings: item.Warnings, Detail: detail}
}

func errorOf(params FailureParams) *envelopeError {
	err := params.Err
	if err == nil {
		return nil
	}
	doc := &envelopeError{Kind: err.Kind, Code: err.Code, Message: ErrorText(err), Params: err.Params}
	if follow := err.FollowUp; follow != nil {
		doc.FollowUp = &envelopeFollow{Command: follow.Command, Request: follow.Request, Code: follow.Code, Params: follow.Params}
	}
	for _, field := range err.Fields {
		flag := params.Flags.Spell(field.Path)
		doc.Fields = append(doc.Fields, envelopeField{Path: field.Path, Code: field.Code, Params: field.Params, Accepted: field.Accepted, With: field.With, Flag: flag, Message: field.Message(flag)})
	}
	return doc
}
