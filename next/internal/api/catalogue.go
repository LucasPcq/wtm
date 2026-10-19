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

// A Command is one command of the catalogue: its name, the schema of its request as a surface renders it, its questions and its execution.
type Command struct {
	Name string
	Form domain.Form // the schema outside any repository: no option observed, no answer remembered
	// Questions is the form of the call's request as the repository stands: the options observed, the answers remembered, the problems of each field by its path; the call's options are not read.
	Questions func(ctx context.Context, call Call) (domain.Form, error)
	Execute   func(ctx context.Context, call Call) Response
}

// A Call is one execution of a command: the directory of the repository it runs on, its request and its run options.
// The request is its Answers, each field's by its path, folded as the command line folds its flags, over Request, the request's JSON, which may be empty.
type Call struct {
	Dir     string
	Request json.RawMessage
	Answers []domain.Answered
	Options domain.RunOptions
}

// A Response is how an execution ended: its status, from the kind of its failure, and its envelope.
type Response struct {
	Status   int
	Envelope domain.Envelope
}

// An Outcome is what a command's run returns: it reports itself as the envelope of command, err being the run's refusal if any.
type Outcome interface {
	Envelope(command string, err *domain.Error) domain.Envelope
}

// A Form is a command's questions as its use case observed them: its schema, as a surface renders it.
type Form interface {
	Form() domain.Form
}

// Bind returns the command called name that opens its use case on the call's directory with open, then asks it the call's request with questions or runs it with run, method expressions such as create.UseCase.Questions and create.UseCase.Run.
// The use case is opened on every call, so the catalogue lists its commands outside any repository, with the form of zero questions; the request's JSON is decoded strictly, a field it does not know refused, and an answer the request refuses is refused by its path.
func Bind[U any, Req domain.Answerable[Req], Q Form, Out Outcome](name string, open func(ctx context.Context, dir string) (U, error), questions func(useCase U, ctx context.Context, req Req) (Q, error), run func(useCase U, ctx context.Context, req Req, opts domain.RunOptions) (Out, error)) Command {
	prepare := func(ctx context.Context, call Call) (U, Req, error) {
		var (
			useCase U
			req     Req
		)
		// A relative directory would resolve against the server process's own working directory.
		if !filepath.IsAbs(call.Dir) {
			return useCase, req, domain.InvalidMessage("the directory of a " + name + " call must be absolute, not " + strconv.Quote(call.Dir))
		}

		req, err := requestOf[Req](call)
		if err != nil {
			return useCase, req, refusedRequest(name, err)
		}

		useCase, err = open(ctx, call.Dir)
		return useCase, req, err
	}

	ask := func(ctx context.Context, call Call) (domain.Form, error) {
		useCase, req, err := prepare(ctx, call)
		if err != nil {
			return domain.Form{}, err
		}

		asked, err := questions(useCase, ctx, req)
		if err != nil {
			return domain.Form{}, err
		}
		return asked.Form(), nil
	}

	execute := func(ctx context.Context, call Call) Response {
		useCase, req, err := prepare(ctx, call)
		if err != nil {
			return respond(domain.Envelope{Command: name, Err: domain.AsError(domain.AsErrorParams{Err: err})})
		}

		opts := call.Options
		opts.Surface = cmp.Or(opts.Surface, surfaceAPI)
		outcome, err := run(useCase, ctx, req, opts)
		return respond(outcome.Envelope(name, domain.AsError(domain.AsErrorParams{Err: err})))
	}

	var zero Q
	return Command{Name: name, Form: zero.Form(), Questions: ask, Execute: execute}
}

// requestOf is the call's request: its JSON decoded, none being the zero request, then each of its answers given in turn.
func requestOf[Req domain.Answerable[Req]](call Call) (Req, error) {
	var req Req
	if len(call.Request) == 0 {
		return domain.Fold(req, call.Answers)
	}

	req, err := decoded[Req](call.Request)
	if err != nil {
		return req, err
	}
	return domain.Fold(req, call.Answers)
}

// refusedRequest is err as the refusal of an invalid request: an answer's own refusal as it is, by the path it names, a JSON that does not decode worded as such.
func refusedRequest(name string, err error) error {
	if refusal, ok := errors.AsType[*domain.Error](err); ok {
		return refusal
	}

	return domain.InvalidMessage("invalid " + name + " request: " + err.Error())
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
