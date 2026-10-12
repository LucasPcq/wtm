package git

import (
	"errors"
	"strings"
)

// Error is a git call that failed. Exit is -1 when git did not run to an exit
// status: missing from PATH, or killed.
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

var errNoRef = errors.New("no ref given")
