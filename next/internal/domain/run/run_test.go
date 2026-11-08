package run

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTheURLsOfAWorktreeAreThoseOfItsJobsUp(t *testing.T) {
	jobs := []Job{{Name: "web", WorkDir: "/a", Up: true, URL: "http://web.a"}, {Name: "api", WorkDir: "/a", Up: true, URL: "http://api.a"}, {Name: "worker", WorkDir: "/a", Up: true}, {Name: "db", WorkDir: "/a", URL: "http://db.a"}, {Name: "web", WorkDir: "/b", Up: true, URL: "http://web.b"}}

	assert.Equal(t, []string{"http://api.a", "http://web.a"}, URLsIn(jobs, "/a"))
	assert.Equal(t, []string{}, URLsIn(jobs, "/c"))
}

func TestTheJobsUpInAWorktreeAreItsOwnByName(t *testing.T) {
	jobs := []Job{{Name: "web", WorkDir: "/a", Up: true}, {Name: "api", WorkDir: "/a", Up: true}, {Name: "db", WorkDir: "/a"}, {Name: "web", WorkDir: "/b", Up: true}}

	assert.Equal(t, []string{"api", "web"}, UpIn(jobs, "/a"))
	assert.Equal(t, []string{}, UpIn(jobs, "/c"))
}

func TestTheProfileOfAWorktreeIsThatOfTheJobsItRunsForItself(t *testing.T) {
	jobs := []Job{
		{Name: "db", WorkDir: "/main", Up: true, Profile: "full", SharedDir: "/main"},
		{Name: "db", WorkDir: "/a", Up: true, Profile: "full", Hold: HoldClaim, SharedDir: "/main"},
		{Name: "web", WorkDir: "/a", Profile: "old"},
		{Name: "api", WorkDir: "/a", Up: true, Profile: "dev"},
	}

	assert.Equal(t, "dev", ProfileIn(jobs, "/a"), "a claim and a job down say nothing")
	assert.Empty(t, ProfileIn(jobs, "/main"), "a shared service run for every worktree is no profile of main's")
	assert.Empty(t, ProfileIn(jobs, "/b"))
}

func TestWhyTheJobsAreUnknownNamesTheDaemonsFailure(t *testing.T) {
	assert.Equal(t, "the run daemon is not running: which jobs are up is unknown", UnknownMessage(ErrUnknown))
	assert.Equal(t, "couldn't ask the run daemon which jobs are up: refused", UnknownMessage(errors.New("refused")))
}

func TestAJobAListedRunnerStartsStandsAsItsRunnerNestedOrNot(t *testing.T) {
	declared := project.Run{Jobs: []project.Job{
		{Name: "stack", Runs: []string{"proxy"}},
		{Name: "proxy", Runs: []string{"cache"}},
		{Name: "cache"},
		{Name: "web"},
	}}
	jobs := []Job{{Name: "stack", WorkDir: "/a", State: Running, Up: true}}
	standing := func(name string) (Standing, bool) {
		return StandingIn(StandingParams{Jobs: jobs, Dir: "/a", Name: name, Declared: declared})
	}

	cache, ok := standing("cache")
	assert.True(t, ok)
	assert.Equal(t, "stack", cache.Runner, "the stack starts proxy, which starts cache")
	stack, ok := standing("stack")
	assert.True(t, ok)
	assert.Empty(t, stack.Runner, "a job listed stands as itself")
	_, ok = standing("web")
	assert.False(t, ok)
}

func TestARunnerUpWinsOverOneThatIsNot(t *testing.T) {
	declared := project.Run{Jobs: []project.Job{{Name: "old", Runs: []string{"cache"}}, {Name: "stack", Runs: []string{"cache"}}, {Name: "cache"}}}
	jobs := []Job{{Name: "old", WorkDir: "/a", State: Crashed}, {Name: "stack", WorkDir: "/a", State: Running, Up: true}}

	cache, _ := StandingIn(StandingParams{Jobs: jobs, Dir: "/a", Name: "cache", Declared: declared})

	assert.Equal(t, "stack", cache.Runner)
}
