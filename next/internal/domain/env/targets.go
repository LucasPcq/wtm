package env

import (
	"path"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// LinkKind is the kind of a run.toml link into a .env file, spelled as its table.
type LinkKind string

// The two kinds of link: a port, or a value.
const (
	LinkPort  LinkKind = "env_port"
	LinkValue LinkKind = "env"
)

// An Ignored link names a .env file the project does not declare, so wtm does not write it.
type Ignored struct {
	Kind LinkKind
	Key  string
	File string
}

// Targets returns run keeping only the links into a declared .env file, and the links it left out.
func Targets(run project.Run, files []project.EnvFile) (project.Run, []Ignored) {
	declared := make(map[string]bool, len(files))
	for _, file := range files {
		declared[file.Target] = true
	}

	kept := run
	kept.EnvPorts, kept.EnvValues = nil, nil
	var ignored []Ignored
	for _, link := range run.EnvPorts {
		if !declared[link.File] {
			ignored = append(ignored, Ignored{Kind: LinkPort, Key: link.Key, File: link.File})
			continue
		}
		kept.EnvPorts = append(kept.EnvPorts, link)
	}
	for _, link := range run.EnvValues {
		if !declared[link.File] {
			ignored = append(ignored, Ignored{Kind: LinkValue, Key: link.Key, File: link.File})
			continue
		}
		kept.EnvValues = append(kept.EnvValues, link)
	}
	return kept, ignored
}

// An OwnedEntry is a value wtm sets in a .env file, and whether setting it changed the file.
type OwnedEntry struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Changed bool   `json:"changed"`
}

// OwnedTargets returns the declared .env files docker compose reads, the .env of a directory where a job runs it, which wtm gives a COMPOSE_PROJECT_NAME.
func OwnedTargets(run project.Run, files []project.EnvFile) []string {
	declared := make(map[string]string, len(files))
	for _, file := range files {
		declared[path.Clean(file.Target)] = file.Target
	}

	var targets []string
	for _, dir := range composeDirs(run) {
		if target, ok := declared[path.Join(dir, ".env")]; ok {
			targets = append(targets, target)
		}
	}
	return targets
}

// ownedWrites returns a COMPOSE_PROJECT_NAME write into each owned target, none when the worktree has no compose project.
func ownedWrites(run project.Run, files []project.EnvFile, composeProject string) []OwnedEntry {
	if composeProject == "" {
		return nil
	}

	var writes []OwnedEntry
	for _, target := range OwnedTargets(run, files) {
		writes = append(writes, OwnedEntry{File: target, Key: ComposeProjectKey, Value: composeProject})
	}
	return writes
}
