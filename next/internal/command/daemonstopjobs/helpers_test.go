package daemonstopjobs_test

import (
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// usecase is stop-jobs over d, under ops, a path under /src/acme in acme, any other in no repository.
func usecase(d *daemon, ops operation.Service) daemonstopjobs.UseCase {
	return daemonstopjobs.UseCase{Operations: ops, Daemon: d, RepoRoot: func(_ context.Context, path string) (string, error) {
		if path == "/src/acme" || path == "/src/acme/apps" {
			return "/src/acme", nil
		}
		return "", errors.New("not a git repository")
	}}
}

// operations is an operations service over in-memory locks and events.
func operations() operation.Service {
	locks := &operationtest.Locks{}
	return operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}
}

// worktrees are the worktrees of a plan, each with the words of its jobs, as name: jobs.
func worktrees(plan daemonstopjobs.Plan) map[string][]string {
	got := map[string][]string{}
	for _, repo := range plan.Repos {
		for _, t := range repo.Worktrees {
			for _, job := range t.Jobs {
				got[t.Worktree] = append(got[t.Worktree], job.Name+":"+string(job.Fate)+held(job))
			}
		}
	}
	return got
}

func held(job runjobs.StopJob) string {
	if job.Shared() {
		return ":held"
	}
	return ""
}
