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

// Sources returns the files to copy for request, asking exists about the paths the strategy reads, and a warning for each declared file that is copied from a fallback or not at all. It fails when exists fails or the strategy is unknown.
func Sources(request CopyRequest, exists func(path string) (bool, error)) ([]Source, []CopyWarning, error) {
	var sources []Source
	var warnings []CopyWarning
	for _, file := range request.Files {
		from, fileWarnings, err := sourceOf(request, exists, file)
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
func sourceOf(request CopyRequest, exists func(path string) (bool, error), file project.EnvFile) (string, []CopyWarning, error) {
	switch request.Strategy {
	case project.EnvExample:
		from, err := template(templateCandidates(request, file), exists)
		if err != nil || from != "" {
			return from, nil, err
		}
		return "", []CopyWarning{NoTemplate{Target: file.Target}}, nil
	case project.EnvMain:
		return inDir(inDirParams{Dir: request.MainPath, Target: file.Target, Exists: exists})
	case project.EnvParent:
		return fromParent(request, exists, file)
	}
	return "", nil, unknownStrategy(request.Strategy)
}

type inDirParams struct {
	Dir    string
	Target string
	Exists func(path string) (bool, error)
}

// inDir returns Target under Dir when it exists, else nothing and a SourceMissing warning.
func inDir(params inDirParams) (string, []CopyWarning, error) {
	path := filepath.Join(params.Dir, params.Target)
	found, err := params.Exists(path)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "", []CopyWarning{SourceMissing{Target: params.Target, Dir: params.Dir}}, nil
	}
	return path, nil, nil
}

// fromParent returns the parent worktree's copy of file when it exists, else the main worktree's with a ParentFallback warning.
func fromParent(request CopyRequest, exists func(path string) (bool, error), file project.EnvFile) (string, []CopyWarning, error) {
	main := inDirParams{Dir: request.MainPath, Target: file.Target, Exists: exists}
	if request.ParentPath == "" {
		return inDir(main)
	}

	path := filepath.Join(request.ParentPath, file.Target)
	found, err := exists(path)
	if err != nil {
		return "", nil, err
	}
	if found {
		return path, nil, nil
	}

	fallback, warnings, err := inDir(main)
	return fallback, append([]CopyWarning{ParentFallback{Target: file.Target}}, warnings...), err
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

// template returns the first of candidates that exists, empty when none does.
func template(candidates []string, exists func(path string) (bool, error)) (string, error) {
	for _, candidate := range candidates {
		found, err := exists(candidate)
		if err != nil {
			return "", err
		}
		if found {
			return candidate, nil
		}
	}
	return "", nil
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
