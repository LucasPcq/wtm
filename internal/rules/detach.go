package rules

import (
	"fmt"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Removable narrows a config to the shared jobs that carve out a namespace and
// declare how to give it back: the only ones a removal has anything to ask of.
func Removable(cfg domain.RunConfig) domain.RunConfig {
	var jobs []domain.JobConfig
	for _, job := range cfg.Jobs {
		if IsShared(job) && HasNamespace(job) && !IsBlankCommand(job.Namespace.Remove) {
			jobs = append(jobs, job)
		}
	}
	return domain.RunConfig{Jobs: jobs}
}

type HeldNamespacesParams struct {
	Holdings []domain.NamespaceHolding
	Up       map[string]bool
}

// HeldNamespaces lists what the removal gives back, worktree by worktree in the
// order they were given, then in the config's order.
func HeldNamespaces(params HeldNamespacesParams) []domain.HeldNamespace {
	var held []domain.HeldNamespace
	for _, holding := range params.Holdings {
		for _, job := range holding.Config.Jobs {
			held = append(held, domain.HeldNamespace{
				Name:       NamespaceName(NamespaceNameParams{Config: holding.Config, Ref: HoldingRef(holding, job.Name)}),
				Job:        job.Name,
				Up:         params.Up[job.Name],
				SharedWith: holding.SharedWith,
			})
		}
	}
	return held
}

// HoldingRef is the slice of job a holding names, recomputed from the
// worktree's own environment the way its attach computed it.
func HoldingRef(holding domain.NamespaceHolding, job string) domain.NamespaceRef {
	slug := holding.Env[domain.EnvWorktree]
	if slug == "" {
		slug = WorktreeSlug(holding.Branch)
	}
	ordinal, _ := strconv.Atoi(holding.Env[domain.EnvOrdinal])
	return domain.NamespaceRef{Job: job, Worktree: slug, Ordinal: ordinal}
}

// DownServices are the services holding data that cannot take it back now, each
// once, in the order the data names them.
func DownServices(held []domain.HeldNamespace) []string {
	var jobs []string
	seen := map[string]bool{}
	for _, namespace := range held {
		if namespace.Up || namespace.SharedWith != "" || seen[namespace.Job] {
			continue
		}
		seen[namespace.Job] = true
		jobs = append(jobs, namespace.Job)
	}
	return jobs
}

type DataRecapLinesParams struct {
	Held []domain.HeldNamespace
	// StartDown is the answer to the data step: start the services that are down
	// to drop what they hold, or leave it owed until they next start.
	StartDown bool
	KeepData  bool
}

// DataRecapLines says what a removal does to each namespace it holds. A recap
// that promised a DROP DATABASE to a service that is down described a removal
// that was not going to happen.
func DataRecapLines(params DataRecapLinesParams) []string {
	if len(params.Held) == 0 {
		return nil
	}
	if params.KeepData {
		return []string{domain.CleanKeepDataLine}
	}
	lines := make([]string, 0, len(params.Held))
	for _, namespace := range params.Held {
		switch {
		case namespace.SharedWith != "":
			lines = append(lines, fmt.Sprintf(domain.CleanDataSharedRecapFmt, namespace.Name, namespace.SharedWith))
		case namespace.Up:
			lines = append(lines, fmt.Sprintf(domain.CleanWillDeleteNamespaceFmt, namespace.Name, namespace.Job))
		case params.StartDown:
			lines = append(lines, fmt.Sprintf(domain.DataRecapStartFmt, namespace.Name, namespace.Job, namespace.Job))
		default:
			lines = append(lines, fmt.Sprintf(domain.DataRecapDeferFmt, namespace.Name, namespace.Job))
		}
	}
	return lines
}
