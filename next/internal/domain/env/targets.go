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

type TargetsParams struct {
	Run   project.Run
	Files []project.EnvFile
}

func Targets(params TargetsParams) (project.Run, []Ignored) {
	declared := make(map[string]bool, len(params.Files))
	for _, file := range params.Files {
		declared[file.Target] = true
	}
	kept := params.Run
	kept.EnvPorts, kept.EnvValues = nil, nil
	var ignored []Ignored
	for _, link := range params.Run.EnvPorts {
		if !declared[link.File] {
			ignored = append(ignored, Ignored{Kind: LinkPort, Key: link.Key, File: link.File})
			continue
		}
		kept.EnvPorts = append(kept.EnvPorts, link)
	}
	for _, link := range params.Run.EnvValues {
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

type OwnedTargetsParams struct {
	Run   project.Run
	Files []project.EnvFile
}

func OwnedTargets(params OwnedTargetsParams) []string {
	byDir := make(map[string]string, len(params.Files))
	for _, file := range params.Files {
		byDir[path.Dir(file.Target)] = file.Target
	}
	var targets []string
	for _, dir := range ComposeDirs(params.Run) {
		if target, declared := byDir[dir]; declared {
			targets = append(targets, target)
		}
	}
	return targets
}

type ownedWritesParams struct {
	Run            project.Run
	Files          []project.EnvFile
	ComposeProject string
}

func ownedWrites(params ownedWritesParams) []OwnedEntry {
	if params.ComposeProject == "" {
		return nil
	}
	var writes []OwnedEntry
	for _, target := range OwnedTargets(OwnedTargetsParams{Run: params.Run, Files: params.Files}) {
		writes = append(writes, OwnedEntry{File: target, Key: ComposeProjectKey, Value: params.ComposeProject})
	}
	return writes
}
