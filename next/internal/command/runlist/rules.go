package runlist

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run list reads: the repository and its run.toml, what the daemon was asked, the worktree read, the meta record of each worktree by branch, and the jobs of run.toml the daemon does not list there whose log holds a line.
type Facts struct {
	worktree.Repository
	repository.Asked
	Target     worktree.Worktree
	Records    map[string]worktree.Meta
	RecordErrs map[string]error     // by branch: a record that exists and does not read
	Logged     map[string]*run.Line // by job: the last line a job the daemon lists nothing of left in its log
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is the worktree read as the JSON envelope reports it: its branch (none on no branch) and path, the profile its jobs were started under (none when it runs none, or none started under one), then the profiles of run.toml and its jobs, each in run.toml's order, with what can be done with it.
type Detail struct {
	Branch   string    `json:"branch"`
	Path     string    `json:"path"`
	Profile  string    `json:"profile"`
	Profiles []Profile `json:"profiles"`
	Jobs     []Job     `json:"jobs"`
}

// A Profile is one profile of run.toml: its name, whether a start with no profile named starts it (Default), its jobs, whether the worktree runs it, and what can be done with it: start it, stop its jobs.
type Profile struct {
	Name    string            `json:"name"`
	Default bool              `json:"default"`
	Jobs    []string          `json:"jobs"`
	Running bool              `json:"running"`
	Actions []domain.FollowUp `json:"actions"`
}

// A Job is one job of run.toml as the worktree runs it: its name and kind, its state there (stopped once it ran, not_started when it never did, null when the daemon cannot tell), what of it the daemon holds, the folder it runs in (run.toml's cwd) and its command, the ports it binds there (none when they cannot be told), its url while it is up, whether it runs once for every worktree (Shared), the jobs it starts itself, and what can be done with it: start it, stop it, read its output.
type Job struct {
	Name    string            `json:"name"`
	Kind    project.JobKind   `json:"kind"`
	State   *run.State        `json:"state"`
	Hold    run.Hold          `json:"hold,omitempty"`
	Dir     string            `json:"dir,omitempty"`
	Command string            `json:"command"`
	Ports   map[string]int    `json:"ports,omitempty"`
	URL     string            `json:"url,omitempty"`
	Shared  bool              `json:"shared,omitempty"`
	Runs    []string          `json:"runs,omitempty"`
	Actions []domain.FollowUp `json:"actions"`
}

// The codes of what can be done with an entry of run.toml: start a profile, stop its jobs; start a job, stop it, read its output.
const (
	CodeUp    = Name + ".up"
	CodeDown  = Name + ".down"
	CodeStart = Name + ".start"
	CodeStop  = Name + ".stop"
	CodeLogs  = Name + ".logs"
)

// CodeMetaUnreadable warns of a worktree whose meta record does not read: the ports its jobs bind are not known.
const CodeMetaUnreadable = Name + ".meta_unreadable"

// The paths the commands an entry's actions follow up with answer a profile and a job by.
const (
	pathProfile = "profile"
	pathJob     = "job"
)

// ----------------------------------------------------------------------------
// List

// List reports the worktree read: each profile and each job of run.toml, its state there, its ports at the worktree's offset above the main worktree's, and what can be done with it.
func List(f Facts) (domain.Unit[Detail], []domain.Warning) {
	name := f.Target.Name()
	d := Detail{Branch: f.Target.Branch(), Path: f.Target.Path, Profile: run.ProfileIn(f.Jobs, f.Target.Path), Profiles: []Profile{}, Jobs: []Job{}}
	worktreeIs := domain.Answered{Path: domain.PathWorktree, Values: []string{name}}

	for _, p := range f.Run.Profiles {
		profileIs := domain.Answered{Path: pathProfile, Values: []string{p.Name}}
		d.Profiles = append(d.Profiles, Profile{Name: p.Name, Default: p.Default, Jobs: p.Jobs, Running: p.Name == d.Profile, Actions: []domain.FollowUp{
			{Command: domain.CommandRunUp, Code: CodeUp, Message: "start the profile", Answers: []domain.Answered{worktreeIs, profileIs}},
			{Command: domain.CommandRunDown, Code: CodeDown, Message: "stop its jobs", Answers: []domain.Answered{worktreeIs, profileIs}},
		}})
	}

	offset := env.OffsetOf(env.OffsetParams{Worktree: f.Target, Worktrees: f.Worktrees, Records: f.Records, Run: f.Run})
	for _, job := range f.Run.Jobs {
		jobIs := domain.Answered{Path: pathJob, Values: []string{job.Name}}
		found := Job{Name: job.Name, Kind: job.Kind, Dir: job.Cwd, Command: job.Cmd, Ports: env.PortsAt(job, offset), Shared: job.Scope == project.Shared, Runs: job.Runs, Actions: []domain.FollowUp{
			{Command: domain.CommandRunStart, Code: CodeStart, Message: "start the job", Answers: []domain.Answered{worktreeIs, jobIs}},
			{Command: domain.CommandRunStop, Code: CodeStop, Message: "stop the job", Answers: []domain.Answered{worktreeIs, jobIs}},
			{Command: domain.CommandRunLogs, Code: CodeLogs, Message: "read its output", Answers: []domain.Answered{worktreeIs, jobIs}},
		}}
		found.State, found.Hold, found.URL = f.standing(job.Name)
		d.Jobs = append(d.Jobs, found)
	}

	item := domain.Unit[Detail]{Subject: name, Status: domain.StatusDone, Detail: &d}
	if err := f.RecordErrs[d.Branch]; err != nil {
		item.Warnings = []domain.Warning{{Code: CodeMetaUnreadable, Message: "couldn't read the record of " + d.Branch + ": the ports its jobs bind are not known: " + err.Error(), Params: domain.Params{"branch": d.Branch}}}
	}
	return item, f.Warnings(Name)
}

// standing is where the job called name stands in the worktree read, what of it the daemon holds, and its url while it is up: the daemon's state, a claim reading as the shared job it holds, a job a runner starts itself as that runner; else stopped once its log holds a line, not_started when it never ran; nil when the daemon cannot tell.
func (f Facts) standing(name string) (*run.State, run.Hold, string) {
	if f.JobsErr != nil {
		return nil, "", ""
	}
	standing, listed := f.standingIn(name)
	if !listed {
		return new(run.Unlisted(f.Logged[name] != nil)), "", ""
	}
	job := standing.Job
	// a job its runner starts has its runner's state, never its hold nor its address
	if standing.Runner != "" {
		return &job.State, "", ""
	}

	state := job.State
	if job.Hold == run.HoldClaim {
		state = run.Running
	}
	url := ""
	if job.Up {
		url = job.URL
	}
	return &state, job.Hold, url
}

// standingIn is what the daemon lists of the job called name in the worktree read, or of the runner there that starts it.
func (f Facts) standingIn(name string) (run.Standing, bool) {
	return run.StandingIn(run.StandingParams{Jobs: f.Jobs, Dir: f.Target.Path, Name: name, Declared: f.Run})
}
