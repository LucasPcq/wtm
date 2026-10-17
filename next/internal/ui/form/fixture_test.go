package form

import (
	"cmp"
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var errStale = errors.New("stale")

type fakeStep struct {
	label    string
	standing Standing
	box      *Confirm
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
	s.box = NewConfirm(true)
	return nil
}

func (s *fakeStep) Answer() string {
	s.answer = "no"
	if s.box.Yes() {
		s.answer = "yes"
	}
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
		Observe: func(context.Context) error { f.observed++; return nil },
		Preview: func(context.Context) error { return f.previewErr },
		Plan:    func(int) string { return "plan: " + strings.Join([]string{f.first.answer, f.second.answer}, ", ") },
		Label:   strings.ToUpper,
		Apply:   func(context.Context) error { f.applied++; return f.applyErr },
		Stale:   func(err error) bool { return errors.Is(err, errStale) },
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
