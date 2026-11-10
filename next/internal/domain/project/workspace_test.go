package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWorkspacePatternMatchesFoldersByName(t *testing.T) {
	cases := map[string]struct {
		pattern, dir string
		want         bool
	}{
		"star":          {"apps/*", "apps/web", true},
		"star_deeper":   {"apps/*", "apps/web/src", false},
		"double_star":   {"packages/**", "packages/ui/lib", true},
		"double_star_0": {"packages/**", "packages", true},
		"prefix":        {"apps/web-*", "apps/web-admin", true},
		"other":         {"apps/*", "tools/x", false},
		"trailing":      {"apps/*/", "apps/web", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Matches(c.pattern, c.dir))
		})
	}
}
