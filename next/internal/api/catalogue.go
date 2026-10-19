// Package api is the catalogue of wtm-next's commands, knowing none of them: each one's name, the schema of its request and its execution, a JSON request in and an envelope out, for a surface other than the command line.
package api

import (
	"cmp"
	"context"
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

// A Call is one execution of a command: the directory of the repository it runs on, its request as the answer of each field by its path, folded as the command line folds its flags, and its run options.
type Call struct {
	Dir     string
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

// A Request is a command's request: it takes the answer of a field at a time and says the answer it gives each.
type Request[R any] interface {
	domain.Answerable[R]
	domain.Valued
}

// A Form is a command's questions as its use case observed them: its schema, as a surface renders it.
type Form interface {
	Form() domain.Form
}

// Bind returns the command called name that opens its use case on the call's directory with open, then asks it the call's request with questions or runs it with run, method expressions such as create.UseCase.Questions and create.UseCase.Run.
// The use case is opened on every call, so the catalogue lists its commands outside any repository, with the form of zero questions; an answer the request refuses is refused by its path. A stale plan is followed up by its request as a dry run, as the command line follows it up.
func Bind[U any, Req Request[Req], Q Form, Out Outcome](name string, open func(ctx context.Context, dir string) (U, error), questions func(useCase U, ctx context.Context, req Req) (Q, error), run func(useCase U, ctx context.Context, req Req, opts domain.RunOptions) (Out, error)) Command {
	var zero Q
	form := zero.Form()

	prepare := func(ctx context.Context, call Call) (U, Req, error) {
		var (
			useCase U
			req     Req
		)
		// A relative directory would resolve against the server process's own working directory.
		if !filepath.IsAbs(call.Dir) {
			return useCase, req, domain.InvalidMessage("the directory of a " + name + " call must be absolute, not " + strconv.Quote(call.Dir))
		}

		req, err := domain.Fold(req, call.Answers)
		if err != nil {
			return useCase, req, err
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
		doc := outcome.Envelope(name, domain.AsError(domain.AsErrorParams{Err: err}))
		if doc.Err != nil && doc.Err.Code == domain.CodePlanStale {
			doc.FollowUps = append(doc.FollowUps, domain.Replan(name, domain.AnswersOf(form, req)))
		}
		return respond(doc)
	}

	return Command{Name: name, Form: form, Questions: ask, Execute: execute}
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
