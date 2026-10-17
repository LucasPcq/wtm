// Package api is the catalogue of wtm-next's commands, knowing none of them: each one's name, the schema of its request and its execution, a JSON request in and an envelope out, for a surface other than the command line.
package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// surfaceAPI is the surface an operation records when its call names none.
const surfaceAPI = "api"

// A Catalogue lists the commands a surface can run, in the order it shows them.
type Catalogue []Command

// Find returns the command called name, and whether the catalogue has one.
func (c Catalogue) Find(name string) (Command, bool) {
	for _, command := range c {
		if command.Name == name {
			return command, true
		}
	}

	return Command{}, false
}

// A Command is one command of the catalogue: its name, the schema of its request as a surface renders it, and its execution.
type Command struct {
	Name    string
	Form    domain.Form
	Execute func(ctx context.Context, call Call) Response
}

// A Call is one execution of a command: the directory of the repository it runs on, its request as JSON, and its run options.
type Call struct {
	Dir     string
	Request json.RawMessage
	Options domain.RunOptions
}

// A Response is how an execution ended: its status, from the kind of its failure, and its envelope.
type Response struct {
	Status   int
	Envelope domain.Envelope
}

// An Outcome is what a command's run returns: it reports itself as an envelope, err being the run's refusal if any.
type Outcome interface {
	Envelope(err *domain.Error) domain.Envelope
}

// Bind returns the command called name, of schema form, that opens its use case on the call's directory with open, then runs it with run, a method expression such as create.UseCase.Run.
// The use case is opened on every execution, so the catalogue lists its commands outside any repository; the request is decoded strictly, a field it does not know refused.
func Bind[U, Req any, Out Outcome](name string, form domain.Form, open func(ctx context.Context, dir string) (U, error), run func(useCase U, ctx context.Context, req Req, opts domain.RunOptions) (Out, error)) Command {
	refused := func(err error) Response {
		return respond(domain.Envelope{Command: name, Err: domain.AsError(domain.AsErrorParams{Err: err})})
	}

	execute := func(ctx context.Context, call Call) Response {
		// A relative directory would resolve against the server process's own working directory.
		if !filepath.IsAbs(call.Dir) {
			return refused(domain.InvalidMessage("the directory of a " + name + " call must be absolute, not " + strconv.Quote(call.Dir)))
		}

		req, err := decoded[Req](call.Request)
		if err != nil {
			return refused(domain.InvalidMessage("invalid " + name + " request: " + err.Error()))
		}

		opts := call.Options
		opts.Surface = cmp.Or(opts.Surface, surfaceAPI)

		useCase, err := open(ctx, call.Dir)
		if err != nil {
			return refused(err)
		}

		outcome, err := run(useCase, ctx, req, opts)
		return respond(outcome.Envelope(domain.AsError(domain.AsErrorParams{Err: err})))
	}

	return Command{Name: name, Form: form, Execute: execute}
}

// decoded is the one JSON object of data as a Req, refusing a field Req does not have and anything after the object.
func decoded[Req any](data []byte) (Req, error) {
	var req Req
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return req, err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return req, errors.New("unexpected data after the request")
	}

	return req, nil
}

// statusClientClosed is the status of an operation its caller cancelled: nginx's 499, which net/http does not name.
const statusClientClosed = 499

var statusByKind = map[domain.Kind]int{
	domain.KindInvalid:      http.StatusBadRequest,
	domain.KindRefused:      http.StatusForbidden,
	domain.KindConflict:     http.StatusConflict,
	domain.KindPrecondition: http.StatusPreconditionFailed,
	domain.KindCancelled:    statusClientClosed,
	domain.KindInternal:     http.StatusInternalServerError,
}

func respond(doc domain.Envelope) Response {
	if doc.Err == nil {
		return Response{Status: http.StatusOK, Envelope: doc}
	}

	status, known := statusByKind[doc.Err.Kind]
	if !known {
		status = http.StatusInternalServerError
	}
	return Response{Status: status, Envelope: doc}
}
