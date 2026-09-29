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
	for _, job := range params.Jobs {
		for _, name := range job.Touches {
			owner, foreign := foreignOwner(foreignOwnerParams{Service: declared[name], Isolation: params.Isolation})
			if !foreign {
				continue
			}
			risks = append(risks, domain.DataRisk{Job: job.Name, Service: name, Owner: owner, WorkDir: params.WorkDir})
		}
	}
	return risks
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
		if params.Several {
			job = fmt.Sprintf(domain.RunForeignDataInFmt, risk.Job, filepath.Base(risk.WorkDir))
		}
		lines = append(lines, fmt.Sprintf(domain.RunForeignDataLineFmt, job, risk.Service, owner))
	}
	return lines
}
