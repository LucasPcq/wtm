// Package runps reads the jobs wtm's daemon runs on the machine, grouped by the worktree they run for, each repository's shared services with the worktrees holding them; a read, it changes nothing and never starts the daemon.
package runps

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "run-ps"

// A UseCase lists the jobs of the machine: the ports it reads them through, and the command that hands a skewed daemon's jobs over to this wtm's.
type UseCase struct {
	Jobs    Jobs
	Open    Open
	ReadLog ReadLog
	Restart *domain.FollowUp
}

// An Outcome is what Run read: one Item per group of jobs, and the warnings of what could not be read.
type Outcome = domain.Read[Detail]

// An Item is one group of jobs.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads the jobs of every repository of the machine, or of the one req names: a read, it takes no lock, runs no operation and never starts the daemon. An error returned is a refusal of the repository named.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the jobs, each repository they run for, and the logs of the jobs of a profile the daemon does not list.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report each group, crashes first.
	items, warnings := List(facts)
	return Outcome{Items: items, Warnings: warnings, Interrupted: facts.Interrupted}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is run ps' form: a read, it asks nothing the repository decides.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what run ps reports: the jobs the daemon knows, then each repository they run for (only the one req names, when it names one), then the logs of the jobs a worktree's profile declares that the daemon does not list there.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	var named *Project
	if req.Repo != "" {
		dir := req.Repo
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(req.Caller.Cwd, dir)
		}
		p, err := u.Open(ctx, dir)
		if err != nil {
			return Facts{}, err
		}
		named = &p
	}

	facts := Facts{Asked: repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, Restart: u.Restart}, func() {}), Unreadable: map[string]error{}, Logged: map[LogKey]*run.Line{}}
	if named != nil {
		facts.Projects = []Project{*named}
		facts.Jobs = slices.DeleteFunc(facts.Jobs, func(job run.Job) bool { return !named.owns(job) })
	} else {
		facts.Projects = u.projects(ctx, facts)
	}

	for key, file := range Unlisted(facts.Projects, facts.Jobs) {
		if lines, _ := u.ReadLog(file, 1); len(lines) > 0 {
			facts.Logged[key] = &lines[len(lines)-1]
		}
	}
	facts.Interrupted = ctx.Err() != nil
	return facts, nil
}

// projects opens each repository the jobs run for, once, sorted by their main checkout; a dir whose repository cannot be read is kept in facts.Unreadable, its jobs grouped in a project of that dir alone.
func (u UseCase) projects(ctx context.Context, facts Facts) []Project {
	var projects []Project
	opened := func(dir string) bool {
		return slices.ContainsFunc(projects, func(p Project) bool {
			return p.MainPath == dir || p.worktreeAt(dir) != nil
		})
	}
	for _, job := range facts.Jobs {
		dir := RepoOf(job)
		if dir == "" {
			dir = job.WorkDir
		}
		if opened(dir) {
			continue
		}
		p, err := u.Open(ctx, dir)
		if err != nil {
			facts.Unreadable[dir] = err
			p = Project{}
			p.MainPath = dir
		}
		projects = append(projects, p)
	}

	slices.SortFunc(projects, func(a, b Project) int { return strings.Compare(a.MainPath, b.MainPath) })
	return projects
}
