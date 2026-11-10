package project

import (
	"path"
	"slices"
	"strings"
)

// Detected is what a repository's files suggest of its settings, for a first init: the .env files it holds, each with its template, the hooks a worktree would run, each with the file it was found in, and the folders a hook can run in.
type Detected struct {
	EnvFiles []EnvFile
	OnCreate []FoundHook
	OnClean  []FoundHook
	Dirs     []string
}

// FoundHook is a hook the repository suggests, and the file it was found in, relative to the repository.
type FoundHook struct {
	Hook
	File string `json:"file"`
}

const (
	envFileName      = ".env"
	envLocalFileName = ".env.local"
)

// TemplateSuffixes are the suffixes of an .env file's template, in the order one is preferred to another beside the same file.
func TemplateSuffixes() []string {
	return []string{".example", ".dist", ".sample", ".template", ".tmpl"}
}

// EnvCandidate reports whether name, a file's base name, is an .env file wtm copies: .env, .env.local, or .env with a template suffix; .env.production and the like are not.
func EnvCandidate(name string) bool {
	return name == envFileName || name == envLocalFileName || templateRank(name) >= 0
}

// EnvFilesOf are the .env files paths make, slash-separated and relative to the repository: each .env with its preferred template beside it, a template alone standing for its .env, and .env.local marked local; sorted by target.
func EnvFilesOf(paths []string) []EnvFile {
	byTarget := map[string]*EnvFile{}
	ranks := map[string]int{}
	entry := func(target string) *EnvFile {
		if byTarget[target] == nil {
			byTarget[target], ranks[target] = &EnvFile{Target: target}, len(TemplateSuffixes())
		}
		return byTarget[target]
	}

	for _, p := range paths {
		dir, name := path.Split(p)
		switch rank := templateRank(name); {
		case name == envFileName:
			entry(p)
		case name == envLocalFileName:
			entry(p).Local = true
		case rank >= 0:
			target := dir + envFileName
			if file := entry(target); rank < ranks[target] {
				file.Template, ranks[target] = p, rank
			}
		}
	}

	files := make([]EnvFile, 0, len(byTarget))
	for _, file := range byTarget {
		files = append(files, *file)
	}
	slices.SortFunc(files, func(a, b EnvFile) int { return strings.Compare(a.Target, b.Target) })
	return files
}

// templateRank is the rank of name's suffix among TemplateSuffixes when name is .env with one, else -1.
func templateRank(name string) int {
	return slices.IndexFunc(TemplateSuffixes(), func(suffix string) bool { return name == envFileName+suffix })
}

// The installers a repository's root says it uses: the package manager package.json names, else the first lockfile found, in this order.
var installers = []struct{ manager, lockfile, cmd string }{
	{"pnpm", "pnpm-lock.yaml", "pnpm install"},
	{"npm", "package-lock.json", "npm install"},
	{"yarn", "yarn.lock", "yarn install"},
	{"bun", "bun.lockb", "bun install"},
	{"go", "go.mod", "go mod download"},
	{"pip", "requirements.txt", "pip install -r requirements.txt"},
}

const packageJSON = "package.json"

// InstallOf is the hook that installs the dependencies of a repository whose root holds files: the one of the package manager package.json names (manager, as its packageManager field spells it, "pnpm@9.0.0"), found in package.json, else the one of the first lockfile among files; false when neither says.
func InstallOf(manager string, files []string) (FoundHook, bool) {
	name, _, _ := strings.Cut(manager, "@")
	for _, installer := range installers {
		if name != "" && installer.manager == name {
			return FoundHook{Hook: Hook{Cmd: installer.cmd}, File: packageJSON}, true
		}
	}
	for _, installer := range installers {
		if slices.Contains(files, installer.lockfile) {
			return FoundHook{Hook: Hook{Cmd: installer.cmd}, File: installer.lockfile}, true
		}
	}

	return FoundHook{}, false
}

// composeFiles are the names of a docker compose file, in the order docker compose looks for one.
var composeFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

// TeardownOf is the hook that stops what a worktree's docker compose file started, before its removal, when the root of a repository holding files has one: `docker compose down`, found in that file.
func TeardownOf(files []string) (FoundHook, bool) {
	for _, name := range composeFiles {
		if slices.Contains(files, name) {
			return FoundHook{Hook: Hook{Cmd: "docker compose down"}, File: name}, true
		}
	}

	return FoundHook{}, false
}
