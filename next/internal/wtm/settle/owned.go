package settle

import (
	"path"

	"github.com/LucasPcq/wtm/next/internal/wtm/jobenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

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
		byDir[path.Dir(file.Target)] = file.Target
	}
	var targets []string
	for _, dir := range jobenv.ComposeDirs(params.Run) {
		if target, declared := byDir[dir]; declared {
			targets = append(targets, target)
		}
	}
	return targets
}

type ownedWritesParams struct {
	Run            run.Config
	Files          []project.EnvFile
	ComposeProject string
}

func ownedWrites(params ownedWritesParams) []OwnedEntry {
	if params.ComposeProject == "" {
		return nil
	}
	var writes []OwnedEntry
	for _, target := range OwnedTargets(OwnedTargetsParams{Run: params.Run, Files: params.Files}) {
		writes = append(writes, OwnedEntry{File: target, Key: jobenv.ComposeProjectKey, Value: params.ComposeProject})
	}
	return writes
}
