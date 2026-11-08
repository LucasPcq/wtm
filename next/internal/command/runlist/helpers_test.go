package runlist_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// read runs run list over s for req, asked from feat/a, and returns the worktree read.
func read(t *testing.T, s seed, req runlist.Request) runlist.Detail {
	t.Helper()
	outcome, err := usecase(s).Run(t.Context(), req.From(domain.Caller{Cwd: featA}), domain.RunOptions{})
	require.NoError(t, err)
	require.Len(t, outcome.Items, 1)
	require.NotNil(t, outcome.Items[0].Detail)
	return *outcome.Items[0].Detail
}

// job is the job of d called name.
func job(t *testing.T, d runlist.Detail, name string) runlist.Job {
	t.Helper()
	for _, found := range d.Jobs {
		if found.Name == name {
			return found
		}
	}
	t.Fatalf("no job %s", name)
	return runlist.Job{}
}

// stateOf is the state of the job of d called name, "" when it cannot be told.
func stateOf(t *testing.T, d runlist.Detail, name string) run.State {
	t.Helper()
	if state := job(t, d, name).State; state != nil {
		return *state
	}
	return ""
}

// commands are the command lines of follow-ups, each as its command and its answers.
func commands(followUps []domain.FollowUp) [][]string {
	var lines [][]string
	for _, f := range followUps {
		line := []string{f.Command}
		for _, a := range f.Answers {
			line = append(line, a.Path+"="+a.Values[0])
		}
		lines = append(lines, line)
	}
	return lines
}
