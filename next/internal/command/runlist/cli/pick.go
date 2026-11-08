package cli

import (
	"context"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	entryLabel  = "Profile or job"
	entryHelp   = "Select a profile or a job of "
	actionLabel = "Action"
	profileKind = "profile"
	jobKind     = "job"
	jobsCounted = " jobs"
)

// pick asks the user a profile or a job of the worktree read, then what to do with it, and returns read with the command line that does it as its follow-up.
func pick(ctx context.Context, read runlist.Outcome, _ runlist.Request, run *uicli.Run) (runlist.Outcome, error) {
	if len(read.Items) != 1 || read.Items[0].Detail == nil {
		return read, nil
	}
	p := &picker{subject: read.Items[0].Subject, detail: *read.Items[0].Detail}
	if err := form.Walk(ctx, p.script(), run.Streams.In, run.Streams.Err); err != nil {
		return read, err
	}

	read.FollowUps = []domain.FollowUp{p.action}
	return read, nil
}

// A picker is the walk of the picker: the worktree read, the entry picked (a kind and a name) and the action picked of those it offers.
type picker struct {
	subject string
	detail  runlist.Detail
	entry   string
	action  domain.FollowUp
}

// script walks the entry, then the action, over the read the picker opened on: nothing an answer changes needs reading again.
func (p *picker) script() form.Script {
	return form.Script{Steps: []form.Step{&entryStep{p: p}, &actionStep{p: p}}, Observe: func(context.Context) (func(), error) { return func() {}, nil }, Label: func(path string) string { return path }}
}

// offered are the actions of the entry picked.
func (p *picker) offered() []domain.FollowUp {
	kind, name, _ := strings.Cut(p.entry, ":")
	switch kind {
	case profileKind:
		at := slices.IndexFunc(p.detail.Profiles, func(pr runlist.Profile) bool { return pr.Name == name })
		if at >= 0 {
			return p.detail.Profiles[at].Actions
		}
	case jobKind:
		at := slices.IndexFunc(p.detail.Jobs, func(job runlist.Job) bool { return job.Name == name })
		if at >= 0 {
			return p.detail.Jobs[at].Actions
		}
	}
	return nil
}

// name is the name of the entry picked, as the trail shows it.
func (p *picker) name() string {
	_, name, _ := strings.Cut(p.entry, ":")
	return name
}

// ----------------------------------------------------------------------------
// The entry

// entryStep asks a profile or a job: the profiles first, then a rule, then the jobs, each badged as its row of the table.
type entryStep struct {
	p    *picker
	list *widget.SelectList
}

func (s *entryStep) Label() string { return entryLabel }

func (s *entryStep) Description() string { return entryHelp + s.p.subject }

func (s *entryStep) Standing() form.Standing { return form.Standing{Value: s.p.name()} }

func (s *entryStep) Open() tea.Cmd {
	st := output.StylesOf(io.Discard)
	var options []widget.Option
	for _, pr := range s.p.detail.Profiles {
		options = append(options, widget.Option{Label: pr.Name, Value: profileKind + ":" + pr.Name, Badge: strings.TrimSpace(profileTags(pr, st) + "  " + strconv.Itoa(len(pr.Jobs)) + jobsCounted)})
	}
	if len(options) > 0 && len(s.p.detail.Jobs) > 0 {
		options = append(options, widget.Option{Separator: true})
	}
	for _, job := range s.p.detail.Jobs {
		options = append(options, widget.Option{Label: job.Name, Value: jobKind + ":" + job.Name, Badge: string(job.Kind) + "  " + stateOf(job, st)})
	}
	s.list = widget.NewSelectList(widget.SelectParams{Options: options, Start: s.p.entry})
	return nil
}

func (s *entryStep) Widget() widget.Widget { return s.list }

func (s *entryStep) Answer() string {
	s.p.entry = s.list.Value()
	return s.p.name()
}

func (s *entryStep) Reset() { s.p.entry = "" }

// ----------------------------------------------------------------------------
// The action

// actionStep asks what to do with the entry picked, among what it offers.
type actionStep struct {
	p    *picker
	list *widget.SelectList
}

func (s *actionStep) Label() string { return actionLabel }

func (s *actionStep) Description() string { return "What to do with " + s.p.name() }

func (s *actionStep) Standing() form.Standing { return form.Standing{Value: s.p.action.Message} }

func (s *actionStep) Open() tea.Cmd {
	var options []widget.Option
	for _, action := range s.p.offered() {
		options = append(options, widget.Option{Label: action.Message, Value: action.Code})
	}
	s.list = widget.NewSelectList(widget.SelectParams{Options: options})
	return nil
}

func (s *actionStep) Widget() widget.Widget { return s.list }

func (s *actionStep) Answer() string {
	offered := s.p.offered()
	if at := slices.IndexFunc(offered, func(action domain.FollowUp) bool { return action.Code == s.list.Value() }); at >= 0 {
		s.p.action = offered[at]
	}
	return s.p.action.Message
}

func (s *actionStep) Reset() { s.p.action = domain.FollowUp{} }
