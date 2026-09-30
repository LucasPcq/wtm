package runlogs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// A profile brought up over a job that was already up started nothing for it:
// `started` there claimed an act that never happened, the way `stopped` did for
// a stop that found nothing.
func TestRunReportsAnAlreadyRunningServiceAsSuch(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"docker": "job docker " + domain.JobAlreadyRunningSuffix},
	}

	outcome, _ := run(t, service, docker, api)

	statuses := map[string]string{}
	for _, result := range outcome.Results {
		statuses[result.Name] = result.Status
	}
	if statuses["docker"] != domain.JobActionAlreadyRunning {
		t.Errorf("docker status = %q, want %q", statuses["docker"], domain.JobActionAlreadyRunning)
	}
	if statuses["api"] != domain.JobActionStarted {
		t.Errorf("api status = %q, want %q", statuses["api"], domain.JobActionStarted)
	}
}
