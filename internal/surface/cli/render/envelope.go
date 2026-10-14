package render

import (
	"encoding/json"
	"io"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// EnvelopeVersion is the "v" of every document a command on the engine writes.
const EnvelopeVersion = 2

// Names spells a field path the way the surface asks for it: a flag, a
// positional. A path it does not know comes back unchanged.
type Names func(path string) string

// EnvelopeParams is what a run produced; the envelope always has the same
// keys, whichever of them the run reached.
type EnvelopeParams struct {
	Command     string
	OK          bool
	Op          kernel.OpID
	Items       []kernel.Item[json.RawMessage]
	Plan        json.RawMessage
	Fingerprint string
	View        json.RawMessage
	FollowUps   []kernel.FollowUp
	Warnings    []kernel.Warning
	Err         kernel.Error
	Wording     Wording
}

type envelope struct {
	V           int               `json:"v"`
	Command     string            `json:"command"`
	OK          bool              `json:"ok"`
	Op          kernel.OpID       `json:"op,omitempty"`
	Items       []envelopeItem    `json:"items,omitempty"`
	Plan        json.RawMessage   `json:"plan,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	View        json.RawMessage   `json:"view,omitempty"`
	FollowUps   []kernel.FollowUp `json:"follow_ups"`
	Warnings    []envelopeWarning `json:"warnings"`
	Error       *envelopeError    `json:"error,omitempty"`
}

type envelopeItem struct {
	Subject string          `json:"subject"`
	Status  kernel.Status   `json:"status"`
	Reason  kernel.Reason   `json:"reason,omitempty"`
	Error   *envelopeError  `json:"error,omitempty"`
	Detail  json.RawMessage `json:"detail"`
}

type envelopeWarning struct {
	Code    kernel.Code   `json:"code"`
	Params  kernel.Params `json:"params,omitempty"`
	Message string        `json:"message"`
}

type envelopeError struct {
	Kind     kernel.Kind       `json:"kind"`
	Code     kernel.Code       `json:"code"`
	Message  string            `json:"message"`
	Params   kernel.Params     `json:"params,omitempty"`
	FollowUp *kernel.FollowUp  `json:"follow_up,omitempty"`
	Fields   []envelopeField   `json:"fields,omitempty"`
	Blockers []envelopeBlocker `json:"blockers,omitempty"`
}

type envelopeField struct {
	kernel.FieldError
	Flag    string `json:"flag"`
	Message string `json:"message"`
}

type envelopeBlocker struct {
	kernel.Blocker
	Flag    string `json:"flag,omitempty"`
	Message string `json:"message"`
}

func WriteEnvelope(w io.Writer, params EnvelopeParams) error {
	doc := envelope{
		V:           EnvelopeVersion,
		Command:     params.Command,
		OK:          params.OK,
		Op:          params.Op,
		Plan:        params.Plan,
		Fingerprint: params.Fingerprint,
		View:        params.View,
		FollowUps:   orEmpty(params.FollowUps),
		Warnings:    params.Wording.envelopeWarnings(params.Warnings),
		Error:       params.Wording.envelopeErrorOf(params.Err),
	}
	for _, item := range params.Items {
		doc.Items = append(doc.Items, envelopeItem{
			Subject: item.Subject,
			Status:  item.Status,
			Reason:  item.Reason,
			Error:   params.Wording.envelopeErrorOf(item.Error),
			Detail:  item.Detail,
		})
	}
	return encodeJSON(w, doc)
}

func (wd Wording) envelopeWarnings(warnings []kernel.Warning) []envelopeWarning {
	out := make([]envelopeWarning, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, envelopeWarning{Code: warning.Code, Params: warning.Params, Message: wd.Warning(warning)})
	}
	return out
}

func (wd Wording) envelopeErrorOf(err kernel.Error) *envelopeError {
	if err == nil {
		return nil
	}
	base := err.Base()
	doc := &envelopeError{
		Kind:     err.Kind(),
		Code:     base.Code,
		Message:  wd.Error(err),
		Params:   base.Params,
		FollowUp: base.FollowUp,
	}
	switch typed := err.(type) {
	case *kernel.InvalidError:
		for _, field := range typed.Fields {
			doc.Fields = append(doc.Fields, envelopeField{FieldError: field, Flag: wd.Names(field.Path), Message: wd.Field(field)})
		}
	case *kernel.RefusedError:
		for _, blocker := range typed.Blockers {
			doc.Blockers = append(doc.Blockers, envelopeBlocker{Blocker: blocker, Flag: wd.blockerFlag(blocker), Message: wd.Blocker(blocker)})
		}
	case *kernel.Failure:
	}
	return doc
}

func (wd Wording) blockerFlag(blocker kernel.Blocker) string {
	if blocker.Field == "" {
		return ""
	}
	return wd.Names(blocker.Field)
}

// orEmpty keeps a list key an array: a reader never tells null from none.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
