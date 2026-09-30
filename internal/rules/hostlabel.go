package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HostLabel turns a derived name into one DNS label. WorktreeSlug is not enough
// here: it keeps underscores, leaves a trailing dash and bounds nothing, none of
// which a hostname accepts.
func HostLabel(s string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, s)

	var b strings.Builder
	for _, r := range mapped {
		if r == '-' && strings.HasSuffix(b.String(), "-") {
			continue
		}
		b.WriteRune(r)
	}

	label := strings.Trim(b.String(), "-")
	if len(label) > domain.HostLabelMaxLen {
		label = strings.Trim(label[:domain.HostLabelMaxLen], "-")
	}
	if label == "" {
		return domain.HostLabelFallback
	}
	return label
}

// IsHostLabels reports whether s is a dotted sequence of valid DNS labels. It is
// the check a hand-written url.host goes through: a value the user typed is
// refused, never silently corrected, so the URL on screen is the one they wrote.
func IsHostLabels(s string) bool {
	if s == "" {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if !isHostLabel(label) {
			return false
		}
	}
	return true
}

func isHostLabel(label string) bool {
	if label == "" || len(label) > domain.HostLabelMaxLen {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, r := range label {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}
