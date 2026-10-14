package git

import (
	"errors"
	"strings"
)

type Error struct {
	Args   []string
	Exit   int
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	detail := e.Stderr
	if detail == "" {
		detail = e.Err.Error()
	}
	return "git " + strings.Join(e.Args, " ") + ": " + detail
}

func (e *Error) Unwrap() error { return e.Err }

var (
	ErrNoLocalBranch = errors.New("no local branch")
	errNoRef         = errors.New("no ref given")
)
