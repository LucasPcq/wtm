package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Stream is a command that sends lines until its caller stops it rather than returning one result (wtm events): its name, the schema of its request, and its watch, which sends each line of the call to the sink until ctx is done.
// It responds once the stream ends: no error once ctx is done, else the envelope of the refusal or failure that ended it, a refusal before any line among them.
type Stream struct {
	Name  string
	Form  domain.Form
	Watch func(ctx context.Context, call Call, sink Sink) Response
}

// A Sink is where a stream goes: Send receives each line, its error ending the stream; Warn receives what kept the stream from a line or a repository for a while (the bus lost and joined again, a repository skipped), nil dropping it.
type Sink struct {
	Send func(line json.RawMessage) error
	Warn func(err error)
}

// BindStream returns the stream called name that folds the call's answers into its request, refusing an answer by its path and a relative directory, then watches with watch, the request asked from the call's directory, or from nowhere when the call names none; the call's options are not read.
func BindStream[Req Request[Req], Q Form](name string, watch func(ctx context.Context, req Req, sink Sink) error) Stream {
	var zero Q

	run := func(ctx context.Context, call Call, sink Sink) Response {
		// A relative directory would resolve against the server process's own working directory.
		if call.Dir != "" && !filepath.IsAbs(call.Dir) {
			return Response{Envelope: domain.Envelope{Command: name, Err: domain.InvalidMessage("the directory of a " + name + " call must be absolute, not " + strconv.Quote(call.Dir))}}
		}

		var req Req
		req, err := domain.Fold(req, call.Answers)
		if err == nil {
			err = watch(ctx, domain.CalledBy(req, call.caller()), sink)
		}
		if err == nil || ctx.Err() != nil {
			return Response{Envelope: domain.Envelope{Command: name}}
		}
		return Response{Envelope: domain.Envelope{Command: name, Err: domain.AsError(domain.AsErrorParams{Err: err})}}
	}

	return Stream{Name: name, Form: zero.Form(), Watch: run}
}
