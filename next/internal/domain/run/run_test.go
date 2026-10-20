package run

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheJobsUpInAWorktreeAreItsOwnByName(t *testing.T) {
	jobs := []Job{{Name: "web", WorkDir: "/a", Up: true}, {Name: "api", WorkDir: "/a", Up: true}, {Name: "db", WorkDir: "/a"}, {Name: "web", WorkDir: "/b", Up: true}}

	assert.Equal(t, []string{"api", "web"}, UpIn(jobs, "/a"))
	assert.Equal(t, []string{}, UpIn(jobs, "/c"))
}

func TestWhyTheJobsAreUnknownNamesTheDaemonsFailure(t *testing.T) {
	assert.Equal(t, "the run daemon is not running: which jobs are up is unknown", UnknownMessage(ErrUnknown))
	assert.Equal(t, "couldn't ask the run daemon which jobs are up: refused", UnknownMessage(errors.New("refused")))
}
