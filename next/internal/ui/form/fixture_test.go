package form

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

var errStale = errors.New("stale")

type fakeStep struct {
	label    string
	standing Standing
	box      *SelectList
	answer   string
	resets   int
}

func (s *fakeStep) Label() string       { return s.label }
func (s *fakeStep) Description() string { return "What " + s.label + " asks" }
func (s *fakeStep) Standing() Standing {
	standing := s.standing
	standing.Value = cmp.Or(standing.Value, s.answer)
	return standing
}
func (s *fakeStep) Field() Field { return s.box }
func (s *fakeStep) Reset()       { s.answer, s.resets = "", s.resets+1 }

func (s *fakeStep) Open() tea.Cmd {
	s.box = NewSelectList(SelectParams{Options: []Option{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}}})
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
	applied    int
}

func newFake() *fake {
	return &fake{first: &fakeStep{label: "First"}, second: &fakeStep{label: "Second"}}
}

func (f *fake) script() Script {
	return Script{
		Steps:   []Step{f.first, f.second},
		Observe: func(context.Context) (func(), error) { return func() { f.observed++ }, nil },
		Preview: func(context.Context) (func(), error) { return func() {}, f.previewErr },
		Plan: func(int, *output.Styles) string {
			return "plan: " + strings.Join([]string{f.first.answer, f.second.answer}, ", ")
		},
		Label: strings.ToUpper,
		Apply: func(context.Context) error { f.applied++; return f.applyErr },
		Stale: func(err error) bool { return errors.Is(err, errStale) },
	}
}

type refreshingStep struct {
	*fakeStep
	refreshed int
}

func (s *refreshingStep) Refreshable() bool { return true }
func (s *refreshingStep) Refreshed()        { s.refreshed++ }

func picks() []Pick {
	return []Pick{
		{Label: "a.txt", Value: "a.txt", Tag: "modified", Tone: ToneWarning},
		{Label: "b.txt", Value: "b.txt", Tag: "untracked"},
		{Label: "notes.md", Value: "notes.md"},
	}
}

// enters is a terminal on which the user keeps pressing enter.
type enters struct{}

func (enters) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = '\r'
	return 1, nil
}

func refusingBad(value string) error {
	if value == "bad" {
		return errors.New("Not a valid name")
	}
	return nil
}

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

// questions are a fake command's questions: the request as asked, its problems, and the answer remembered for mode.
type questions struct {
	req        request
	problems   []domain.FieldError
	remembered string
}

func (q questions) Form() domain.Form {
	one := func(path string) string { return strings.Join(q.req[path], "") }
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.TextListField{FieldSpec: domain.FieldSpec{Path: "branches", Flag: "<branches>", Label: "Branches", Help: "The branches"}, Entries: q.req["branches"], Options: []domain.Option{{Value: "feature", Badge: "existing"}}},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: "from", Flag: "--from", Label: "Source", Help: "The source"}, Value: one("from"), Options: []domain.Option{{Value: "main"}, {Value: "dev", Badge: "1 behind"}}, Pinned: "dev", Refreshable: true},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: "mode", Flag: "--mode", Label: "Mode", Help: "How"}, Value: one("mode"), Options: []domain.Option{{Value: "fast"}, {Value: "slow", Short: "slowly"}}, Remembered: q.remembered, Rememberable: true, Remember: slices.Contains(q.req[domain.PathRemember], "mode")},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: "ff", Flag: "--ff", Label: "Update", Help: "Update first"}, Value: one("ff") == "true", Rememberable: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: "keep", Flag: "--keep", Label: "Keep", Help: "Keep it"}, Value: one("keep") == "true", Yes: domain.Option{Label: "Copy", Short: "copy"}, No: domain.Option{Label: "Move", Short: "move"}, NoFirst: true},
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: "files", Flag: "--files", Label: "Files", Help: "The files"}, Picked: q.req["files"], Options: []domain.Option{{Value: "a.txt"}, {Value: "b.txt"}}, Empty: "Pick one"},
		domain.TextField{FieldSpec: domain.FieldSpec{Path: "name", Flag: "--name", Label: "Name", Help: "The name"}, Value: one("name")},
	}}
}

type outcome struct {
	req         request
	fingerprint string
}

func (o outcome) PlanFingerprint() string { return o.fingerprint }

// app is a fake use case: a branch "taken" is refused, and each run is recorded.
type app struct {
	remembered string
	runs       []domain.RunOptions
}

func (a *app) questions(_ context.Context, req request) (questions, error) {
	q := questions{req: req, remembered: a.remembered}
	for i, branch := range req["branches"] {
		if branch == "taken" {
			q.problems = append(q.problems, domain.FieldError{Path: "branches[" + strconv.Itoa(i) + "]", Template: "{value} is taken", Params: domain.Params{"value": branch}})
		}
	}
	return q, nil
}

func (a *app) run(_ context.Context, req request, opts domain.RunOptions) (outcome, error) {
	a.runs = append(a.runs, opts)
	return outcome{req: req, fingerprint: "print"}, nil
}

func fakeCommand() *Command[*app, request, questions, outcome] {
	return &Command[*app, request, questions, outcome]{
		YesUsage:  "Run unattended",
		Open:      func(context.Context, string) (*app, error) { return &app{}, nil },
		Questions: (*app).questions,
		Run:       (*app).run,
		Plan: func(o outcome, _ int, _ *output.Styles) string {
			return "plan of " + strings.Join(o.req["branches"], ", ")
		},
	}
}

// observed is a session of the fake command over a, its first read landed.
func observed(t *testing.T, a *app, given request) *Session[*app, request, questions, outcome] {
	t.Helper()
	s := fakeCommand().Session(t.Context(), SessionParams[*app, request]{App: a, Request: given})
	apply, err := s.observe(t.Context())
	require.NoError(t, err)
	apply()
	return s
}
