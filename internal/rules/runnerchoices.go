package rules

import (
	"github.com/LucasPcq/wtm/internal/domain"
)

type RunnerChoicesParams struct {
	Config domain.RunConfig
	// ComposeJobs never run other jobs and are never run by one: a stack is a
	// process tree of its own, described by its file.
	ComposeJobs []string
}

// RunnerCandidates are the services that could be starting the others: declared
// at the root of the repository, holding no port of their own. It is where a
// `turbo run dev` behind a filter lands, and the only thing wtm reads is the
// directory — never the command.
func RunnerCandidates(params RunnerChoicesParams) []string {
	exempt := namesSet(params.ComposeJobs)

	var names []string
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindService || exempt[job.Name] || runsCompose(job) {
			continue
		}
		if ScriptJobCwd(job.Cwd) == ScriptJobCwd("") && len(job.Ports) == 0 {
			names = append(names, job.Name)
		}
	}
	return names
}

// RunnerChoices is one row per service a root-level one could be starting: the
// apps declared in a subdirectory, and the runners themselves — a `dev` fanning
// out to `dev:shop` and `dev:crm`, each fanning out to its own apps, is the
// shape a turborepo takes, and it cannot be written a row at a time otherwise.
// Pre-filled from the relation run.toml already holds, so a re-init shows what
// was settled rather than asking again.
func RunnerChoices(params RunnerChoicesParams) []domain.JobRunnerChoice {
	runners := RunnerCandidates(params)
	if len(runners) == 0 {
		return nil
	}

	isRunner := namesSet(runners)
	exempt := namesSet(params.ComposeJobs)

	var choices []domain.JobRunnerChoice
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindService || exempt[job.Name] || runsCompose(job) {
			continue
		}
		if ScriptJobCwd(job.Cwd) == ScriptJobCwd("") && !isRunner[job.Name] {
			continue
		}
		options := runnerOptionsFor(runnerOptionsParams{
			Config: params.Config, Job: job.Name, Runners: runners,
		})
		if len(options) == 1 {
			continue
		}
		choices = append(choices, domain.JobRunnerChoice{
			Job:     job.Name,
			Label:   JobLabelWithCwd(job),
			Runners: directRunnersOf(params.Config, job.Name, isRunner),
			Options: options,
		})
	}
	return choices
}

type runnerOptionsParams struct {
	Config  domain.RunConfig
	Job     string
	Runners []string
}

// runnerOptionsFor is what a row may be set to: no runner, or any candidate that
// would not end up running itself. A runner offered as its own parent — directly
// or through the chain it already starts — is a config run.toml refuses to load,
// so the step never proposes it.
func runnerOptionsFor(params runnerOptionsParams) []string {
	reached := namesSet(RunnerChildren(params.Config, params.Job))

	options := []string{""}
	for _, runner := range params.Runners {
		if runner == params.Job || reached[runner] {
			continue
		}
		options = append(options, runner)
	}
	return options
}

// JobLabelWithCwd names a job by where it runs, which is what tells two `dev`
// scripts of a monorepo apart.
func JobLabelWithCwd(job domain.JobConfig) string {
	if job.Cwd == "" || job.Cwd == ScriptJobCwd("") {
		return job.Name
	}
	return job.Name + domain.RunnerListCwdSep + job.Cwd
}

// directRunnersOf names the candidates that start this job themselves. The
// transitive reading belongs to what a runner holds, never to what a row says:
// a grandparent shown as the answer would be written back as a direct relation,
// flattening the chain the reader composed.
func directRunnersOf(cfg domain.RunConfig, job string, candidates map[string]bool) []string {
	var runners []string
	for _, candidate := range cfg.Jobs {
		if !candidates[candidate.Name] {
			continue
		}
		for _, child := range candidate.Runs {
			if child == job {
				runners = append(runners, candidate.Name)
				break
			}
		}
	}
	return runners
}

type ApplyRunnerChoicesParams struct {
	Config  domain.RunConfig
	Choices []domain.JobRunnerChoice
}

// ApplyRunnerChoices writes the relation the step settled. A row set back to
// "none" withdraws it — but only rows the step showed are ever removed: a
// `runs` entry naming a job this step never offered (a task, a compose stack,
// a service at the root) was written elsewhere and is not ours to drop.
func ApplyRunnerChoices(params ApplyRunnerChoicesParams) domain.RunConfig {
	cfg := params.Config
	if len(params.Choices) == 0 {
		return cfg
	}

	offered := map[string]bool{}
	children := map[string][]string{}
	touched := map[string]bool{}
	for _, choice := range params.Choices {
		offered[choice.Job] = true
		for _, option := range choice.Options {
			if option != "" {
				touched[option] = true
			}
		}
		for _, runner := range choice.Runners {
			children[runner] = append(children[runner], choice.Job)
		}
	}

	out := cfg
	out.Jobs = make([]domain.JobConfig, len(cfg.Jobs))
	copy(out.Jobs, cfg.Jobs)
	for i, job := range out.Jobs {
		if !touched[job.Name] {
			continue
		}
		kept := make([]string, 0, len(job.Runs))
		for _, child := range job.Runs {
			if !offered[child] {
				kept = append(kept, child)
			}
		}
		out.Jobs[i].Runs = append(kept, children[job.Name]...)
		if len(out.Jobs[i].Runs) == 0 {
			out.Jobs[i].Runs = nil
		}
	}
	return out
}

func namesSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// AddressingChoices orders the two modes with the one the project is already on
// first: a step opens on the answer that is settled, and asks to move away from
// it rather than back to it.
func AddressingChoices(current domain.Addressing) []domain.Addressing {
	if current == domain.AddressingPorts {
		return []domain.Addressing{domain.AddressingPorts, domain.AddressingNames}
	}
	return []domain.Addressing{domain.AddressingNames, domain.AddressingPorts}
}

// AddressingLabel names one mode with an example of what it writes.
func AddressingLabel(mode domain.Addressing) string {
	if mode == domain.AddressingPorts {
		return domain.AddressingPortsLabel
	}
	return domain.AddressingNamesLabel
}

// AnyJobPublishesAName says whether the addressing mode changes anything at
// all: with no published job, both modes write the same values.
func AnyJobPublishesAName(cfg domain.RunConfig) bool {
	for _, job := range cfg.Jobs {
		if job.URL != nil {
			return true
		}
	}
	return false
}

// ScriptsStepDescription warns about the monorepo trap where it is sprung: a
// reader who checks only the root scripts gets no port and no url proposed for
// the apps those start, and finds out three steps later. Only the workspace a
// script sits in is read — never what its command does.
func ScriptsStepDescription(scripts []domain.PackageScript) string {
	root, nested := false, false
	for _, script := range scripts {
		if script.Workspace == "" {
			root = true
			continue
		}
		nested = true
	}
	if !root || !nested {
		return domain.ScriptsStepDesc
	}
	return domain.ScriptsStepDesc + "\n\n" + domain.MonorepoRootHint
}
