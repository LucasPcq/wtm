package status_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
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
		fixes = append(fixes, string(problem.Code)+" "+problem.Fix.Command)
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

func TestAParentGoneOrACycleIsFixedByMovingOntoTheBaseBranch(t *testing.T) {
	cases := map[string]struct {
		params  status.ProblemsParams
		code    status.ProblemCode
		message string
	}{
		"parent_gone": {params: status.ProblemsParams{Branch: "feat/a", Parent: "feat/deleted", ParentGone: true, Onto: "main"}, code: status.CodeParentGone, message: "its parent feat/deleted no longer exists"},
		"cycle":       {params: status.ProblemsParams{Branch: "feat/a", Cycle: true, Onto: "main"}, code: status.CodeCycle, message: "its parents lead back to it"},
		"no_base":     {params: status.ProblemsParams{Branch: "feat/a", Cycle: true}},
		"base_itself": {params: status.ProblemsParams{Branch: "main", Cycle: true, Onto: "main"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			problems := status.Problems(c.params)

			if c.code == "" {
				assert.Empty(t, problems)
				return
			}
			assert.Len(t, problems, 1)
			assert.Equal(t, c.code, problems[0].Code)
			assert.Equal(t, c.message, problems[0].Message)
			assert.Equal(t, &domain.FollowUp{Command: domain.CommandReparent, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}, {Path: "to", Values: []string{"main"}}}, Code: string(c.code), Message: "move it onto main"}, problems[0].Fix)
		})
	}
}
