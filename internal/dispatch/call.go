package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
)

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
		items = append(items, kernel.Retype(kernel.RetypeParams[D, json.RawMessage]{Item: item, Detail: detail}))
	}
	erased.Outcome = &kernel.Outcome[json.RawMessage]{Items: items, FollowUps: result.Outcome.FollowUps, Warnings: result.Outcome.Warnings}
	return erased, nil
}

const unknownFieldPrefix = "json: unknown field "

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
	// encoding/json has no typed error for an unknown field.
	if name, unknown := strings.CutPrefix(err.Error(), unknownFieldPrefix); unknown {
		return []kernel.FieldError{{Path: strings.Trim(name, `"`), Code: kernel.CodeUnknownField}}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return []kernel.FieldError{{Path: typeErr.Field, Code: kernel.CodeInvalid}}
	}
	return nil
}
