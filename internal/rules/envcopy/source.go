// Package envcopy decides where each .env file of a new worktree is copied from.
package envcopy

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type Request struct {
	Strategy           domain.EnvStrategy
	Files              []domain.EnvFile
	TargetDir          string
	MainWorktreePath   string
	ParentWorktreePath string
}

// Source is one copy to make.
type Source struct {
	Src string
	Dst string
}

// Candidates are the paths whose presence decides the copy, for the caller to
// stat before Sources decides.
func Candidates(request Request) ([]string, error) {
	var paths []string
	for _, file := range request.Files {
		switch request.Strategy {
		case domain.EnvStrategyExample:
			paths = append(paths, TemplateCandidates(request.MainWorktreePath, file)...)
		case domain.EnvStrategyMain:
			paths = append(paths, filepath.Join(request.MainWorktreePath, file.Target))
		case domain.EnvStrategyParent:
			if request.ParentWorktreePath != "" {
				paths = append(paths, filepath.Join(request.ParentWorktreePath, file.Target))
			}
			paths = append(paths, filepath.Join(request.MainWorktreePath, file.Target))
		default:
			return nil, unknownStrategy(request.Strategy)
		}
	}
	return paths, nil
}

type SourcesParams struct {
	Request Request
	Present map[string]bool
}

// Sources is where each file comes from under its strategy, and what could
// not be found: the parent strategy falls back to the main checkout.
func Sources(params SourcesParams) ([]Source, []Warning, error) {
	var sources []Source
	var warnings []Warning
	for _, file := range params.Request.Files {
		src, fileWarnings, err := sourceOf(sourceOfParams{SourcesParams: params, File: file})
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, fileWarnings...)
		if src != "" {
			sources = append(sources, Source{Src: src, Dst: filepath.Join(params.Request.TargetDir, file.Target)})
		}
	}
	return sources, warnings, nil
}

type sourceOfParams struct {
	SourcesParams
	File domain.EnvFile
}

func sourceOf(params sourceOfParams) (string, []Warning, error) {
	request := params.Request
	target := params.File.Target
	switch request.Strategy {
	case domain.EnvStrategyExample:
		src := Template(TemplateCandidates(request.MainWorktreePath, params.File), params.Present)
		if src == "" {
			return "", []Warning{NoTemplate{Target: target}}, nil
		}
		return src, nil, nil
	case domain.EnvStrategyMain:
		src, warnings := dirSource(dirSourceParams{Dir: request.MainWorktreePath, Target: target, Present: params.Present})
		return src, warnings, nil
	case domain.EnvStrategyParent:
		src, warnings := parentSource(params)
		return src, warnings, nil
	default:
		return "", nil, unknownStrategy(request.Strategy)
	}
}

type dirSourceParams struct {
	Dir     string
	Target  string
	Present map[string]bool
}

func dirSource(params dirSourceParams) (string, []Warning) {
	src := filepath.Join(params.Dir, params.Target)
	if !params.Present[src] {
		return "", []Warning{SourceMissing{Target: params.Target, Dir: params.Dir}}
	}
	return src, nil
}

func parentSource(params sourceOfParams) (string, []Warning) {
	request := params.Request
	main := dirSourceParams{Dir: request.MainWorktreePath, Target: params.File.Target, Present: params.Present}
	if request.ParentWorktreePath == "" {
		return dirSource(main)
	}
	src := filepath.Join(request.ParentWorktreePath, params.File.Target)
	if params.Present[src] {
		return src, nil
	}
	fallback, warnings := dirSource(main)
	return fallback, append([]Warning{ParentFallback{Target: params.File.Target}}, warnings...)
}

func unknownStrategy(strategy domain.EnvStrategy) error {
	return fmt.Errorf("unknown env strategy: %s", strategy)
}
