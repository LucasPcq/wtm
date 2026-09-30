// Package open runs the `wtm run open` flow.
package open

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/run/urls"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
)

type Request struct {
	// Worktree is the positional as it was typed; Cwd is what answers for it
	// when nobody is asked.
	Worktree string
	Cwd      string
	Job      string
	// Raw opens the job's own port. The .env answers that one whatever the
	// addressing says, which is why it also silences the drift warning.
	Raw    bool
	Config domain.RunConfig
}

type Outcome struct {
	WorkDir string
	Entry   domain.JobURLEntry
	Aborted bool
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter flow.Presenter
	// Open hands the resolved address to whatever this surface opens URLs with.
	// It is a surface capability rather than a service the flow reaches for: the
	// desktop's own opener is one answer to it, a dashboard pane another.
	Open func(url string) error
}

func Run(params Params) (Outcome, error) {
	if err := target.RequireDeclared(target.DeclaredParams{Config: params.Request.Config, Job: params.Request.Job}); err != nil {
		return Outcome{}, err
	}
	f := &openFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		open:      params.Open,
		reader: urls.Open(urls.Params{
			Context: params.Context,
			Config:  params.Request.Config,
			Raw:     params.Request.Raw,
		}),
	}
	return f.run()
}

type openFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter flow.Presenter
	open      func(url string) error
	reader    urls.Reader

	named *target.Resolved
}

func (f *openFlow) run() (Outcome, error) {
	named, err := target.Named(target.ResolveParams{ProjectDir: f.ctx.ProjectDir, Query: f.request.Worktree})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	workDir := target.WorkDir(target.WorkDirParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
	published, err := f.reader.In(workDir)
	if err != nil {
		return Outcome{}, err
	}
	entry, err := rules.PickPublishedURL(published, answers.Value(target.KeyJob))
	if err != nil {
		return Outcome{}, err
	}

	// The warning says the .env answers on a port where a name is published. It
	// is meaningless under --raw, which asked for the port, and equally so with
	// no proxy: there is no name for the .env to disagree with.
	if !f.request.Raw && f.reader.Serving() {
		if notice, drifting := addressing.Notice(addressing.Params{Context: f.ctx, WorkDirs: []string{workDir}}); drifting {
			f.presenter.Status(notice)
		}
	}
	if err := f.open(entry.URL); err != nil {
		return Outcome{}, err
	}
	return Outcome{WorkDir: workDir, Entry: entry}, nil
}

// published feeds the picker. A worktree whose addresses cannot be resolved
// offers none, and the run then refuses it by name.
func (f *openFlow) published(workDir string) []domain.JobURLEntry {
	entries, _ := f.reader.In(workDir)
	return entries
}

func (f *openFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdOpen,
		Presets:  target.Presets(target.PresetParams{Named: f.named, Job: f.request.Job}),
		Steps: []flow.Step{
			target.WorktreeStep(target.WorktreeParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				// What each worktree already has up, which is half of deciding
				// which one to open. A daemon that cannot answer leaves the
				// badges off rather than refusing the run.
				Running: rules.RunningJobsByWorktree(runjobs.Load()),
			}),
			target.URLStep(target.URLParams{
				Published: f.published,
				Named:     f.named,
				Cwd:       f.request.Cwd,
				Flag:      domain.FlagJob,
			}),
		},
	}
}
