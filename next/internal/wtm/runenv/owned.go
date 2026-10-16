package runenv

import (
	"path"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

var ownedKeys = [...]string{envComposeProject}

type OwnedEntry struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Changed bool   `json:"changed"`
}

type OwnedTargetsParams struct {
	Run   run.Config
	Files []project.EnvFile
}

func OwnedTargets(params OwnedTargetsParams) []string {
	byDir := make(map[string]string, len(params.Files))
	for _, file := range params.Files {
		byDir[jobDir(path.Dir(file.Target))] = file.Target
	}
	var targets []string
	for _, dir := range composeDirs(params.Run) {
		if target, declared := byDir[dir]; declared {
			targets = append(targets, target)
		}
	}
	return targets
}

type ownedWritesParams struct {
	Run    run.Config
	Files  []project.EnvFile
	Values map[string]string
}

func ownedWrites(params ownedWritesParams) []OwnedEntry {
	var writes []OwnedEntry
	for _, target := range OwnedTargets(OwnedTargetsParams{Run: params.Run, Files: params.Files}) {
		for _, key := range ownedKeys {
			if value := params.Values[key]; value != "" {
				writes = append(writes, OwnedEntry{File: target, Key: key, Value: value})
			}
		}
	}
	return writes
}
