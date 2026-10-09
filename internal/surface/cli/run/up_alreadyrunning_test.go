package run

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// A profile brought up over a service that was already up reports it as such,
// goes on to the rest, and exits zero: the job is where the caller wanted it,
// but nothing was started for it.
func TestRunUpJSONReportsAnAlreadyRunningServiceAsSuch(t *testing.T) {
	setupUpProject(t, &fakeDaemon{Answers: map[string][]process.Response{
		"docker": {{Status: process.StatusError, Message: "job docker " + domain.JobAlreadyRunningSuffix}},
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdUp, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run up: %v", err)
	}

	statuses := map[string]string{}
	for _, result := range decodeRunJobs(t, stdout) {
		statuses[result.Name] = result.Status
	}
	want := map[string]string{
		"docker":  domain.JobActionAlreadyRunning,
		"migrate": domain.JobActionDone,
		"api":     domain.JobActionStarted,
	}
	for name, status := range want {
		if statuses[name] != status {
			t.Errorf("%s status = %q, want %q\n%s", name, statuses[name], status, stdout)
		}
	}
}
