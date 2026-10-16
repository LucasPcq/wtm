// Package branch reads branch names: whether git accepts one, the names derived from it, and where it stands against origin.
package branch

import (
	"fmt"
	"strings"
)

func Problem(name string) error {
	reason := reason(name)
	if reason == "" {
		return nil
	}
	return fmt.Errorf("%q is not a valid branch name: %s", name, reason)
}

func Blank(name string) bool {
	return strings.TrimSpace(name) == ""
}

func reason(name string) string {
	switch {
	case name == "@", name == "HEAD":
		return "it is reserved by git"
	case strings.HasPrefix(name, "-"):
		return "it cannot start with -"
	case strings.HasPrefix(name, "/"), strings.HasSuffix(name, "/"), strings.Contains(name, "//"):
		return "a / cannot start or end it, or follow another"
	case strings.HasSuffix(name, "."), strings.Contains(name, ".."):
		return "it cannot hold .. or end with ."
	case strings.Contains(name, "@{"):
		return "it cannot hold @{"
	case strings.ContainsFunc(name, forbiddenInRef):
		return `it cannot hold a space, a control character or any of ~ ^ : ? * [ \`
	case badComponent(name):
		return "no part between slashes may start with . or end with .lock"
	}
	return ""
}

func badComponent(name string) bool {
	for _, component := range strings.Split(name, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return true
		}
	}
	return false
}

func forbiddenInRef(r rune) bool {
	return r < 0x20 || r == 0x7f || strings.ContainsRune(` ~^:?*[\`, r)
}
