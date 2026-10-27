package cli

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	worktreeLabel = "Worktree"
	worktreeHelp  = "Select a worktree"
	actionLabel   = "Action"
	loadingPRs    = "loading pull requests…"
)

// pick asks the user a worktree of read, then what to do with it, and returns read with the command line that does it as its follow-up. The pull requests load once the worktrees are shown, and r reads everything again after fetching origin.
func (c *listCommand) pick(ctx context.Context, read list.Outcome, req list.Request, run *uicli.Run) (list.Outcome, error) {
	p := &picker{app: c.app, read: read, caller: req.Caller, withPRs: req.WithPRs}
	if err := form.Walk(ctx, p.script(), run.Streams.In, run.Streams.Err); err != nil {
		return p.read, err
	}

	return p.chosen(), nil
}

// A picker is the walk of the picker: the read shown and whether it holds the pull requests, changed on the event loop only, where the caller stands, which every read again keeps, and the worktree and the action picked so far.
type picker struct {
	app      list.UseCase
	read     list.Outcome
	caller   domain.Caller
	withPRs  bool
	worktree string
	picked   list.Detail     // the worktree picked, as it was read then
	action   domain.FollowUp // the action picked of those it offers
}

// script walks the worktree, then the action; the pull requests land later unless the request asked for them.
func (p *picker) script() form.Script {
	script := form.Script{Steps: []form.Step{&worktreeStep{p: p}, &actionStep{p: p}}, Observe: p.observe, Refresh: p.refresh, Label: func(path string) string { return path }}
	if !p.withPRs {
		script.Later = p.later
	}
	return script
}

// chosen is the read shown, with the action picked on the worktree picked as its follow-up.
func (p *picker) chosen() list.Outcome {
	chosen := p.read
	chosen.FollowUps = []domain.FollowUp{p.action}
	return chosen
}

// observe keeps the read the picker opened on: nothing an answer changes needs reading again.
func (p *picker) observe(context.Context) (func(), error) {
	return func() {}, nil
}

// refresh reads every worktree again, with its pull request, once origin is fetched.
func (p *picker) refresh(ctx context.Context) (func(), error) {
	read, err := p.app.Run(ctx, list.Request{WithPRs: true, Fetch: true, Caller: p.caller}, domain.RunOptions{Surface: domain.SurfaceTUI})
	return func() {
		if err == nil {
			p.read, p.withPRs = read, true
		}
	}, err
}

// later reads the pull requests and gives each worktree shown its own, unless a refresh brought them first: the read shown keeps what a refresh read since.
func (p *picker) later(ctx context.Context) (func(), error) {
	read, err := p.app.Run(ctx, list.Request{WithPRs: true, Caller: p.caller}, domain.RunOptions{Surface: domain.SurfaceTUI})
	return func() {
		if err == nil && !p.withPRs {
			p.read = list.WithPullRequests(p.read, read)
		}
		p.withPRs = true
	}, err
}

// detail is the worktree picked.
func (p *picker) detail() *list.Detail {
	at := slices.IndexFunc(p.read.Items, func(item list.Item) bool { return item.Subject == p.worktree })
	if at < 0 {
		return nil
	}
	return p.read.Items[at].Detail
}

// ----------------------------------------------------------------------------
// The worktree

// worktreeStep asks the worktree, each one read that offers an action badged as its row of the list, the failures and warnings of the read under its title.
type worktreeStep struct {
	p    *picker
	list *widget.SelectList
}

func (s *worktreeStep) Label() string { return worktreeLabel }

// Description is the step's help, then each worktree that could not be read and why some of what the list shows is missing.
func (s *worktreeStep) Description() string {
	lines := []string{worktreeHelp}
	if !s.p.withPRs {
		lines[0] += output.Dot + loadingPRs
	}
	for _, item := range s.p.read.Items {
		if item.Err != nil {
			lines = append(lines, output.GlyphFailure+" "+item.Err.Error())
		}
	}
	for _, warning := range s.p.read.Warnings {
		lines = append(lines, output.GlyphAttention+" "+warning.Message)
	}
	return strings.Join(lines, "\n")
}

func (s *worktreeStep) Standing() form.Standing { return form.Standing{Value: s.p.worktree} }

func (s *worktreeStep) Open() tea.Cmd {
	s.list = s.build()
	return nil
}

func (s *worktreeStep) build() *widget.SelectList {
	plain := output.StylesOf(io.Discard)
	var options []widget.Option
	for _, item := range s.p.read.Items {
		if item.Detail != nil && len(item.Detail.Offered()) > 0 {
			options = append(options, widget.Option{Label: shown(*item.Detail, s.p.caller.Cwd), Value: item.Subject, Badge: badge(cellsOf(*item.Detail, plain))})
		}
	}
	return widget.NewSelectList(widget.SelectParams{Options: options, Start: s.p.worktree})
}

func (s *worktreeStep) Widget() widget.Widget { return s.list }

func (s *worktreeStep) Answer() string {
	s.p.worktree = s.list.Value()
	if detail := s.p.detail(); detail != nil {
		s.p.picked = *detail
	}
	return s.p.worktree
}

func (s *worktreeStep) Reset() { s.p.worktree = "" }

// Refreshable reports whether the list can be rebuilt now: not while the user types a filter.
func (s *worktreeStep) Refreshable() bool { return !s.list.Filtering() }

// Refreshed rebuilds the list from the worktrees read again, keeping the one selected.
func (s *worktreeStep) Refreshed() {
	kept := s.list.Value()
	s.list = s.build()
	s.list.StartOn(kept)
}

// badge is a row's cells after its subject, those it has, as one badge.
func badge(cells []string) string {
	return strings.Join(slices.DeleteFunc(cells, func(cell string) bool { return cell == "" }), "  ")
}

// ----------------------------------------------------------------------------
// The action

// actionStep asks what to do with the worktree picked, among what it offers.
type actionStep struct {
	p    *picker
	list *widget.SelectList
}

func (s *actionStep) Label() string { return actionLabel }

func (s *actionStep) Description() string { return "What to do with " + s.p.worktree }

func (s *actionStep) Standing() form.Standing { return form.Standing{Value: s.p.action.Message} }

func (s *actionStep) Open() tea.Cmd {
	var options []widget.Option
	for _, action := range s.p.picked.Offered() {
		options = append(options, widget.Option{Label: action.Message, Value: action.Code})
	}
	s.list = widget.NewSelectList(widget.SelectParams{Options: options})
	return nil
}

func (s *actionStep) Widget() widget.Widget { return s.list }

func (s *actionStep) Answer() string {
	offered := s.p.picked.Offered()
	at := slices.IndexFunc(offered, func(action domain.FollowUp) bool { return action.Code == s.list.Value() })
	if at >= 0 {
		s.p.action = offered[at]
	}
	return s.p.action.Message
}

func (s *actionStep) Reset() { s.p.action = domain.FollowUp{} }
