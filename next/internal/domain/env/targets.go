package env

import (
	"path"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

type LinkKind string

const (
	LinkPort  LinkKind = "env_port"
	LinkValue LinkKind = "env"
)

type Ignored struct {
	Kind LinkKind
	Key  string
	File string
}

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

type OwnedEntry struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Changed bool   `json:"changed"`
}

func OwnedTargets(run project.Run, files []project.EnvFile) []string {
	byDir := make(map[string]string, len(files))
	for _, file := range files {
		byDir[path.Dir(file.Target)] = file.Target
	}
	var targets []string
	for _, dir := range ComposeDirs(run) {
		if target, declared := byDir[dir]; declared {
			targets = append(targets, target)
		}
	}
	return targets
}

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
