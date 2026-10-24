package api

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type request struct {
	Name   string
	Caller domain.Caller
}

func (r request) From(caller domain.Caller) request {
	r.Caller = caller
	return r
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

// outcome reports the request it ran with as its op, so a test sees what the use case was given; its plan is a plan and its detail a detail.
type outcome struct{ op string }

type (
	plan   struct{ Steps int }
	detail struct{ Path string }
)

func (o outcome) Envelope(params domain.EnvelopeParams) domain.Envelope {
	return domain.Outcome[plan, detail]{Op: o.op}.Envelope(params)
}

func (o outcome) Types() (reflect.Type, reflect.Type) {
	return domain.Outcome[plan, detail]{}.Types()
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

func (r request) Values(path string) []string {
	if path != "name" || r.Name == "" {
		return nil
	}
	return []string{r.Name}
}

// watched records what a stream's watch was given and sends lines, failing with refusal once sent.
type watched struct {
	watched bool
	req     request
	lines   []string
	refusal error
}

func (w *watched) watch(_ context.Context, req request, send func(json.RawMessage) error) error {
	w.watched, w.req = true, req
	for _, line := range w.lines {
		if err := send(json.RawMessage(line)); err != nil {
			return err
		}
	}
	return w.refusal
}
