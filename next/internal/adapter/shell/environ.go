package shell

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type EnvironParams struct {
	Unset []string
	Set   map[string]string
}

func Environ(params EnvironParams) []string {
	env := slices.DeleteFunc(os.Environ(), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		_, set := params.Set[key]
		return set || slices.Contains(params.Unset, key)
	})
	for _, key := range slices.Sorted(maps.Keys(params.Set)) {
		env = append(env, key+"="+params.Set[key])
	}
	return env
}

func ExitCode(err error) (int, bool) {
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() < 0 {
		return 0, false
	}
	return exit.ExitCode(), true
}
