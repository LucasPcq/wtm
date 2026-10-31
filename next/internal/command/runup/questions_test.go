package runup_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestTheWorktreeTheCallerStandsInIsTheDefaultAndEachOptionSaysWhatItRuns(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)
	w.daemon.listed[0].Profile = "dev"

	questions := w.questions(t, runup.Request{Caller: domain.Caller{Cwd: w.at(featA)}})

	assert.Equal(t, []string{featA}, questions.Worktrees.Picked)
	assert.Equal(t, domain.OriginDefault, questions.Worktrees.Origin)
	var b domain.Option
	for _, option := range questions.Worktrees.Options {
		if option.Value == featB {
			b = option
		}
	}
	assert.Equal(t, domain.Option{Value: featB, Up: []string{"api"}, Profile: "dev"}, b)
	assert.Empty(t, questions.Errors)
}

func TestTheProfilesAreOfferedWithTheirJobsTheDefaultPicked(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, starting("", featA))

	assert.Equal(t, "dev", questions.Profile.Value)
	assert.Equal(t, domain.OriginConfig, questions.Profile.Origin)
	assert.Equal(t, domain.Option{Value: "dev", Default: true, Jobs: []string{"api", "web"}}, questions.Profiles[0])
}

func TestEachProblemOfARequestIsAFieldErrorOfItsOwn(t *testing.T) {
	w := newWorld(t)
	yes := true
	for name, c := range map[string]struct {
		req  runup.Request
		want []string
	}{
		"no_worktree":        {runup.Request{}, []string{domain.CodeRequired}},
		"not_found":          {starting("", "nope"), []string{domain.CodeNotFound}},
		"given_twice":        {starting("", featA, featA), []string{domain.CodeDistinct}},
		"gone":               {starting("", gone), []string{runup.CodeWorktreeGone}},
		"profile_not_found":  {starting("ghost", featA), []string{domain.CodeNotFound}},
		"exclusive_several":  {runup.Request{Worktrees: []string{featA, featB}, Exclusive: &yes}, []string{runup.CodeExclusiveMany}},
		"exclusive_parallel": {runup.Request{Worktrees: []string{featA}, Exclusive: &yes, Parallel: true}, []string{runup.CodeBothWays}},
	} {
		t.Run(name, func(t *testing.T) {
			refused := w.refused(t, c.req)

			assert.Equal(t, domain.KindInvalid, refused.Kind)
			assert.Equal(t, c.want, fieldCodes(refused.Fields))
		})
	}
}

func TestSeveralProfilesAndNoDefaultRequireOne(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.stateDir+"/run.toml", "[[job]]\nname = \"api\"\nkind = \"service\"\ncmd = \"api\"\n\n[[profile]]\nname = \"a\"\njobs = [\"api\"]\n\n[[profile]]\nname = \"b\"\njobs = [\"api\"]\n")

	refused := w.refused(t, starting("", featA))

	assert.Equal(t, []string{domain.CodeRequired}, fieldCodes(refused.Fields))
	assert.Equal(t, runup.PathProfile, refused.Fields[0].Path)
}

func TestTheOthersQuestionIsAskedOnlyWhenAnotherWorktreeRunsJobsAndRunTomlDoesNotSay(t *testing.T) {
	w := newWorld(t)
	assert.Equal(t, domain.SkipAlone, w.questions(t, starting("", featA)).Exclusive.Skip)

	w.up("api", featB, nil)
	asked := w.questions(t, starting("", featA)).Exclusive
	assert.Empty(t, asked.Skip)
	assert.False(t, asked.Value, "keeping them is the safe default")

	writeFile(t, w.stateDir+"/run.toml", "concurrency = \"exclusive\"\n"+runToml)
	settled := w.questions(t, starting("", featA)).Exclusive
	assert.Equal(t, domain.SkipConfigured, settled.Skip)
	assert.True(t, settled.Value)
}

func TestRunTomlsExclusiveIsSetAsideForSeveralWorktreesWithAWarning(t *testing.T) {
	w := newWorld(t)
	w.up("api", verbatim, nil)
	writeFile(t, w.stateDir+"/run.toml", "concurrency = \"exclusive\"\n"+runToml)

	planned, warnings := w.plan(t, starting("", featA, featB))

	assert.False(t, planned.Exclusive)
	assert.Equal(t, []string{runup.CodeExclusiveIgnored}, codes(warnings))
}

func TestTheForceQuestionIsAskedOnlyWhenAJobChangesForeignData(t *testing.T) {
	w := newWorld(t)

	assert.Equal(t, domain.SkipSafe, w.questions(t, starting("full", featA)).Force.Skip)
	assert.Empty(t, w.questions(t, starting("full", verbatim)).Force.Skip)
}

func TestARequestReadsBackAsTheAnswersItWasGiven(t *testing.T) {
	no := false
	req := runup.Request{Worktrees: []string{featA, featB}, Profile: "full", Exclusive: &no, Parallel: true, Force: true, NoProbe: true, Detach: true}

	again, err := domain.Fold(runup.Request{}, domain.AnswersOf(runup.Questions{}.Form(), req))

	assert.NoError(t, err)
	assert.Equal(t, req, again)
}
