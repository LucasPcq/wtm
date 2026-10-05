// Package status runs the `wtm status` flow: a worktree's whole state, read
// without numbering, waking or writing anything. The only question it asks is
// which worktree, and only a fully interactive run asks it.
package status

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	envflow "github.com/LucasPcq/wtm/internal/flow/env"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/run/urls"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// Worktree is the positional as it was typed; Cwd is the worktree the
	// picker opens on, and the answer when nobody is asked.
	Worktree string
	Cwd      string
}

type Params struct {
	Context flow.Context
	Request Request
	// Prompter asks which worktree only in a fully interactive run; Unattended
	// takes the current one, which keeps every other path question-free.
	Prompter  flow.Prompter
	Presenter flow.Presenter
	// Jobs is what is up, machine-wide; nil reads it without waking the daemon.
	Jobs func() []domain.JobInfo
}

type Outcome struct {
	Document domain.StatusDocument
	Aborted  bool
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	named, err := target.Named(ctx, target.ResolveParams{ProjectDir: params.Context.ProjectDir, Query: params.Request.Worktree})
	if err != nil {
		return Outcome{}, err
	}
	answers, err := params.Prompter.Ask(session(ctx, sessionParams{Params: params, Named: named}))
	if errors.Is(err, domain.ErrUserAborted) {
		params.Presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	workDir := target.WorkDir(ctx, target.WorkDirParams{Answers: answers, Named: named, Cwd: params.Request.Cwd})
	branch := target.NamedBranch(ctx, target.NamedBranchParams{Named: namedList(named), Dir: workDir})
	if branch == "" {
		return Outcome{}, fmt.Errorf("%w: %s", domain.ErrStatusDetached, workDir)
	}
	var doc domain.StatusDocument
	err = params.Presenter.Stage(ctx, flow.StageParams{
		Message: domain.StatusLoading,
		Work: func(ctx context.Context) error {
			identity, err := worktree.Identity(ctx, refOf(params.Context, branch))
			if err != nil {
				return err
			}
			r, err := open(ctx, params)
			if err != nil {
				return err
			}
			doc, err = r.document(identity)
			return err
		},
	})
	return Outcome{Document: doc}, err
}

type sessionParams struct {
	Params
	Named *target.Resolved
}

// session is the run module's own worktree question, opened on the current
// worktree: a positional answers it, and so does the current worktree when
// nobody can be asked.
func session(ctx context.Context, params sessionParams) flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdStatus,
		Presets:  target.Presets(target.PresetParams{Named: params.Named}),
		Steps: []flow.Step{target.WorktreeStep(ctx, target.WorktreeParams{
			ProjectDir: params.Context.ProjectDir,
			Current:    params.Request.Cwd,
		})},
	}
}

// RunAll is every worktree of the repository a branch names, main first as
// git lists it; the jobs are read once for all of them. It asks nothing.
func RunAll(ctx context.Context, params Params) ([]domain.StatusDocument, error) {
	var docs []domain.StatusDocument
	err := params.Presenter.Stage(ctx, flow.StageParams{
		Message: domain.StatusLoading,
		Work: func(ctx context.Context) error {
			var err error
			docs, err = readAll(ctx, params)
			return err
		},
	})
	return docs, err
}

func readAll(ctx context.Context, params Params) ([]domain.StatusDocument, error) {
	identities, err := worktree.Identities(ctx, worktree.IdentitiesParams{ProjectDir: params.Context.ProjectDir, StateDir: params.Context.StateDir})
	if err != nil {
		return nil, err
	}
	r, err := open(ctx, params)
	if err != nil {
		return nil, err
	}
	docs := make([]domain.StatusDocument, 0, len(identities))
	for _, identity := range identities {
		doc, err := r.document(identity)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

type reader struct {
	runCtx context.Context
	ctx    flow.Context
	run    domain.RunConfig
	jobs   []domain.JobInfo
}

func open(ctx context.Context, params Params) (reader, error) {
	run, err := runconfig.Load(params.Context.StateDir)
	if err != nil {
		return reader{}, err
	}
	return reader{runCtx: ctx, ctx: params.Context, run: run, jobs: jobsOf(params)}, nil
}

func refOf(ctx flow.Context, branch string) worktree.WorktreeRef {
	return worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch}
}

func (r reader) document(identity domain.WorktreeIdentity) (domain.StatusDocument, error) {
	ref := refOf(r.ctx, identity.Branch)
	read := readRun(r.runCtx, readRunParams{Context: r.ctx, Ref: ref, Run: r.run})
	jobs := rules.StatusJobs(rules.StatusJobsParams{
		Declared: r.run.Jobs,
		Up:       upJobs(r.runCtx, upJobsParams{Path: identity.Path, Jobs: r.jobs}),
		URLs:     read.urls,
	})
	files := r.ctx.Config.Project.Env.Files
	source := envflow.SourceOf(r.runCtx, envflow.SourceParams{Context: r.ctx, Branch: identity.Branch})
	missing := envsvc.MissingTargets(envsvc.MissingTargetsParams{
		WorktreePath: identity.Path,
		MainPath:     r.ctx.ProjectDir,
		ParentPath:   source.ParentPath,
		Strategy:     source.Strategy,
		Files:        files,
	})
	adoption, err := worktree.IsolationAdoptionFor(r.runCtx, worktree.IsolationAdoptionParams{Ref: ref, WorktreePath: identity.Path})
	if err != nil {
		return domain.StatusDocument{}, err
	}

	return domain.StatusDocument{
		Branch:     identity.Branch,
		Path:       identity.Path,
		Main:       identity.IsMain,
		Isolation:  identity.Isolation,
		Addressing: read.addressing,
		Offset:     read.offset,
		RunConfig:  rules.IsRunInitialized(r.run),
		Env:        domain.StatusEnv{Declared: len(files), Missing: rules.MissingEnvTargets(missing)},
		Jobs:       jobs,
		Problems: rules.StatusProblems(rules.StatusProblemsParams{
			Branch:           identity.Branch,
			MissingEnv:       missing,
			Jobs:             jobs,
			IsolationPending: adoption.Pending,
		}),
	}, nil
}

func namedList(named *target.Resolved) []target.Resolved {
	if named == nil {
		return nil
	}
	return []target.Resolved{*named}
}

func jobsOf(params Params) []domain.JobInfo {
	if params.Jobs == nil {
		return runjobs.Current()
	}
	return params.Jobs()
}

type readRunParams struct {
	Context flow.Context
	Ref     worktree.WorktreeRef
	Run     domain.RunConfig
}

type runRead struct {
	addressing *domain.Addressing
	offset     *int
	urls       map[string]string
}

// readRun leaves the offset and the addresses out for a worktree no run has
// numbered: reading them must not be what numbers it.
func readRun(ctx context.Context, params readRunParams) runRead {
	if !rules.IsRunInitialized(params.Run) {
		return runRead{}
	}
	addressing := rules.EffectiveAddressing(params.Run)
	env, err := worktree.BranchEnv(ctx, params.Ref)
	if err != nil {
		return runRead{addressing: &addressing}
	}
	offset := rules.PortOffsetFromEnv(env)
	entries := urls.Open(urls.Params{Context: params.Context, Config: params.Run}).At(env)
	byJob := make(map[string]string, len(entries))
	for _, entry := range entries {
		byJob[entry.Job] = entry.URL
	}
	return runRead{addressing: &addressing, offset: &offset, urls: byJob}
}

type upJobsParams struct {
	Path string
	Jobs []domain.JobInfo
}

// upJobs names where each shared service runs by asking git once per checkout
// holding one, not once per job.
func upJobs(ctx context.Context, params upJobsParams) []domain.JobSnapshot {
	branches := map[string]string{}
	for _, job := range params.Jobs {
		if job.SharedDir == "" {
			continue
		}
		if _, seen := branches[job.SharedDir]; seen {
			continue
		}
		branches[job.SharedDir] = target.BranchOf(ctx, job.SharedDir)
	}
	return rules.WorktreeJobs(rules.WorktreeJobsParams{
		Path:     params.Path,
		Jobs:     params.Jobs,
		Branches: branches,
	})
}
