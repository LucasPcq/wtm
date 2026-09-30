// Package url runs the `wtm run url` flow.
package url

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/run/urls"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Request struct {
	// Worktree is the positional as it was typed; Cwd is what answers for it —
	// this command never asks. Inside curl $(wtm run url --job api) the
	// substitution captures stdout but stdin is still the terminal, so a form
	// would open mid-substitution and hang the shell.
	Worktree string
	Cwd      string
	Job      string
	Raw      bool
	Config   domain.RunConfig
}

// Outcome is every address the worktree publishes, already narrowed to --job
// when one was named. One() is the same set read as the single line a text
// surface prints; which of the two a caller wants follows its format, not this
// flow's business.
type Outcome struct {
	WorkDir string
	Entries []domain.JobURLEntry
}

func (o Outcome) One() (domain.JobURLEntry, error) {
	return rules.PickPublishedURL(o.Entries, "")
}

type Params struct {
	Context flow.Context
	Request Request
}

// Run needs neither Prompter nor Presenter: it asks nothing and shows nothing,
// which is the whole contract of a substitution surface.
func Run(params Params) (Outcome, error) {
	if err := target.RequireDeclared(target.DeclaredParams{Config: params.Request.Config, Job: params.Request.Job}); err != nil {
		return Outcome{}, err
	}
	named, err := target.Named(target.ResolveParams{
		ProjectDir: params.Context.ProjectDir,
		Query:      params.Request.Worktree,
	})
	if err != nil {
		return Outcome{}, err
	}

	workDir := target.WorkDir(target.WorkDirParams{Named: named, Cwd: params.Request.Cwd})
	entries, err := urls.Open(urls.Params{
		Context: params.Context,
		Config:  params.Request.Config,
		Raw:     params.Request.Raw,
	}).In(workDir)
	if err != nil {
		return Outcome{}, err
	}

	if params.Request.Job != "" {
		entry, err := rules.PickPublishedURL(entries, params.Request.Job)
		if err != nil {
			return Outcome{}, err
		}
		entries = []domain.JobURLEntry{entry}
	}
	return Outcome{WorkDir: workDir, Entries: entries}, nil
}
