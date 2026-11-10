package project

import "strings"

// Workspace is what a repository's package manifests say: the package manager it uses ("" when none says), how it was found (its packageManager field, else a lockfile), and the places that hold a package.json, the root first when it holds one, then its workspace packages by folder.
type Workspace struct {
	Manager     string
	ManagerFrom string
	Places      []Place
}

// Place is a folder of the repository holding a package.json: its path relative to the repository ("." for the root), its package's name without its scope, and its scripts, sorted by name.
type Place struct {
	Dir     string   `json:"dir"`
	Name    string   `json:"name"`
	Scripts []Script `json:"scripts"`
}

// Script is a script of a package.json: its name and the command it runs.
type Script struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}

// ManagerOf is the package manager a packageManager field names ("pnpm@9.12.0" is pnpm), and whether it names one.
func ManagerOf(field string) (string, bool) {
	name, _, _ := strings.Cut(field, "@")
	return name, name != ""
}

// Lockfiles are the lockfile of each package manager, in the order one is preferred when several lie at the root.
func Lockfiles() []struct{ Manager, File string } {
	return []struct{ Manager, File string }{
		{"pnpm", "pnpm-lock.yaml"}, {"npm", "package-lock.json"}, {"yarn", "yarn.lock"}, {"bun", "bun.lockb"}, {"bun", "bun.lock"},
	}
}

// Matches reports whether the folder dir, slash-separated and relative to the repository, is selected by the workspace pattern, where * is any name and ** any number of folders.
func Matches(pattern, dir string) bool {
	return matchSegments(strings.Split(strings.Trim(pattern, "/"), "/"), strings.Split(dir, "/"))
}

func matchSegments(pattern, dir []string) bool {
	switch {
	case len(pattern) == 0:
		return len(dir) == 0
	case pattern[0] == "**":
		return matchSegments(pattern[1:], dir) || len(dir) > 0 && matchSegments(pattern, dir[1:])
	case len(dir) == 0:
		return false
	}
	return segmentMatches(pattern[0], dir[0]) && matchSegments(pattern[1:], dir[1:])
}

// segmentMatches matches one folder name against one segment of a pattern, where * is any run of characters.
func segmentMatches(pattern, name string) bool {
	before, after, wild := strings.Cut(pattern, "*")
	if !wild {
		return pattern == name
	}
	if !strings.HasPrefix(name, before) {
		return false
	}
	rest := name[len(before):]
	for i := 0; i <= len(rest); i++ {
		if segmentMatches(after, rest[i:]) {
			return true
		}
	}
	return false
}
