package form_test

import (
	"cmp"
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
	tea "github.com/charmbracelet/bubbletea"
)

var errStale = errors.New("stale")

// failingAfterFirst is an Observe whose first read succeeds and every later one fails with err, counted in reads: Observe runs in a cmd, on a goroutine of Bubble Tea's own.
func failingAfterFirst(reads *atomic.Int32, err error) func(context.Context) (func(), error) {
	return func(context.Context) (func(), error) {
		if reads.Add(1) > 1 {
			return func() {}, err
		}
		return func() {}, nil
	}
}

type fakeStep struct {
	label    string
	standing form.Standing
	box      *widget.SelectList
	answer   string
	resets   int
}

func (s *fakeStep) Label() string       { return s.label }
func (s *fakeStep) Description() string { return "What " + s.label + " asks" }
func (s *fakeStep) Standing() form.Standing {
	standing := s.standing
	standing.Value = cmp.Or(standing.Value, s.answer)
	return standing
}
func (s *fakeStep) Widget() widget.Widget { return s.box }
func (s *fakeStep) Reset()                { s.answer, s.resets = "", s.resets+1 }

func (s *fakeStep) Open() tea.Cmd {
	s.box = widget.NewSelectList(widget.SelectParams{Options: []widget.Option{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}}})
	return nil
}

func (s *fakeStep) Answer() string {
	s.answer = s.box.Value()
	return s.answer
}

type fake struct {
	first      *fakeStep
	second     *fakeStep
	observed   int
	previewErr error
	applyErr   error
	applied    atomic.Int32 // set from the apply's cmd, which Bubble Tea runs on a goroutine of its own
}

func newFake() *fake {
	return &fake{first: &fakeStep{label: "First"}, second: &fakeStep{label: "Second"}}
}

func (f *fake) script() form.Script {
	return form.Script{
		Steps:   []form.Step{f.first, f.second},
		Observe: func(context.Context) (func(), error) { return func() { f.observed++ }, nil },
		Preview: func(context.Context) (func(), error) { return func() {}, f.previewErr },
		Plan: func(int, *output.Styles) string {
			return "plan: " + strings.Join([]string{f.first.answer, f.second.answer}, ", ")
		},
		Label: strings.ToUpper,
		Apply: func(context.Context) error { f.applied.Add(1); return f.applyErr },
		Stale: func(err error) bool { return errors.Is(err, errStale) },
	}
}

type refreshingStep struct {
	*fakeStep
	refreshed int
}

func (s *refreshingStep) Refreshable() bool { return true }
func (s *refreshingStep) Refreshed()        { s.refreshed++ }

// A request is a fake command's request: the values of each path given.
type request map[string][]string

func (r request) Answer(path string, values []string) (request, error) {
	if path == "nope" {
		return r, domain.Unknown(path)
	}

	next := maps.Clone(r)
	if next == nil {
		next = request{}
	}
	if values == nil {
		delete(next, path)
	} else {
		next[path] = values
	}
	return next, nil
}

func (r request) Values(path string) []string { return r[path] }

// Skips rules the worktree out of a request that reads them all.
func (r request) Skips(path string) domain.Skip {
	if path == "worktree" && len(r["all"]) > 0 {
		return domain.SkipAll
	}
	return ""
}

// questions are a fake command's questions: the request as asked, its problems, and the answer remembered for mode.
type questions struct {
	req        request
	problems   []domain.FieldError
	remembered string
}

func (q questions) Form() domain.Form {
	one := func(path string) string { return strings.Join(q.req[path], "") }
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.TextListField{FieldSpec: domain.FieldSpec{Path: "branches"}, Entries: q.req["branches"], Options: []domain.Option{{Value: "feature", Tag: domain.TagLocal}}},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: "from"}, Value: one("from"), Options: []domain.Option{{Value: "main"}, {Value: "dev", Origin: &worktree.Origin{Behind: 1, State: worktree.Behind}}}, Pinned: "dev", Refreshable: true},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: "mode"}, Value: one("mode"), Options: []domain.Option{{Value: "fast"}, {Value: "slow"}}, Remembered: q.remembered, Rememberable: true, Remember: slices.Contains(q.req[domain.PathRemember], "mode")},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: "ff"}, Value: one("ff") == "true", Rememberable: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: "keep"}, Value: one("keep") == "true", NoFirst: true},
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: "files"}, Picked: q.req["files"], Options: []domain.Option{{Value: "a.txt"}, {Value: "b.txt"}}},
		domain.TextField{FieldSpec: domain.FieldSpec{Path: "name", Skip: domain.Skip(one("unworded"))}, Value: one("name")},
	}}
}

// fakeWords are how the command line words the fake command's fields.
var fakeWords = map[string]form.Words{
	"branches": {Flag: "<branches>", Label: "Branches", Help: "The branches"},
	"from":     {Flag: "--from", Label: "Source", Help: "The source"},
	"mode":     {Flag: "--mode", Label: "Mode", Help: "How", Options: map[string]form.OptionWords{"slow": {Short: "slowly"}}},
	"ff":       {Flag: "--ff", Label: "Update", Help: "Update first"},
	"keep":     {Flag: "--keep", Label: "Keep", Help: "Keep it", Options: map[string]form.OptionWords{"true": {Label: "Copy", Short: "copy"}, "false": {Label: "Move", Short: "move"}}},
	"files":    {Flag: "--files", Label: "Files", Help: "The files", Empty: "Pick one"},
	"name":     {Flag: "--name", Label: "Name", Help: "The name"},
	"worktree": {Flag: "<worktree>", Label: "Worktree", Help: "The worktree to read"},
	"all":      {Flag: "--all", Label: "All", Help: "Every worktree"},
}

func wordsOf[Q any](_ Q, field domain.Field) form.Words {
	return fakeWords[field.Spec().Path]
}

// An outcome is a run of the fake command: its plan is the request it ran.
type outcome = domain.Outcome[request, struct{}]

// app is a fake use case: a branch "taken" is refused, and each run is recorded.
type app struct {
	remembered string
	runs       []domain.RunOptions
}

func (a *app) Questions(_ context.Context, req request) (questions, error) {
	q := questions{req: req, remembered: a.remembered}
	for i, branch := range req["branches"] {
		if branch == "taken" {
			q.problems = append(q.problems, domain.FieldError{Path: "branches[" + strconv.Itoa(i) + "]", Template: "{value} is taken", Params: domain.Params{"value": branch}})
		}
	}
	return q, nil
}

func (a *app) Run(_ context.Context, req request, opts domain.RunOptions) (outcome, error) {
	a.runs = append(a.runs, opts)
	return outcome{Plan: req, Fingerprint: "print"}, nil
}

func fakeCommand() *form.Command[request, questions, outcome] {
	return &form.Command[request, questions, outcome]{
		YesUsage: "Run unattended",
		Words:    wordsOf[questions],
		Open:     func(context.Context, string) (form.UseCase[request, questions, outcome], error) { return &app{}, nil },
		Preview: func(o outcome) form.Preview {
			return form.Preview{Rows: []output.PlanRow{{Label: "plan of", Values: o.Plan["branches"]}}}
		},
	}
}

// observed is a session of the fake command over a, its first read landed.
func observed(t *testing.T, a *app, given request) *form.Session[request, questions, outcome] {
	t.Helper()
	s := fakeCommand().Session(t.Context(), a, given, domain.RunOptions{})
	apply, err := s.Observe(t.Context())
	require.NoError(t, err)
	apply()
	return s
}

// A read is what the fake read command found.
type read = domain.Read[struct{}]

// reader is a fake read use case that finds read, asking nothing.
type reader struct{ read read }

func (r reader) Questions(_ context.Context, req request) (questions, error) {
	return questions{req: req}, nil
}

func (r reader) Run(context.Context, request, domain.RunOptions) (read, error) { return r.read, nil }

// ambiguousName is a worktree's name the fake read about one worktree refuses as ambiguous: feat/y and feat/z hold it.
const ambiguousName = "feat"

// subjects are the questions of a fake read about one worktree, its subject: a positional select on main, unless the request reads them all.
type subjects struct{ req request }

func (q subjects) Form() domain.Form {
	field := domain.SelectField{FieldSpec: domain.FieldSpec{Path: "worktree"}, Value: "main", Options: []domain.Option{{Value: "main"}, {Value: "feat/x", Detached: true}}}
	if len(q.req["all"]) > 0 {
		field.Skip = domain.SkipAll
	}
	// an ambiguous name narrows the options to what it matches
	if slices.Equal(q.req["worktree"], []string{ambiguousName}) {
		field.Value, field.Options = "", []domain.Option{{Value: "feat/y"}, {Value: "feat/z"}}
	}
	return domain.Form{Fields: []domain.Field{field, domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: "all"}}}}
}

// subjectReader is a fake read about one worktree, recording the request it read and counting its questions read.
type subjectReader struct {
	read  *request
	asked *int
}

func (r subjectReader) Questions(_ context.Context, req request) (subjects, error) {
	*r.asked++
	return subjects{req: req}, nil
}

func (r subjectReader) Run(_ context.Context, req request, _ domain.RunOptions) (read, error) {
	*r.read = req
	if slices.Equal(req["worktree"], []string{ambiguousName}) {
		return read{}, domain.Invalid([]domain.FieldError{{Path: "worktree", Code: domain.CodeAmbiguous, Template: "{value} matches several worktrees", Params: domain.Params{"value": ambiguousName}}})
	}
	return read{Items: []domain.Unit[struct{}]{{Subject: strings.Join(req["worktree"], ""), Status: domain.StatusDone}}}, nil
}

// subjectQuery is a read about one worktree, the request it read once launched, and how many times it read its questions.
func subjectQuery() (*form.Query[request, subjects, read], *request, *int) {
	got, asked := request{}, 0
	return &form.Query[request, subjects, read]{
		Words: wordsOf[subjects],
		Open: func(context.Context, string) (form.Reader[request, subjects, read], error) {
			return subjectReader{read: &got, asked: &asked}, nil
		},
		Draw: func(w io.Writer, got read, _ request, _ output.View) {
			_, _ = io.WriteString(w, "read "+got.Items[0].Subject+"\n")
		},
	}, &got, &asked
}

// fakeQuery is a read command over the fake request, finding one subject, whose picker picks `wtm go feat/x` and counts its picks.
func fakeQuery() (*form.Query[request, questions, read], *int) {
	picked := 0
	return &form.Query[request, questions, read]{
		Words: wordsOf[questions],
		Open: func(context.Context, string) (form.Reader[request, questions, read], error) {
			return reader{read: read{Items: []domain.Unit[struct{}]{{Subject: "feat/x", Status: domain.StatusDone}}}}, nil
		},
		Draw: func(w io.Writer, got read, _ request, _ output.View) {
			_, _ = io.WriteString(w, "read "+strconv.Itoa(len(got.Items))+"\n")
		},
		Pick: func(_ context.Context, got read, _ request, _ *uicli.Run) (read, error) {
			picked++
			got.FollowUps = []domain.FollowUp{{Command: "go", Answers: []domain.Answered{{Path: "worktree", Values: []string{"feat/x"}}}, Message: "Go there"}}
			return got, nil
		},
	}, &picked
}
