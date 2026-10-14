package dispatch

import (
	"context"
	"encoding/json"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

type QueryHandle[Req, V any] struct {
	query     kernel.Query[Req, V]
	messages  text.Messages
	exitCodes map[kernel.Code]int
}

type RegisterQueryParams[Req, V any] struct {
	Query     kernel.Query[Req, V]
	Messages  text.Messages
	ExitCodes map[kernel.Code]int
}

func RegisterQuery[Req, V any](params RegisterQueryParams[Req, V]) *QueryHandle[Req, V] {
	return &QueryHandle[Req, V]{query: params.Query, messages: params.Messages, exitCodes: params.ExitCodes}
}

func (h *QueryHandle[Req, V]) Name() string                   { return h.query.Name }
func (h *QueryHandle[Req, V]) Schema() []kernel.FieldSpec     { return nil }
func (h *QueryHandle[Req, V]) Messages() text.Messages        { return h.messages }
func (h *QueryHandle[Req, V]) ExitCodes() map[kernel.Code]int { return h.exitCodes }
func (*QueryHandle[Req, V]) Fillable() (Fillable, bool)       { return nil, false }

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
