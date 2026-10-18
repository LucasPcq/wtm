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

// pick asks the user a worktree of read, then what to do with it, and returns read with the command line that does it as its follow-up. The pull requests load once the worktrees are shown, and r reads them again after fetching their branches from origin.
func (c *listCommand) pick(ctx context.Context, read list.Outcome, req list.Request, run *uicli.Run) (list.Outcome, error) {
	p := &picker{app: c.app, req: req, read: read, cwd: run.Env.Cwd, withPRs: req.WithPRs}
	if err := form.Walk(ctx, p.script(), run.Streams.In, run.Streams.Err); err != nil {
		return p.read, err
	}

	return p.chosen(), nil
}

// A picker is the walk of the picker: the read shown and how it was read, and the worktree and the action picked so far.
type picker struct {
	app      list.UseCase
	req      list.Request
	read     list.Outcome
	cwd      string
	withPRs  bool // the read shown holds the pull requests
	worktree string
	action   list.Action
}

// script walks the worktree, then the action; the pull requests land later unless the request asked for them.
func (p *picker) script() form.Script {
	script := form.Script{Steps: []form.Step{&worktreeStep{p: p}, &actionStep{p: p}}, Observe: p.observe, Refresh: p.refresh, Label: func(path string) string { return path }}
	if !p.withPRs {
		script.Later = p.later
	}
	return script
}

// chosen is the read shown, with the command line of the action picked on the worktree picked as its follow-up.
func (p *picker) chosen() list.Outcome {
	chosen := p.read
	chosen.FollowUps = []domain.FollowUp{p.app.FollowUp(*p.detail(), p.action)}
	return chosen
}

// observe keeps the read the picker opened on: nothing an answer changes needs reading again.
func (p *picker) observe(context.Context) (func(), error) {
	return func() {}, nil
}

// refresh reads every worktree again once their branches are fetched from origin.
func (p *picker) refresh(ctx context.Context) (func(), error) {
	req := p.req
	req.WithPRs, req.Fetch = p.withPRs, true
	read, err := p.app.Run(ctx, req, domain.RunOptions{Surface: uicli.SurfaceTUI})
	return func() {
		if err == nil {
			p.read = read
		}
	}, err
}

// later reads every worktree again with its pull request.
func (p *picker) later(ctx context.Context) (func(), error) {
	req := p.req
	req.WithPRs = true
	read, err := p.app.Run(ctx, req, domain.RunOptions{Surface: uicli.SurfaceTUI})
	return func() {
		p.withPRs = true
		if err == nil {
			p.read = read
		}
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

// worktreeStep asks the worktree, each one read badged as its row of the list, the warnings of the read under its title.
type worktreeStep struct {
	p    *picker
	list *widget.SelectList
}

func (s *worktreeStep) Label() string { return worktreeLabel }

// Description is the step's help, then why some of what the list shows is missing.
func (s *worktreeStep) Description() string {
	lines := []string{worktreeHelp}
	if !s.p.withPRs {
		lines[0] += output.Dot + loadingPRs
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
	active := list.Holding(s.p.read.Items, s.p.cwd)
	var options []widget.Option
	for _, item := range s.p.read.Items {
		if item.Detail != nil {
			options = append(options, widget.Option{Label: shown(item, s.p.cwd), Value: item.Subject, Badge: badge(cellsOf(*item.Detail, item.Subject == active, plain))})
		}
	}
	return widget.NewSelectList(widget.SelectParams{Options: options, Start: s.p.worktree})
}

func (s *worktreeStep) Widget() widget.Widget { return s.list }

func (s *worktreeStep) Answer() string {
	s.p.worktree = s.list.Value()
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

func (s *actionStep) Standing() form.Standing { return form.Standing{Value: list.Labels[s.p.action]} }

func (s *actionStep) Open() tea.Cmd {
	var options []widget.Option
	if detail := s.p.detail(); detail != nil {
		for _, action := range list.Offers(*detail) {
			options = append(options, widget.Option{Label: list.Labels[action], Value: string(action)})
		}
	}
	s.list = widget.NewSelectList(widget.SelectParams{Options: options})
	return nil
}

func (s *actionStep) Widget() widget.Widget { return s.list }

func (s *actionStep) Answer() string {
	s.p.action = list.Action(s.list.Value())
	return list.Labels[s.p.action]
}

func (s *actionStep) Reset() { s.p.action = "" }
