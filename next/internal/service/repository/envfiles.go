package repository

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// EnvFilesParams name the .env files to read for one worktree: the project's declared files, the worktree's directory, its parent worktree's ("" when its parent has none) and the main worktree's.
type EnvFilesParams struct {
	Files     []project.EnvFile
	Dir       string
	ParentDir string
	MainDir   string
}

// EnvFiles reads each of params' files for the worktree: its own copy, its template in the worktree, as the example strategy finds it in a checkout, the parent worktree's copy and the main worktree's.
func EnvFiles(reader EnvReader, params EnvFilesParams) ([]env.FileRead, error) {
	reads := make([]env.FileRead, 0, len(params.Files))
	for _, file := range params.Files {
		read := env.FileRead{File: file}
		at := []struct {
			lines *[]dotenv.Line
			dir   string
		}{{&read.Child, params.Dir}, {&read.Parent, params.ParentDir}, {&read.Main, params.MainDir}}
		for _, copy := range at {
			if copy.dir == "" {
				continue
			}
			lines, err := reader.Read(filepath.Join(copy.dir, file.Target))
			if err != nil {
				return nil, err
			}
			*copy.lines = lines
		}

		template, err := templateOf(reader, params.Dir, file)
		if err != nil {
			return nil, err
		}
		read.Template = template
		reads = append(reads, read)
	}

	return reads, nil
}

// templateOf reads the template of file in the worktree at dir; none when it has none.
func templateOf(reader EnvReader, dir string, file project.EnvFile) ([]dotenv.Line, error) {
	exists := func(path string) (bool, error) {
		lines, err := reader.Read(path)
		return lines != nil, err
	}
	sources, _, err := env.Sources(env.CopyRequest{Strategy: project.EnvExample, Files: []project.EnvFile{file}, MainPath: dir}, exists)
	if err != nil || len(sources) == 0 {
		return nil, err
	}

	return reader.Read(sources[0].From)
}
