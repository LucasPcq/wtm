package output

import "strings"

var flags = map[string]string{
	"branches":      "<branches>",
	"from":          "--from",
	"env_from":      "--env-from",
	"isolation":     "--isolation",
	"fast_forward":  "--ff",
	"if_not_exists": "--if-not-exists",
	"remember":      "--remember",
}

func Flag(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	flag, known := flags[name]
	switch {
	case !known:
		return path
	case indexed:
		return flag + "[" + index
	}
	return flag
}
