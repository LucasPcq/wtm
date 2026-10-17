package worktree

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// BranchProblem returns why git would refuse name as a branch name, or nil when it would accept it.
func BranchProblem(name string) error {
	reason := reason(name)
	if reason == "" {
		return nil
	}

	return fmt.Errorf("%q is not a valid branch name: %s", name, reason)
}

// BlankBranch reports whether name is empty or only white space.
func BlankBranch(name string) bool {
	return strings.TrimSpace(name) == ""
}

// reason returns why git would refuse name as a branch name (the rules of git check-ref-format), or "" when it would accept it.
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

// PathSegment returns name as one segment of a worktree's path: its slashes become dashes.
func PathSegment(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

// StateSegment returns name as one segment of a path in the state directory, escaped so that no branch name can reach another segment.
func StateSegment(name string) string {
	return url.PathEscape(name)
}

// Slug returns name as the identifier a worktree's environment knows it by: lowercase letters, digits, `_` and `-`, "wtm" when nothing is left.
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

// HostLabel returns name as a DNS label: lowercase letters, digits and single dashes, at most 63 bytes, "wtm" when nothing is left.
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

// Clash is another live branch whose worktree would get the same name as Branch's: the slug or host label they share.
type Clash struct {
	Branch string
	Name   string
}

// ClashWith returns the first branch of others whose slug or host label is the same as branch's, and reports whether there is one; others holds the branches of the live worktrees.
func ClashWith(branch string, others []string) (Clash, bool) {
	slug := Slug(branch)
	host := HostLabel(slug)

	for _, live := range others {
		if live == "" || live == branch {
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
