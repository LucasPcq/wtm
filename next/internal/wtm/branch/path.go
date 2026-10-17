package branch

import (
	"cmp"
	"net/url"
	"strings"
)

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
