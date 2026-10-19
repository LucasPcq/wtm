package api

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type request struct {
	Name string `json:"name"`
	Loud bool   `json:"loud"`
}

func (r request) Answer(path string, values []string) (request, error) {
	if path != "name" {
		return r, domain.Unknown(path)
	}

	name, err := domain.One[string](path, values)
	r.Name = name
	return r, err
}

// questions name the request they were asked with as the label of their one field, so a test sees what the use case was given.
type questions struct{ asked string }

func (q questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{domain.TextField{FieldSpec: domain.FieldSpec{Path: "name", Label: q.asked}}}}
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

func (u useCase) Questions(_ context.Context, req request) (questions, error) {
	return questions{asked: u.dir + ":" + req.Name}, u.refusal
}

func (u useCase) Run(_ context.Context, req request, opts domain.RunOptions) (outcome, error) {
	return outcome{op: u.dir + ":" + req.Name + ":" + opts.Surface}, u.refusal
}
