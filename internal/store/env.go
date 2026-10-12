package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvDisk writes the .env files in place, each locked by fs.ScratchLockPath:
// a lock file beside it would show up in the user's checkout.
type EnvDisk struct{}

var _ EnvFiles = EnvDisk{}

// Copy provisions the files of a new worktree from the source its strategy
// names; a file with no source is skipped and reported, never an error.
func (EnvDisk) Copy(ctx context.Context, request EnvCopy) ([]EnvWarning, error) {
	sources, warnings, err := CopySources(CopySourcesParams{Request: request, Exists: fs.FileExists})
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		if err := copyEnv(ctx, source); err != nil {
			return nil, err
		}
	}
	return warnings, nil
}

// Edit reads the file under its lock and writes what edit makes of it, so an
// edit computed from the lines never overwrites a change made since. A file
// that renders as it was is not rewritten.
func (EnvDisk) Edit(ctx context.Context, edit EnvEdit) (bool, error) {
	changed := false
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     edit.Path,
		LockPath: fs.ScratchLockPath(edit.Path),
		Mutate: func(current []byte) (fs.Change, error) {
			next, write := EditEnv(EditEnvParams{Edit: edit, Current: current})
			if !write {
				return fs.Keep{}, nil
			}
			changed = true
			return fs.Write{Data: next}, nil
		},
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

type EditEnvParams struct {
	Edit EnvEdit
	// Current is the file's content, nil when it does not exist.
	Current []byte
}

// EditEnv is what an Edit writes over Current, and whether it writes at all.
func EditEnv(params EditEnvParams) ([]byte, bool) {
	if params.Current == nil && !params.Edit.Create {
		return nil, false
	}
	var lines []domain.EnvLine
	if params.Current != nil {
		lines = rules.ParseEnv(string(params.Current))
	}
	rendered := rules.RenderEnv(params.Edit.Edit(lines))
	if params.Current != nil && rendered == rules.RenderEnv(lines) {
		return nil, false
	}
	return []byte(rendered), true
}

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

func copyEnv(ctx context.Context, source EnvSource) error {
	data, err := os.ReadFile(source.Src)
	if err != nil {
		return fmt.Errorf("read %s: %w", source.Src, err)
	}
	_, err = fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     source.Dst,
		LockPath: fs.ScratchLockPath(source.Dst),
		Mutate:   func([]byte) (fs.Change, error) { return fs.Write{Data: data}, nil },
	})
	return err
}
