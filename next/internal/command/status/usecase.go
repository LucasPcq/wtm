// Package status reads the whole state of a worktree, or of every one: where its branch stands and its uncommitted work, how it runs (its isolation, its ports, its jobs), its .env files, and each problem found with the command that fixes it; a read, it changes nothing.
package status

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "status"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// commitsShown is how many of a worktree's last commits are read.
const commitsShown = 5

// errGone is why a worktree whose directory is gone cannot be read.
var errGone = errors.New("its directory is gone")

// A UseCase reads the state of the worktrees of one repository: the ports it reads them through and the service that reads the repository.
type UseCase struct {
	Git          Git
	Meta         Meta
	EnvFiles     EnvFiles
	Jobs         Jobs
	PullRequests PullRequests
	Repository   repository.Service
}

// An Outcome is what Run read: one Item per worktree, and the warnings of what could not be read.
type Outcome = domain.Read[Detail]

// An Item is one worktree: its Detail, or the error that kept it from being read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads the worktree req names, or the one it stands in, or every worktree with All; a read, it takes no lock, runs no operation and never starts the run daemon. An error returned is a refusal of the request or of the repository; a worktree that cannot be read is a failed item, the others still read.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository and settle the worktrees the request reads, before reading any of them.
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Outcome{}, err
	}
	located, err := u.located(ctx, req)
	if err != nil {
		return Outcome{}, err
	}
	targets, err := Targets(repo.Snapshot, req, located)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Read each of them, the daemon's jobs and, when asked, the pull requests.
	facts := u.Observe(ctx, Observed{Repository: repo, Targets: targets, WithPRs: req.WithPRs})

	// 3. Report each worktree, with the problems found and their fixes.
	items, warnings := Report(facts, targets)
	return Outcome{Items: items, Warnings: warnings}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is status's form over the repository: the worktrees it may read, the one req reads, and its problems.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Questions{}, err
	}

	located, err := u.located(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(repo, req, located), nil
}

// located is the path the worktree req names stands for, resolved through its symbolic links from req.Cwd; "" when it names none, or nothing exists there.
func (u UseCase) located(ctx context.Context, req Request) (string, error) {
	if req.Worktree == "" {
		return "", nil
	}

	path := req.Worktree
	if !filepath.IsAbs(path) && req.Cwd != "" {
		path = filepath.Join(req.Cwd, path)
	}
	location, err := u.Git.Locate(ctx, path)
	if err != nil || !location.Exists {
		return "", err
	}
	return location.Path, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observed is what Observe reads more of: the repository read, the worktrees to read, and whether to ask GitHub for pull requests.
type Observed struct {
	Repository worktree.Repository
	Targets    []worktree.Worktree
	WithPRs    bool
}

// Observe reads what status reports of the repository: the record of each live branch, what each target holds, the jobs the daemon knows and, when asked, the newest pull request of each target's branch. The worktrees, the daemon and GitHub are read at once.
func (u UseCase) Observe(ctx context.Context, observed Observed) Facts {
	facts := Facts{Repository: observed.Repository}
	facts.Records, facts.RecordErrs = u.records(observed.Repository)

	var (
		wg           sync.WaitGroup
		jobs         []run.Job
		pullRequests []worktree.PullRequest
		jobsErr      error
		prsErr       error
	)
	wg.Go(func() { jobs, jobsErr = u.Jobs(ctx) })
	if observed.WithPRs {
		branches := make([]string, 0, len(observed.Targets))
		for _, w := range observed.Targets {
			if branch := w.Branch(); branch != "" {
				branches = append(branches, branch)
			}
		}
		wg.Go(func() { pullRequests, prsErr = u.PullRequests(ctx, branches) })
	}
	read := repository.Each(observed.Targets, func(w worktree.Worktree) State { return u.state(ctx, facts, w) })
	wg.Wait()

	states := make(map[string]State, len(read))
	for i, w := range observed.Targets {
		states[w.Path] = read[i]
	}

	facts.States, facts.Jobs, facts.JobsErr, facts.PullRequests, facts.PRsErr = states, jobs, jobsErr, pullRequests, prsErr
	return facts
}

// records reads the record of each live branch; a branch with none has no entry, one whose record does not read its error.
func (u UseCase) records(repo worktree.Repository) (map[string]worktree.Meta, map[string]error) {
	records, errs := map[string]worktree.Meta{}, map[string]error{}
	for _, branch := range repo.Live() {
		if branch == "" {
			continue
		}
		record, err := u.Meta.Load(branch)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			errs[branch] = err
		default:
			records[branch] = record
		}
	}

	return records, errs
}

// state reads one worktree: its changes, then how much they weigh, its last commits and how its branch stands against the base branch, each of which a branch with no commit yet cannot tell, so it is only said unread; then its .env files. A cancelled read cancels the worktree.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	if w.Prunable {
		return State{Err: errGone}
	}

	var state State
	if state.Changes, state.Err = u.Git.Changes(ctx, w.Path); state.Err != nil {
		return state
	}

	unread := func(what string, err error) bool {
		switch {
		case err == nil:
			return false
		case errors.Is(err, context.Canceled):
			state.Err = err
		default:
			state.Unread = append(state.Unread, what+": "+err.Error())
		}
		return true
	}
	var err error
	state.Diffstat, err = u.Git.Diffstat(ctx, w.Path)
	unread("its diffstat", err)
	state.Commits, err = u.Git.Commits(ctx, w.Path, commitsShown)
	unread("its commits", err)
	if base := facts.Config.Worktrees.BaseBranch; !w.Main && w.Branch() != "" && base != "" && facts.Local(base) {
		gap, err := u.Git.AheadBehindOf(ctx, worktree.Span{Branch: w.Branch(), Base: base})
		if !unread("where it stands against "+base, err) {
			state.Base = &gap
		}
	}
	if state.Err != nil {
		return state
	}

	state.Env, state.Err = u.envFiles(facts, w)
	return state
}

// envFiles reads each declared .env file of w, with its template, the parent worktree's copy and the main worktree's.
func (u UseCase) envFiles(facts Facts, w worktree.Worktree) ([]EnvRead, error) {
	parentPath := ""
	if source := facts.Records[w.Branch()].SourceBranch; source != "" {
		parentPath = facts.CheckedOutAt(source)
	}

	var reads []EnvRead
	for _, file := range facts.Config.Env.Files {
		read := EnvRead{File: file}
		paths := []struct {
			lines *[]dotenv.Line
			dir   string
		}{{&read.Child, w.Path}, {&read.Parent, parentPath}, {&read.Main, facts.MainPath}}
		for _, at := range paths {
			if at.dir == "" {
				continue
			}
			lines, err := u.EnvFiles.Read(filepath.Join(at.dir, file.Target))
			if err != nil {
				return nil, err
			}
			*at.lines = lines
		}

		template, err := u.template(w.Path, file)
		if err != nil {
			return nil, err
		}
		read.Template = template
		reads = append(reads, read)
	}

	return reads, nil
}

// template reads the template of file in the worktree at dir, as the example strategy finds it in a checkout and as `wtm env` reads it; none when it has none.
func (u UseCase) template(dir string, file project.EnvFile) ([]dotenv.Line, error) {
	exists := func(path string) (bool, error) {
		lines, err := u.EnvFiles.Read(path)
		return lines != nil, err
	}
	sources, _, err := env.Sources(env.CopyRequest{Strategy: project.EnvExample, Files: []project.EnvFile{file}, MainPath: dir}, exists)
	if err != nil || len(sources) == 0 {
		return nil, err
	}

	return u.EnvFiles.Read(sources[0].From)
}
