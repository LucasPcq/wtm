// Package api is the catalogue of wtm-next's commands, knowing none of them: each one's name, the schema of its request and its execution, a JSON request in and an envelope out, for a surface other than the command line.
package api

import (
	"cmp"
	"context"
	"net/http"
	"path/filepath"
	"reflect"
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

// A Command is one command of the catalogue: its name, the schema of its request as a surface renders it, the Go types of the plan and the detail its envelope carries, its questions and its execution.
type Command struct {
	Name   string
	Form   domain.Form  // the schema outside any repository: no option observed, no answer remembered
	Plan   reflect.Type // nil for a read, which has no plan
	Detail reflect.Type
	// Questions is the form of the call's request as the repository stands: the options observed, the answers remembered, the problems of each field by its path; the call's options are not read.
	Questions func(ctx context.Context, call Call) (domain.Form, error)
	Execute   func(ctx context.Context, call Call) Response
}

// A Call is one execution of a command: the directory of the repository it runs on, which is also where its caller stands (a relative path it answers is read from it, the worktree it stands in found by it, once api resolves its symbolic links with filepath.EvalSymlinks: the one place a surface's directory is resolved), its request as the answer of each field by its path, folded as the command line folds its flags, and its run options.
type Call struct {
	Dir     string
	Answers []domain.Answered
	Options domain.RunOptions
}

// caller is where the call's caller stands: in the directory it names, named, or nowhere when it names none. Cwd is that directory with its symbolic links resolved, as git spells the worktrees it lists and the command line its working directory.
func (c Call) caller() domain.Caller {
	if c.Dir == "" {
		return domain.Caller{}
	}

	cwd := c.Dir
	if resolved, err := filepath.EvalSymlinks(c.Dir); err == nil {
		cwd = resolved
	}
	return domain.Caller{Dir: c.Dir, Named: true, Cwd: cwd}
}

// A Response is how an execution ended: its status, from the kind of its failure, and its envelope.
type Response struct {
	Status   int
	Envelope domain.Envelope
}

// An Outcome is what a command's run returns: it reports itself as an envelope, a stale plan followed up by its replan, and says the Go types of the plan and the detail that envelope carries.
type Outcome interface {
	Envelope(params domain.EnvelopeParams) domain.Envelope
	Types() (plan, detail reflect.Type)
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
	var (
		zero Q
		out  Out
	)
	form := zero.Form()
	plan, detail := out.Types()

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
		req = domain.CalledBy(req, call.caller())

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
		return respond(outcome.Envelope(domain.EnvelopeParams{Command: name, Err: domain.AsError(domain.AsErrorParams{Err: err}), Answers: domain.AnswersOf(form, req)}))
	}

	return Command{Name: name, Form: form, Plan: plan, Detail: detail, Questions: ask, Execute: execute}
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
