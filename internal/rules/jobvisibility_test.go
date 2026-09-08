package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func job(name string, kind domain.JobKind) domain.JobConfig {
	return domain.JobConfig{Name: name, Kind: kind}
}

// The rule this whole ticket turns on, read on the shape that broke the panel:
// fifteen declared jobs, three of which have anything to do with this worktree.
func TestVisibleJobsKeepsWhatLivesOrLeftATraceAndCountsTheRest(t *testing.T) {
	jobs := []domain.JobConfig{
		job("docker-compose", domain.JobKindService),
		job("dev:crm", domain.JobKindService),
		job("crm-api-db:migrate", domain.JobKindTask),
		job("shop-api-db:seed", domain.JobKindTask),
		job("shop-web-dev", domain.JobKindService),
	}

	visible, hidden := VisibleJobs(VisibleJobsParams{
		Jobs: jobs,
		Up: map[string]domain.JobInfo{
			"docker-compose": {Name: "docker-compose", Status: domain.JobStatusRunning},
			"dev:crm":        {Name: "dev:crm", Status: domain.JobStatusRunning},
		},
		// The task ran and the daemon has already dropped it; only its log is left,
		// which is why the logs view passes Traces and a state panel does not.
		Traces: map[string]bool{"crm-api-db:migrate": true},
	})

	if len(visible) != 3 {
		t.Fatalf("visible = %d jobs, want the two up plus the one that ran", len(visible))
	}
	if hidden != 2 {
		t.Errorf("hidden = %d, want the two never started here", hidden)
	}
	if visible[2].Job.Name != "crm-api-db:migrate" || visible[2].State != JobStateRan {
		t.Errorf("the task reads as %+v, want it shown as having run", visible[2])
	}
	// run.toml's order is what a reader learned the project by.
	if visible[0].Job.Name != "docker-compose" || visible[1].Job.Name != "dev:crm" {
		t.Errorf("order = %q/%q, want run.toml's", visible[0].Job.Name, visible[1].Job.Name)
	}
}

// A task never runs — it executes and exits — so "down" was never true of one.
// Until it has run here it simply is not this worktree's business.
func TestATaskThatNeverRanHereIsNotShownAtAll(t *testing.T) {
	visible, hidden := VisibleJobs(VisibleJobsParams{
		Jobs: []domain.JobConfig{job("crm-api-db:migrate", domain.JobKindTask)},
	})

	if len(visible) != 0 {
		t.Errorf("visible = %+v, want nothing for a task that never ran here", visible)
	}
	if hidden != 1 {
		t.Errorf("hidden = %d, want the declaration counted in the catalogue", hidden)
	}
}

// The index is the live answer. A log left by an earlier run must never
// describe a job the daemon is holding right now.
func TestTheIndexOutranksALogLeftByAnEarlierRun(t *testing.T) {
	visible, _ := VisibleJobs(VisibleJobsParams{
		Jobs:   []domain.JobConfig{job("web", domain.JobKindService)},
		Up:     map[string]domain.JobInfo{"web": {Name: "web", Status: domain.JobStatusRunning}},
		Traces: map[string]bool{"web": true},
	})

	if len(visible) != 1 || visible[0].State != JobStateUp {
		t.Fatalf("visible = %+v, want the running job read off the index", visible)
	}
}

// A crashed job is the case the logs view exists for: it has to stay reachable,
// and it has to be distinguishable from one that was stopped on purpose.
func TestACrashedJobKeepsItsRowAndItsOwnGlyph(t *testing.T) {
	visible, _ := VisibleJobs(VisibleJobsParams{
		Jobs: []domain.JobConfig{job("api", domain.JobKindService), job("web", domain.JobKindService)},
		Up: map[string]domain.JobInfo{
			"api": {Name: "api", Status: domain.JobStatusCrashed},
			"web": {Name: "web", Status: domain.JobStatusStopped},
		},
	})

	if len(visible) != 2 {
		t.Fatalf("visible = %d, want both the crashed and the stopped job", len(visible))
	}
	if visible[0].State != JobStateFailed {
		t.Errorf("api reads as %q, want failed", visible[0].State)
	}
	if visible[1].State != JobStateStopped {
		t.Errorf("web reads as %q, want stopped", visible[1].State)
	}
	if JobStateGlyph(visible[0].State) == JobStateGlyph(visible[1].State) {
		t.Error("a crash and a deliberate stop wear the same glyph")
	}
}

func TestCountUpCountsOnlyWhatIsActuallyUp(t *testing.T) {
	visible, _ := VisibleJobs(VisibleJobsParams{
		Jobs: []domain.JobConfig{job("api", domain.JobKindService), job("web", domain.JobKindService)},
		Up: map[string]domain.JobInfo{
			"api": {Name: "api", Status: domain.JobStatusRunning},
			"web": {Name: "web", Status: domain.JobStatusStopped},
		},
	})

	if up := CountUp(visible); up != 1 {
		t.Errorf("up = %d, want only the running job counted", up)
	}
}
