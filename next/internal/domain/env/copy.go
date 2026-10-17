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
			paths = append(paths, templateCandidates(request, file)...)
		case project.EnvMain:
			paths = append(paths, filepath.Join(request.MainPath, file.Target))
		case project.EnvParent:
			paths = append(paths, parentCandidates(request, file)...)
		default:
			return nil, unknownStrategy(request.Strategy)
		}
	}
	return paths, nil
}

func parentCandidates(request CopyRequest, file project.EnvFile) []string {
	main := filepath.Join(request.MainPath, file.Target)
	if request.ParentPath == "" {
		return []string{main}
	}
	return []string{filepath.Join(request.ParentPath, file.Target), main}
}

func Sources(request CopyRequest, exists func(path string) (bool, error)) ([]Source, []CopyWarning, error) {
	paths, err := candidates(request)
	if err != nil {
		return nil, nil, err
	}
	present := map[string]bool{}
	for _, path := range paths {
		found, err := exists(path)
		if err != nil {
			return nil, nil, err
		}
		present[path] = found
	}
	return sourcesOf(request, present)
}

func sourcesOf(request CopyRequest, present map[string]bool) ([]Source, []CopyWarning, error) {
	var sources []Source
	var warnings []CopyWarning
	for _, file := range request.Files {
		from, fileWarnings, err := sourceOf(request, present, file)
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, fileWarnings...)
		if from != "" {
			sources = append(sources, Source{From: from, To: filepath.Join(request.TargetDir, file.Target)})
		}
	}
	return sources, warnings, nil
}

func sourceOf(request CopyRequest, present map[string]bool, file project.EnvFile) (string, []CopyWarning, error) {
	switch request.Strategy {
	case project.EnvExample:
		from := template(templateCandidates(request, file), present)
		if from == "" {
			return "", []CopyWarning{NoTemplate{Target: file.Target}}, nil
		}
		return from, nil, nil
	case project.EnvMain:
		from, warnings := inDir(inDirParams{Dir: request.MainPath, Target: file.Target, Present: present})
		return from, warnings, nil
	case project.EnvParent:
		from, warnings := fromParent(request, present, file)
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

func fromParent(request CopyRequest, present map[string]bool, file project.EnvFile) (string, []CopyWarning) {
	main := inDirParams{Dir: request.MainPath, Target: file.Target, Present: present}
	if request.ParentPath == "" {
		return inDir(main)
	}
	path := filepath.Join(request.ParentPath, file.Target)
	if present[path] {
		return path, nil
	}
	fallback, warnings := inDir(main)
	return fallback, append([]CopyWarning{ParentFallback{Target: file.Target}}, warnings...)
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

func templateCandidates(request CopyRequest, file project.EnvFile) []string {
	dir := request.MainPath
	if file.Template != "" {
		return []string{filepath.Join(dir, file.Template)}
	}
	candidates := make([]string, 0, len(templateSuffixes))
	for _, suffix := range templateSuffixes {
		candidates = append(candidates, filepath.Join(dir, file.Target+suffix))
	}
	return candidates
}

func template(candidates []string, present map[string]bool) string {
	for _, candidate := range candidates {
		if present[candidate] {
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
