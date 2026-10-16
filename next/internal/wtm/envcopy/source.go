// Package envcopy decides where each .env file of a new worktree is copied from.
package envcopy

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

type Request struct {
	Strategy   project.EnvStrategy
	Files      []project.EnvFile
	TargetDir  string
	MainPath   string
	ParentPath string
}

type Source struct {
	From string
	To   string
}

func Candidates(request Request) ([]string, error) {
	var paths []string
	for _, file := range request.Files {
		switch request.Strategy {
		case project.EnvExample:
			paths = append(paths, templateCandidates(requestFile{Request: request, File: file})...)
		case project.EnvMain:
			paths = append(paths, filepath.Join(request.MainPath, file.Target))
		case project.EnvParent:
			paths = append(paths, parentCandidates(requestFile{Request: request, File: file})...)
		default:
			return nil, unknownStrategy(request.Strategy)
		}
	}
	return paths, nil
}

type requestFile struct {
	Request Request
	File    project.EnvFile
}

func parentCandidates(params requestFile) []string {
	main := filepath.Join(params.Request.MainPath, params.File.Target)
	if params.Request.ParentPath == "" {
		return []string{main}
	}
	return []string{filepath.Join(params.Request.ParentPath, params.File.Target), main}
}

type SourcesParams struct {
	Request Request
	Present map[string]bool
}

func Sources(params SourcesParams) ([]Source, []Warning, error) {
	var sources []Source
	var warnings []Warning
	for _, file := range params.Request.Files {
		from, fileWarnings, err := sourceOf(sourceOfParams{SourcesParams: params, File: file})
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, fileWarnings...)
		if from != "" {
			sources = append(sources, Source{From: from, To: filepath.Join(params.Request.TargetDir, file.Target)})
		}
	}
	return sources, warnings, nil
}

type sourceOfParams struct {
	SourcesParams
	File project.EnvFile
}

func sourceOf(params sourceOfParams) (string, []Warning, error) {
	request := params.Request
	switch request.Strategy {
	case project.EnvExample:
		from := template(templateParams{Candidates: templateCandidates(requestFile{Request: request, File: params.File}), Present: params.Present})
		if from == "" {
			return "", []Warning{NoTemplate{Target: params.File.Target}}, nil
		}
		return from, nil, nil
	case project.EnvMain:
		from, warnings := inDir(inDirParams{Dir: request.MainPath, Target: params.File.Target, Present: params.Present})
		return from, warnings, nil
	case project.EnvParent:
		from, warnings := fromParent(params)
		return from, warnings, nil
	}
	return "", nil, unknownStrategy(request.Strategy)
}

type inDirParams struct {
	Dir     string
	Target  string
	Present map[string]bool
}

func inDir(params inDirParams) (string, []Warning) {
	path := filepath.Join(params.Dir, params.Target)
	if !params.Present[path] {
		return "", []Warning{SourceMissing{Target: params.Target, Dir: params.Dir}}
	}
	return path, nil
}

func fromParent(params sourceOfParams) (string, []Warning) {
	main := inDirParams{Dir: params.Request.MainPath, Target: params.File.Target, Present: params.Present}
	if params.Request.ParentPath == "" {
		return inDir(main)
	}
	path := filepath.Join(params.Request.ParentPath, params.File.Target)
	if params.Present[path] {
		return path, nil
	}
	fallback, warnings := inDir(main)
	return fallback, append([]Warning{ParentFallback{Target: params.File.Target}}, warnings...)
}

func unknownStrategy(strategy project.EnvStrategy) error {
	return fmt.Errorf("unknown env strategy: %q", strategy)
}
