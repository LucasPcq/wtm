package store

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvSource is one copy Copy makes.
type EnvSource struct {
	Src string
	Dst string
}

type CopySourcesParams struct {
	Request EnvCopy
	Exists  func(path string) bool
}

// CopySources is where each file of a copy comes from under its strategy, and
// what it could not find.
func CopySources(params CopySourcesParams) ([]EnvSource, []EnvWarning, error) {
	var sources []EnvSource
	var warnings []EnvWarning
	for _, file := range params.Request.Files {
		src, fileWarnings, err := envSource(envSourceParams{CopySourcesParams: params, File: file})
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, fileWarnings...)
		if src != "" {
			sources = append(sources, EnvSource{Src: src, Dst: filepath.Join(params.Request.TargetDir, file.Target)})
		}
	}
	return sources, warnings, nil
}

// TemplateSource is the template provisioning copies for file from dir, ""
// when none is on disk: the one detection pinned, else the first candidate.
func TemplateSource(dir string, file domain.EnvFile) string {
	return templateSource(templateSourceParams{Dir: dir, File: file, Exists: fs.FileExists})
}

type templateSourceParams struct {
	Dir    string
	File   domain.EnvFile
	Exists func(path string) bool
}

func templateSource(params templateSourceParams) string {
	if params.File.Template != "" {
		src := filepath.Join(params.Dir, params.File.Template)
		if params.Exists(src) {
			return src
		}
		return ""
	}
	for _, candidate := range rules.TemplateCandidates(params.File.Target) {
		src := filepath.Join(params.Dir, candidate)
		if params.Exists(src) {
			return src
		}
	}
	return ""
}

type envSourceParams struct {
	CopySourcesParams
	File domain.EnvFile
}

func envSource(params envSourceParams) (string, []EnvWarning, error) {
	request := params.Request
	target := params.File.Target
	switch request.Strategy {
	case domain.EnvStrategyExample:
		src := templateSource(templateSourceParams{Dir: request.MainWorktreePath, File: params.File, Exists: params.Exists})
		if src == "" {
			return "", []EnvWarning{NoTemplate{Target: target}}, nil
		}
		return src, nil, nil
	case domain.EnvStrategyMain:
		src, warnings := dirSource(dirSourceParams{Dir: request.MainWorktreePath, Target: target, Exists: params.Exists})
		return src, warnings, nil
	case domain.EnvStrategyParent:
		src, warnings := parentSource(params)
		return src, warnings, nil
	default:
		return "", nil, fmt.Errorf("unknown env strategy: %s", request.Strategy)
	}
}

type dirSourceParams struct {
	Dir    string
	Target string
	Exists func(path string) bool
}

func dirSource(params dirSourceParams) (string, []EnvWarning) {
	src := filepath.Join(params.Dir, params.Target)
	if !params.Exists(src) {
		return "", []EnvWarning{SourceMissing{Target: params.Target, Dir: params.Dir}}
	}
	return src, nil
}

func parentSource(params envSourceParams) (string, []EnvWarning) {
	request := params.Request
	main := dirSourceParams{Dir: request.MainWorktreePath, Target: params.File.Target, Exists: params.Exists}
	if request.ParentWorktreePath == "" {
		return dirSource(main)
	}
	src := filepath.Join(request.ParentWorktreePath, params.File.Target)
	if params.Exists(src) {
		return src, nil
	}
	fallback, warnings := dirSource(main)
	return fallback, append([]EnvWarning{ParentFallback{Target: params.File.Target}}, warnings...)
}
