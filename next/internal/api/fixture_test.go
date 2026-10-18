package api

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type request struct {
	Name string `json:"name"`
}

// outcome reports the request it ran with as its op, so a test sees what the use case was given.
type outcome struct{ op string }

func (o outcome) Envelope(command string, err *domain.Error) domain.Envelope {
	return domain.Envelope{Command: command, Op: o.op, Err: err}
}

// useCase refuses with refusal, if any, and records the directory it was opened on.
type useCase struct {
	dir     string
	refusal error
}

func (u useCase) Run(_ context.Context, req request, opts domain.RunOptions) (outcome, error) {
	return outcome{op: u.dir + ":" + req.Name + ":" + opts.Surface}, u.refusal
}
