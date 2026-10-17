package env

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A CopyRequest asks which .env files a new worktree at TargetDir copies, following the project's env strategy over its declared files. ParentPath is the worktree of the branch it starts from, empty when that branch has none.
type CopyRequest struct {
	Strategy   project.EnvStrategy
	Files      []project.EnvFile
	TargetDir  string
	MainPath   string
	ParentPath string
}

// A Source is one file to copy: the existing file From to the path To in the new worktree.
type Source struct {
	From string
	To   string
}

// candidates returns every path the strategy may copy from, so their existence can be checked before deciding; it fails on a strategy it does not know.
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

// parentCandidates returns the parent worktree's copy of file, then the main worktree's it falls back to.
func parentCandidates(request CopyRequest, file project.EnvFile) []string {
	main := filepath.Join(request.MainPath, file.Target)
	if request.ParentPath == "" {
		return []string{main}
	}
	return []string{filepath.Join(request.ParentPath, file.Target), main}
}

// Sources returns the files to copy for request, asking exists about each path the strategy may read, and a warning for each declared file that is copied from a fallback or not at all. It fails when exists fails or the strategy is unknown.
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

// sourcesOf decides the sources of every declared file from the paths known to be present.
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

// sourceOf returns the path file is copied from, empty when it is copied from nowhere, and the warnings that choice raises.
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

// inDir returns Target under Dir when it is present, else nothing and a SourceMissing warning.
func inDir(params inDirParams) (string, []CopyWarning) {
	path := filepath.Join(params.Dir, params.Target)
	if !params.Present[path] {
		return "", []CopyWarning{SourceMissing{Target: params.Target, Dir: params.Dir}}
	}
	return path, nil
}

// fromParent returns the parent worktree's copy of file when present, else the main worktree's with a ParentFallback warning.
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

// Copied is what copying a worktree's .env files did: the files written, the warnings raised, and whether any file was written without holding its lock.
type Copied struct {
	Files    []string
	Warnings []CopyWarning
	Unlocked bool
}

// templateSuffixes are the suffixes a template of a .env file is looked for under, in order of preference.
var templateSuffixes = [...]string{".example", ".dist", ".sample", ".template", ".tmpl"}

// templateCandidates returns the template file declares, else every conventional template name of its target in the main worktree.
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

// template returns the first of candidates that is present, empty when none is.
func template(candidates []string, present map[string]bool) string {
	for _, candidate := range candidates {
		if present[candidate] {
			return candidate
		}
	}
	return ""
}

// ParentFallbackParams describes a worktree to create: the project's env strategy, whether it declares .env files, and whether the branch it starts from has a worktree.
type ParentFallbackParams struct {
	Strategy          project.EnvStrategy
	HasFiles          bool
	SourceHasWorktree bool
}

// ParentFallsBackToMain reports whether the parent strategy will copy from the main worktree because the branch the new worktree starts from has none.
func ParentFallsBackToMain(params ParentFallbackParams) bool {
	return params.HasFiles && params.Strategy == project.EnvParent && !params.SourceHasWorktree
}

//go-sumtype:decl CopyWarning

// A CopyWarning is a declared .env file copied from a fallback or not copied at all: NoTemplate, SourceMissing or ParentFallback.
type CopyWarning interface{ warning() }

// NoTemplate warns that the example strategy found no template for Target, which is not copied.
type NoTemplate struct{ Target string }

// SourceMissing warns that Target is not in Dir, the worktree it would be copied from, so it is not copied.
type SourceMissing struct {
	Target string
	Dir    string
}

// ParentFallback warns that the parent worktree has no Target, which is copied from the main worktree instead.
type ParentFallback struct{ Target string }

func (NoTemplate) warning()     {}
func (SourceMissing) warning()  {}
func (ParentFallback) warning() {}
