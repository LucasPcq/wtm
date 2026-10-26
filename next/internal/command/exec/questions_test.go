package exec_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestTheFormOffersEveryWorktreeThatStandsTheCallersPickedFirst(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, exec.Request{Command: "true", Caller: domain.Caller{Cwd: w.at(featB)}})

	var values []string
	for _, option := range questions.Options {
		values = append(values, option.Value)
	}
	assert.Equal(t, []string{"main", w.at("detached"), featA, featB, numbered, unnumbered}, values)
	assert.Equal(t, []string{featB}, questions.Picked)
	assert.Equal(t, map[string][]string{domain.PathWorktree: {domain.CodeRequired}}, fieldCodes(questions.Errors))
}

func TestEachProblemOfARequestIsAFieldErrorOfItsOwn(t *testing.T) {
	w := newWorld(t)
	for name, c := range map[string]struct {
		req  exec.Request
		want map[string][]string
	}{
		"no_worktree":        {exec.Request{Command: "true"}, map[string][]string{domain.PathWorktree: {domain.CodeRequired}}},
		"no_command":         {running(" ", featA), map[string][]string{exec.PathCommand: {domain.CodeRequired}}},
		"command_invalid":    {running("if then", featA), map[string][]string{exec.PathCommand: {exec.CodeCommandInvalid}}},
		"not_found":          {running("true", featA, "nope"), map[string][]string{"worktree[1]": {domain.CodeNotFound}}},
		"given_twice":        {running("true", featA, featA), map[string][]string{"worktree[1]": {domain.CodeDistinct}}},
		"gone":               {running("true", gone), map[string][]string{"worktree[0]": {exec.CodeWorktreeGone}}},
		"all_with_worktrees": {exec.Request{All: true, Worktrees: []string{featA}, Command: "true"}, map[string][]string{exec.PathAll: {exec.CodeAllWithWorktrees}}},
		"jobs_negative":      {exec.Request{All: true, Command: "true", Jobs: "-1"}, map[string][]string{exec.PathJobs: {exec.CodeJobsInvalid}}},
		"jobs_no_number":     {exec.Request{All: true, Command: "true", Jobs: "two"}, map[string][]string{exec.PathJobs: {exec.CodeJobsInvalid}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := w.usecase.Plan(t.Context(), c.req)

			failure := failureOf(t, err)
			assert.Equal(t, domain.KindInvalid, failure.Kind)
			assert.Equal(t, c.want, fieldCodes(failure.Fields))
		})
	}
}

func TestACommandTheShellRefusesSaysWhyInTheShellsWords(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, running("if then", featA))

	assert.Contains(t, questions.Errors[0].Params["reason"], "syntax error")
}

func TestAllSkipsTheWorktreesAndOnlyTheirFlagsSetAllJobsAndPrint(t *testing.T) {
	form := exec.Questions{Request: exec.Request{All: true}}.Form()

	skips := map[string]domain.Skip{}
	for _, field := range form.Fields {
		skips[field.Spec().Path] = field.Spec().Skip
	}
	assert.Equal(t, map[string]domain.Skip{domain.PathWorktree: domain.SkipAll, exec.PathCommand: "", exec.PathAll: domain.SkipFlagOnly, exec.PathJobs: domain.SkipFlagOnly, exec.PathPrint: domain.SkipFlagOnly}, skips)
}

func TestARequestReadsBackAsTheAnswersItWasGiven(t *testing.T) {
	req := exec.Request{Worktrees: []string{featA, featB}, Command: "pnpm test", Jobs: "2", Print: true}

	again, err := domain.Fold(exec.Request{}, domain.AnswersOf(exec.Questions{}.Form(), req))

	assert.NoError(t, err)
	assert.Equal(t, req, again)
}
