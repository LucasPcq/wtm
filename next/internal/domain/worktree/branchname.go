package worktree

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func BranchProblem(name string) error {
	reason := reason(name)
	if reason == "" {
		return nil
	}
	return fmt.Errorf("%q is not a valid branch name: %s", name, reason)
}

func BlankBranch(name string) bool {
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
	return slices.ContainsFunc(strings.Split(name, "/"), func(component string) bool {
		return strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock")
	})
}

func forbiddenInRef(r rune) bool {
	return r < 0x20 || r == 0x7f || strings.ContainsRune(` ~^:?*[\`, r)
}

const (
	hostLabelMaxLen = 63
	fallbackName    = "wtm"
)

func PathSegment(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

func StateSegment(name string) string {
	return url.PathEscape(name)
}

func Slug(name string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, strings.ToLower(name))
	return cmp.Or(strings.TrimLeft(mapped, "-_"), fallbackName)
}

func HostLabel(name string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, name)
	label := strings.Trim(collapseDashes(mapped), "-")
	if len(label) > hostLabelMaxLen {
		label = strings.Trim(label[:hostLabelMaxLen], "-")
	}
	return cmp.Or(label, fallbackName)
}

func collapseDashes(s string) string {
	var b strings.Builder
	previous := rune(0)
	for _, r := range s {
		if r == '-' && previous == '-' {
			continue
		}
		b.WriteRune(r)
		previous = r
	}
	return b.String()
}

type Clash struct {
	Branch string
	Name   string
}

type ClashParams struct {
	Branch string
	Live   []string
}

func ClashWith(params ClashParams) (Clash, bool) {
	slug := Slug(params.Branch)
	host := HostLabel(slug)
	for _, live := range params.Live {
		if live == "" || live == params.Branch {
			continue
		}
		other := Slug(live)
		if other == slug {
			return Clash{Branch: live, Name: slug}, true
		}
		if HostLabel(other) == host {
			return Clash{Branch: live, Name: host}, true
		}
	}
	return Clash{}, false
}
