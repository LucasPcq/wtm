package status_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestProblemsComeMergedThenEnvThenIsolationThenCrashedJobs(t *testing.T) {
	signal, exit := -1, 3
	problems := status.Problems(status.ProblemsParams{
		Branch:  "feat/a",
		Merged:  true,
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
		"its pull request is merged",
		".env is missing",
		"predates the isolation choice: run up and run start refuse it",
		"api crashed (exit 3)",
		"worker crashed (killed by a signal)",
		"cron crashed",
	}, messages)
}

func TestABranchBehindOriginIsFastForwardedBeforeItIsSynced(t *testing.T) {
	problems := status.Problems(status.ProblemsParams{Branch: "feat/a", Merged: true, BehindOrigin: 2, Parent: "main", Lacks: 1, Sync: true})

	var fixes []string
	for _, problem := range problems {
		fixes = append(fixes, problem.Code+" "+problem.Fix.Command)
	}
	assert.Equal(t, []string{"status.merged prune", "status.behind_origin fast-forward", "status.behind sync"}, fixes)
	assert.Equal(t, "origin has 2 commits it lacks", problems[1].Message)
}

func TestNothingToFixIsNoProblemNotNull(t *testing.T) {
	assert.Equal(t, []status.Problem{}, status.Problems(status.ProblemsParams{Branch: "main"}))
}

func TestTheCommitsOfItsParentAreAProblemOnlyOfAWorktreeToSync(t *testing.T) {
	assert.Equal(t, []status.Problem{}, status.Problems(status.ProblemsParams{Branch: "feat/a", Parent: "main", Lacks: 2}))
	assert.Len(t, status.Problems(status.ProblemsParams{Branch: "feat/a", Parent: "main", Lacks: 2, Sync: true}), 1)
}
