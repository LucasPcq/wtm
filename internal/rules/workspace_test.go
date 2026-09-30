package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestMatchWorkspacePattern(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		dir     string
		want    bool
	}{
		{pattern: "apps/*", dir: "apps/web", want: true},
		{pattern: "apps/*", dir: "apps/app-1/back", want: false},
		// Le globstar traverse n'importe quelle profondeur. filepath.Glob le
		// traitait comme un `*`, ce qui rendait apps/app-1/back invisible.
		{pattern: "apps/**", dir: "apps/app-1/back", want: true},
		{pattern: "apps/**", dir: "apps/web", want: true},
		{pattern: "apps/**", dir: "packages/shared", want: false},
		{pattern: "**", dir: "a/b/c/d", want: true},
		{pattern: "**/back", dir: "apps/app-1/back", want: true},
		{pattern: "**/back", dir: "apps/app-1/front", want: false},
		{pattern: "apps/*/*", dir: "apps/app-1/back", want: true},
		{pattern: "packages/shared", dir: "packages/shared", want: true},
		{pattern: "apps/app-?", dir: "apps/app-1", want: true},
	} {
		t.Run(tc.pattern+" vs "+tc.dir, func(t *testing.T) {
			if got := rules.MatchWorkspacePattern(tc.pattern, tc.dir); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseWorkspacePatternsSplitsNegations(t *testing.T) {
	parsed := rules.ParseWorkspacePatterns([]string{"apps/**", "!apps/legacy", "", "packages/*"})

	if len(parsed.Include) != 2 || parsed.Include[0] != "apps/**" || parsed.Include[1] != "packages/*" {
		t.Errorf("include = %v", parsed.Include)
	}
	if len(parsed.Exclude) != 1 || parsed.Exclude[0] != "apps/legacy" {
		t.Errorf("exclude = %v", parsed.Exclude)
	}
}

func TestSelectsWorkspaceHonoursExclusions(t *testing.T) {
	patterns := rules.ParseWorkspacePatterns([]string{"apps/**", "!apps/legacy"})

	if !rules.SelectsWorkspace(patterns, "apps/app-1/back") {
		t.Error("apps/app-1/back est sélectionné par apps/**")
	}
	if rules.SelectsWorkspace(patterns, "apps/legacy") {
		t.Error("une exclusion reprend ce que le pattern large a pris")
	}
}
