package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
)

// Entry erases a command's or a query's types: the request in and the details
// out are JSON.
type Entry interface {
	Name() string
	Schema() []kernel.FieldSpec
	Call(ctx context.Context, call Call) (Reply, error)
}

type Call struct {
	Request  json.RawMessage
	Options  Options
	Progress kernel.Emitter
}

//go-sumtype:decl Reply

type Reply interface{ reply() }

type CommandReply struct {
	Result[json.RawMessage, json.RawMessage]
}

type QueryReply struct {
	View json.RawMessage
}

func (CommandReply) reply() {}
func (QueryReply) reply()   {}

type Registry []Entry

// Registry is every command and query the surfaces serve, on d.
func (d *Dispatcher) Registry() Registry {
	return Registry{RegisterQuery(operations.Query(d.Operations))}
}

func (r Registry) Lookup(name string) (Entry, bool) {
	index := slices.IndexFunc(r, func(entry Entry) bool { return entry.Name() == name })
	if index < 0 {
		return nil, false
	}
	return r[index], true
}

func (h *Handle[Req, F, P, D]) Call(ctx context.Context, call Call) (Reply, error) {
	request, err := decodeRequest[Req](call.Request)
	if err != nil {
		return nil, err
	}
	result, err := h.Run(ctx, RunParams[Req]{Request: request, Options: call.Options, Progress: call.Progress})
	erased, encodeErr := erase(result)
	if encodeErr != nil {
		return nil, classify(encodeErr)
	}
	return CommandReply{Result: erased}, err
}

func erase[P, D any](result Result[P, D]) (Result[json.RawMessage, json.RawMessage], error) {
	detail, err := json.Marshal(result.Plan.Detail)
	if err != nil {
		return Result[json.RawMessage, json.RawMessage]{}, err
	}
	erased := Result[json.RawMessage, json.RawMessage]{
		Op:          result.Op,
		Fingerprint: result.Fingerprint,
		Plan:        kernel.Plan[json.RawMessage]{Detail: detail, Warnings: result.Plan.Warnings},
	}
	if result.Outcome == nil {
		return erased, nil
	}
	items := make([]kernel.Item[json.RawMessage], 0, len(result.Outcome.Items))
	for _, item := range result.Outcome.Items {
		detail, err := json.Marshal(item.Detail)
		if err != nil {
			return Result[json.RawMessage, json.RawMessage]{}, err
		}
		items = append(items, kernel.Item[json.RawMessage]{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Error: item.Error, Detail: detail})
	}
	erased.Outcome = &kernel.Outcome[json.RawMessage]{Items: items, FollowUps: result.Outcome.FollowUps, Warnings: result.Outcome.Warnings}
	return erased, nil
}

// QueryHandle locks nothing: an atomic write leaves a file as it was before or
// after, never half written.
type QueryHandle[Req, V any] struct {
	query kernel.Query[Req, V]
}

func RegisterQuery[Req, V any](query kernel.Query[Req, V]) *QueryHandle[Req, V] {
	return &QueryHandle[Req, V]{query: query}
}

func (h *QueryHandle[Req, V]) Name() string               { return h.query.Name }
func (h *QueryHandle[Req, V]) Schema() []kernel.FieldSpec { return nil }

func (h *QueryHandle[Req, V]) Run(ctx context.Context, request Req) (V, error) {
	view, err := h.query.Run(ctx, request)
	if err != nil {
		return view, classify(err)
	}
	return view, nil
}

func (h *QueryHandle[Req, V]) Call(ctx context.Context, call Call) (Reply, error) {
	request, err := decodeRequest[Req](call.Request)
	if err != nil {
		return nil, err
	}
	view, err := h.Run(ctx, request)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(view)
	if err != nil {
		return nil, classify(err)
	}
	return QueryReply{View: data}, nil
}

// encoding/json has no typed error for an unknown field.
const unknownFieldPrefix = "json: unknown field "

// decodeRequest refuses an unknown field: a typo must not run a command with a
// default in its place.
func decodeRequest[Req any](raw json.RawMessage) (Req, error) {
	var request Req
	if len(bytes.TrimSpace(raw)) == 0 {
		return request, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&request)
	if err == nil {
		return request, nil
	}
	invalid := kernel.Invalid(fieldsOf(err))
	invalid.Cause = err
	return request, invalid
}

func fieldsOf(err error) []kernel.FieldError {
	if name, unknown := strings.CutPrefix(err.Error(), unknownFieldPrefix); unknown {
		return []kernel.FieldError{{Path: strings.Trim(name, `"`), Code: kernel.CodeUnknownField}}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return []kernel.FieldError{{Path: typeErr.Field, Code: kernel.CodeInvalid}}
	}
	return nil
}
