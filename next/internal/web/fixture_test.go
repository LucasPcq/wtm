package web

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const token = "secret"

var repo = domain.Repo{Root: "/code/app", CommonDir: "/code/app/.git"}

type request struct{ Name string }

func (r request) Answer(path string, values []string) (request, error) {
	if path != "name" {
		return r, domain.Unknown(path)
	}
	name, err := domain.One[string](path, values)
	r.Name = name
	return r, err
}

func (r request) Values(path string) []string {
	if path != "name" || r.Name == "" {
		return nil
	}
	return []string{r.Name}
}

// questions label their one field with the request they were asked with.
type questions struct{ asked string }

func (q questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{domain.TextField{FieldSpec: domain.FieldSpec{Path: "name"}, Value: q.asked}}}
}

// outcome reports what it ran with as its op.
type outcome struct{ op string }

func (o outcome) Envelope(params domain.EnvelopeParams) domain.Envelope {
	return domain.Outcome[struct{}, struct{}]{Op: o.op}.Envelope(params)
}

func (o outcome) Types() (reflect.Type, reflect.Type) {
	return domain.Outcome[struct{}, struct{}]{}.Types()
}

// useCase greets the name it is asked: it reports one event, sends its correlation id to started, waits for release when there is one, then ends: cancelled when its context is, else refused with refusal if any.
type useCase struct {
	dir     string
	refusal error
	started chan string
	release chan struct{}
	ended   chan error // receives its context's error as the run ends
}

func (u useCase) Questions(_ context.Context, req request) (questions, error) {
	return questions{asked: u.dir + ":" + req.Name}, u.refusal
}

func (u useCase) Run(ctx context.Context, req request, opts domain.RunOptions) (outcome, error) {
	if opts.Report != nil {
		opts.Report(domain.UnitStarted{Subject: req.Name})
	}
	if u.started != nil {
		u.started <- opts.CorrelationID
	}
	if u.release != nil {
		select {
		case <-u.release:
		case <-ctx.Done():
		}
	}
	if u.ended != nil {
		u.ended <- ctx.Err()
	}
	if ctx.Err() != nil {
		return outcome{}, &domain.Error{Kind: domain.KindCancelled, Code: "greet.cancelled"}
	}
	return outcome{op: u.dir + ":" + req.Name + ":" + string(opts.Surface) + ":" + opts.CorrelationID}, u.refusal
}

func greet(u useCase) api.Command {
	return api.Bind("greet", func(_ context.Context, dir string) (useCase, error) {
		u.dir = dir
		return u, nil
	}, useCase.Questions, useCase.Run)
}

// stream sends its lines and warnings, then refuses with refusal if any, or waits for its end.
type stream struct {
	mu       sync.Mutex
	calls    []api.Call
	lines    []string
	warnings []error
	refusal  error
}

func (s *stream) bind() api.Stream {
	return api.Stream{Name: "events", Watch: func(ctx context.Context, call api.Call, sink api.Sink) api.Response {
		s.mu.Lock()
		s.calls = append(s.calls, call)
		s.mu.Unlock()
		for _, err := range s.warnings {
			sink.Warn(err)
		}
		for _, line := range s.lines {
			if err := sink.Send(json.RawMessage(line)); err != nil {
				return api.Response{}
			}
		}
		if s.refusal != nil {
			return api.Response{Envelope: domain.Envelope{Command: "events", Err: domain.AsError(domain.AsErrorParams{Err: s.refusal})}}
		}
		<-ctx.Done()
		return api.Response{Envelope: domain.Envelope{Command: "events"}}
	}}
}
