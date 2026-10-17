package shell

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

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

func ExitCode(err error) (int, bool) {
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() < 0 {
		return 0, false
	}
	return exit.ExitCode(), true
}
