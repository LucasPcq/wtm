package process

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// IndexedJobs is what the index says is up, read with no daemon to ask: each
// entry gets the verdict a daemon starting now would reach, probing the same
// groups and stacks, and nothing is reaped.
func IndexedJobs() []domain.JobInfo {
	return indexedJobs(indexedJobsParams{
		Records: NewStateStore(StatePath()).Load(),
		Orphans: systemOrphans{},
		Stacks:  systemStacks{},
	})
}

type indexedJobsParams struct {
	Records []domain.JobRecord
	Orphans Orphans
	Stacks  Stacks
}

func indexedJobs(params indexedJobsParams) []domain.JobInfo {
	groups := params.Orphans.Probe(orphanQueries(params.Records))
	stacks := params.Stacks.Probe(stackQueriesOf(params.Records))
	jobs := make([]domain.JobInfo, 0, len(params.Records))
	for _, record := range params.Records {
		group := groups[record.PGID]
		stack := stacks[jobKey(record.Name, record.WorkDir)]
		decision := rules.ReconcileJob(rules.ReconcileJobParams{
			Record:            record,
			WorkDirExists:     dirExists(record.WorkDir),
			GroupAlive:        group.Alive,
			IdentityConfirmed: group.IdentityConfirmed,
			StackKnownDown:    stack.Known && !stack.Up,
		})
		if !decision.Adopt {
			continue
		}
		jobs = append(jobs, domain.JobInfo{
			Name:      record.Name,
			Kind:      record.Config.Kind,
			Status:    indexedStatus(decision),
			PID:       record.PID,
			WorkDir:   record.WorkDir,
			StartedAt: record.StartedAt,
			SharedDir: record.SharedDir,
		})
	}
	return jobs
}

// indexedStatus reads a reap as what it is until a daemon starts: a group
// still alive and ours is a job still running, only nobody reads it.
func indexedStatus(decision rules.ReconcileDecision) domain.JobStatus {
	if decision.Reap {
		return domain.JobStatusRunning
	}
	return decision.Status
}
