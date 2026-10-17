package output

import (
	"strings"
)

const positional = "branches"

func Flag(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	suffix := ""
	if indexed {
		suffix = "[" + index
	}
	switch name {
	case positional:
		return "<" + positional + ">" + suffix
	case "fast_forward":
		return "--ff" + suffix
	}
	return "--" + strings.ReplaceAll(name, "_", "-") + suffix
}
