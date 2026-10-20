package status_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestProblemsComeEnvThenIsolationThenCrashedJobs(t *testing.T) {
	signal, exit := -1, 3
	problems := status.Problems(status.ProblemsParams{
		Subject: "feat/a",
		Missing: []status.Missing{{Target: ".env"}},
		Pending: true,
		Jobs: []status.Job{
			{Name: "web", State: run.Running},
			{Name: "api", State: run.Crashed, ExitCode: &exit},
			{Name: "worker", State: run.Crashed, ExitCode: &signal},
			{Name: "cron", State: run.Crashed},
		},
	})

	var messages []string
	for _, problem := range problems {
		messages = append(messages, problem.Message)
	}
	assert.Equal(t, []string{
		".env is missing",
		"predates the isolation choice: run up and run start refuse it",
		"api crashed (exit 3)",
		"worker crashed (killed by a signal)",
		"cron crashed",
	}, messages)
}

func TestNothingToFixIsNoProblemNotNull(t *testing.T) {
	assert.Equal(t, []status.Problem{}, status.Problems(status.ProblemsParams{Subject: "main"}))
}
