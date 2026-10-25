package run

import (
	"errors"
	"testing"

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

func TestWhyTheJobsAreUnknownNamesTheDaemonsFailure(t *testing.T) {
	assert.Equal(t, "the run daemon is not running: which jobs are up is unknown", UnknownMessage(ErrUnknown))
	assert.Equal(t, "couldn't ask the run daemon which jobs are up: refused", UnknownMessage(errors.New("refused")))
}
