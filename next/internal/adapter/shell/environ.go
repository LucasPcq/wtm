package shell

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Environ returns this process's environment without the variables named in
// unset, with set added over it, in the form [exec.Cmd.Env] takes.
func Environ(unset []string, set map[string]string) []string {
	env := slices.DeleteFunc(os.Environ(), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		_, overridden := set[key]
		return overridden || slices.Contains(unset, key)
	})

	for _, key := range slices.Sorted(maps.Keys(set)) {
		env = append(env, key+"="+set[key])
	}
	return env
}

// ExitCode returns the exit code of the child whose run returned err, and
// reports whether it exited at all: a child killed by a signal, or an err that
// is not an exit, has none.
func ExitCode(err error) (int, bool) {
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() < 0 {
		return 0, false
	}
	return exit.ExitCode(), true
}
