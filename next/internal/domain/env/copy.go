package env

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

type CopyRequest struct {
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

func candidates(request CopyRequest) ([]string, error) {
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
	Request CopyRequest
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
	Request CopyRequest
	Exists  func(path string) (bool, error)
}

func Sources(params SourcesParams) ([]Source, []CopyWarning, error) {
	paths, err := candidates(params.Request)
	if err != nil {
		return nil, nil, err
	}
	present := map[string]bool{}
	for _, path := range paths {
		exists, err := params.Exists(path)
		if err != nil {
			return nil, nil, err
		}
		present[path] = exists
	}
	return sourcesOf(presentParams{Request: params.Request, Present: present})
}

type presentParams struct {
	Request CopyRequest
	Present map[string]bool
}

func sourcesOf(params presentParams) ([]Source, []CopyWarning, error) {
	var sources []Source
	var warnings []CopyWarning
	for _, file := range params.Request.Files {
		from, fileWarnings, err := sourceOf(sourceOfParams{presentParams: params, File: file})
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
	presentParams
	File project.EnvFile
}

func sourceOf(params sourceOfParams) (string, []CopyWarning, error) {
	request := params.Request
	switch request.Strategy {
	case project.EnvExample:
		from := template(templateParams{Candidates: templateCandidates(requestFile{Request: request, File: params.File}), Present: params.Present})
		if from == "" {
			return "", []CopyWarning{NoTemplate{Target: params.File.Target}}, nil
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

func inDir(params inDirParams) (string, []CopyWarning) {
	path := filepath.Join(params.Dir, params.Target)
	if !params.Present[path] {
		return "", []CopyWarning{SourceMissing{Target: params.Target, Dir: params.Dir}}
	}
	return path, nil
}

func fromParent(params sourceOfParams) (string, []CopyWarning) {
	main := inDirParams{Dir: params.Request.MainPath, Target: params.File.Target, Present: params.Present}
	if params.Request.ParentPath == "" {
		return inDir(main)
	}
	path := filepath.Join(params.Request.ParentPath, params.File.Target)
	if params.Present[path] {
		return path, nil
	}
	fallback, warnings := inDir(main)
	return fallback, append([]CopyWarning{ParentFallback{Target: params.File.Target}}, warnings...)
}

func unknownStrategy(strategy project.EnvStrategy) error {
	return fmt.Errorf("unknown env strategy: %s", strategy)
}

type Copied struct {
	Files    []string
	Warnings []CopyWarning
	Unlocked bool
}

var templateSuffixes = [...]string{".example", ".dist", ".sample", ".template", ".tmpl"}

func templateCandidates(params requestFile) []string {
	dir, file := params.Request.MainPath, params.File
	if file.Template != "" {
		return []string{filepath.Join(dir, file.Template)}
	}
	candidates := make([]string, 0, len(templateSuffixes))
	for _, suffix := range templateSuffixes {
		candidates = append(candidates, filepath.Join(dir, file.Target+suffix))
	}
	return candidates
}

type templateParams struct {
	Candidates []string
	Present    map[string]bool
}

func template(params templateParams) string {
	for _, candidate := range params.Candidates {
		if params.Present[candidate] {
			return candidate
		}
	}
	return ""
}

type ParentFallbackParams struct {
	Strategy          project.EnvStrategy
	HasFiles          bool
	SourceHasWorktree bool
}

func ParentFallsBackToMain(params ParentFallbackParams) bool {
	return params.HasFiles && params.Strategy == project.EnvParent && !params.SourceHasWorktree
}

//go-sumtype:decl CopyWarning

type CopyWarning interface{ warning() }

type NoTemplate struct{ Target string }

type SourceMissing struct {
	Target string
	Dir    string
}

type ParentFallback struct{ Target string }

func (NoTemplate) warning()     {}
func (SourceMissing) warning()  {}
func (ParentFallback) warning() {}
