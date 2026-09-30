package rules

import (
	"path"
	"strings"
)

type WorkspacePatterns struct {
	Include []string
	Exclude []string
}

// ParseWorkspacePatterns splits a workspace declaration into what it selects
// and what it takes back. A "!" prefix is pnpm's and npm's way of carving an
// exclusion out of a wider pattern, so dropping those lines would widen the
// selection rather than narrow it.
func ParseWorkspacePatterns(patterns []string) WorkspacePatterns {
	var parsed WorkspacePatterns
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		if negated, found := strings.CutPrefix(trimmed, "!"); found {
			parsed.Exclude = append(parsed.Exclude, negated)
			continue
		}
		parsed.Include = append(parsed.Include, trimmed)
	}
	return parsed
}

// SelectsWorkspace reports whether a directory, relative to the project root
// and slash-separated, is one of the packages the patterns declare.
func SelectsWorkspace(patterns WorkspacePatterns, relDir string) bool {
	for _, pattern := range patterns.Exclude {
		if MatchWorkspacePattern(pattern, relDir) {
			return false
		}
	}
	for _, pattern := range patterns.Include {
		if MatchWorkspacePattern(pattern, relDir) {
			return true
		}
	}
	return false
}

// MatchWorkspacePattern matches a workspace pattern against a relative
// directory, treating "**" as the globstar pnpm and npm mean by it: any number
// of path segments, none included. Go's filepath.Glob does not — it matches a
// single segment — so "apps/**" resolved to the first level only and a
// package at apps/app-1/back was invisible to detection (LUC-208).
func MatchWorkspacePattern(pattern, relDir string) bool {
	return matchSegments(splitPattern(pattern), splitPattern(relDir))
}

func matchSegments(pattern, dir []string) bool {
	if len(pattern) == 0 {
		return len(dir) == 0
	}

	if pattern[0] == "**" {
		for skip := 0; skip <= len(dir); skip++ {
			if matchSegments(pattern[1:], dir[skip:]) {
				return true
			}
		}
		return false
	}

	if len(dir) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], dir[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], dir[1:])
}

func splitPattern(value string) []string {
	var segments []string
	for _, segment := range strings.Split(path.Clean(strings.TrimSpace(value)), "/") {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	return segments
}
