package env

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type MissingTargetsParams struct {
	WorktreePath string
	MainPath     string
	// ParentPath is the parent's worktree, empty when it has none.
	ParentPath string
	Strategy   domain.EnvStrategy
	Files      []domain.EnvFile
}

// MissingTargets names the declared .env files the worktree lacks, and whether
// `wtm env` will rebuild each as create would have. It only stats files, so no
// value can leak from it.
func MissingTargets(params MissingTargetsParams) []domain.EnvMissingFile {
	missing := []domain.EnvMissingFile{}
	for _, file := range params.Files {
		if fileExists(filepath.Join(params.WorktreePath, file.Target)) {
			continue
		}
		template := resolveTemplateSrc(params.WorktreePath, file) != ""
		missing = append(missing, domain.EnvMissingFile{
			Target:      file.Target,
			Scaffolded:  scaffolds(scaffoldsParams{MissingTargetsParams: params, File: file, Template: template}),
			HasTemplate: template,
		})
	}
	return missing
}

type scaffoldsParams struct {
	MissingTargetsParams
	File     domain.EnvFile
	Template bool
}

// scaffolds answers on disk what scaffoldOf decides on the lines read.
func scaffolds(params scaffoldsParams) bool {
	if params.Strategy == domain.EnvStrategyExample {
		return params.Template
	}
	for _, dir := range []string{params.ParentPath, params.MainPath} {
		if dir != "" && fileExists(filepath.Join(dir, params.File.Target)) {
			return true
		}
	}
	return false
}
