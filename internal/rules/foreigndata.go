package rules

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ForeignDataParams struct {
	Config domain.RunConfig
	// Jobs are the ones this run starts in WorkDir.
	Jobs      []domain.JobConfig
	WorkDir   string
	Isolation domain.Isolation
	// Main is the main checkout, which owns its data by definition: every
	// other worktree is copied from it, or carved beside it.
	Main bool
}

// ForeignDataRisks are the jobs about to start whose `touches` reach data
// this worktree does not own. Only what the config declares is read: a
// command is never guessed at.
func ForeignDataRisks(params ForeignDataParams) []domain.DataRisk {
	if params.Main {
		return nil
	}
	declared := make(map[string]domain.JobConfig, len(params.Config.Jobs))
	for _, job := range params.Config.Jobs {
		declared[job.Name] = job
	}

	var risks []domain.DataRisk
	for _, started := range touchersOf(params.Config, params.Jobs) {
		for _, name := range started.Job.Touches {
			owner, foreign := foreignOwner(foreignOwnerParams{Service: declared[name], Isolation: params.Isolation})
			if !foreign {
				continue
			}
			risks = append(risks, domain.DataRisk{Job: started.Job.Name, Service: name, Owner: owner, WorkDir: params.WorkDir, Via: started.Via})
		}
	}
	return risks
}

type startedJob struct {
	Job domain.JobConfig
	Via string
}

// touchersOf is every job a run starts: those it names, and those their runners
// start as their `runs` — a migration a dev runner launches touches the same
// database as one started by hand.
func touchersOf(cfg domain.RunConfig, jobs []domain.JobConfig) []startedJob {
	declared := jobsByName(cfg)
	seen := map[string]bool{}
	var started []startedJob
	for _, job := range jobs {
		if !seen[job.Name] {
			seen[job.Name] = true
			started = append(started, startedJob{Job: job})
		}
	}
	for _, job := range jobs {
		for _, child := range RunnerChildren(cfg, job.Name) {
			if seen[child] {
				continue
			}
			seen[child] = true
			started = append(started, startedJob{Job: declared[child], Via: job.Name})
		}
	}
	return started
}

// DeclaresTouches says whether anything a run starts — itself or through a
// runner — declares `touches`, which is all the guard ever reads.
func DeclaresTouches(cfg domain.RunConfig, jobs []domain.JobConfig) bool {
	for _, started := range touchersOf(cfg, jobs) {
		if len(started.Job.Touches) > 0 {
			return true
		}
	}
	return false
}

// ForeignDataHints name the way out for each cause present: a verbatim worktree
// gets its own data from its isolation, a shared service only from a namespace.
func ForeignDataHints(risks []domain.DataRisk) []string {
	var hints []string
	source := false
	services := map[string]bool{}
	var unsliced []string
	for _, risk := range risks {
		if risk.Owner == domain.DataOwnerSource {
			source = true
			continue
		}
		if !services[risk.Service] {
			services[risk.Service] = true
			unsliced = append(unsliced, risk.Service)
		}
	}
	if source {
		hints = append(hints, fmt.Sprintf(domain.RunForeignDataIsolateHintFmt, domain.FlagIsolation, domain.IsolationIsolated))
	}
	for _, service := range unsliced {
		hints = append(hints, fmt.Sprintf(domain.RunForeignDataNamespaceHintFmt, service))
	}
	return hints
}

type foreignOwnerParams struct {
	Service   domain.JobConfig
	Isolation domain.Isolation
}

func foreignOwner(params foreignOwnerParams) (domain.DataOwner, bool) {
	if IsVerbatim(params.Isolation) {
		return domain.DataOwnerSource, true
	}
	if IsShared(params.Service) && !HasNamespace(params.Service) {
		return domain.DataOwnerEveryone, true
	}
	return "", false
}

type ForeignDataLinesParams struct {
	Risks []domain.DataRisk
	// Several says the run spans worktrees, so each line names its own.
	Several bool
}

func ForeignDataLines(params ForeignDataLinesParams) []string {
	lines := make([]string, 0, len(params.Risks))
	for _, risk := range params.Risks {
		owner := domain.RunForeignDataOwnerShared
		if risk.Owner == domain.DataOwnerSource {
			owner = domain.RunForeignDataOwnerSource
		}
		job := risk.Job
		if risk.Via != "" {
			job = fmt.Sprintf(domain.RunForeignDataViaFmt, risk.Job, risk.Via)
		}
		if params.Several {
			job = fmt.Sprintf(domain.RunForeignDataInFmt, risk.Job, filepath.Base(risk.WorkDir))
		}
		lines = append(lines, fmt.Sprintf(domain.RunForeignDataLineFmt, job, risk.Service, owner))
	}
	return lines
}
