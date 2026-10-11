// Package env implements strategies for provisioning .env files in new worktrees.
package env

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// CopyEnvFilesParams holds inputs for provisioning .env files.
type CopyEnvFilesParams struct {
	Strategy           domain.EnvStrategy
	Files              []domain.EnvFile
	TargetDir          string
	MainWorktreePath   string
	ParentWorktreePath string
}

// CopyEnvFiles provisions .env files into a new worktree according to the chosen strategy.
func CopyEnvFiles(ctx context.Context, params CopyEnvFilesParams) error {
	for _, file := range params.Files {
		if err := copyEnvFile(ctx, params, file); err != nil {
			return err
		}
	}
	return nil
}

func copyEnvFile(ctx context.Context, params CopyEnvFilesParams, file domain.EnvFile) error {
	dst := filepath.Join(params.TargetDir, file.Target)

	switch params.Strategy {
	case domain.EnvStrategyExample:
		return copyFromExample(ctx, params.MainWorktreePath, dst, file)
	case domain.EnvStrategyMain:
		return copyFromDir(ctx, params.MainWorktreePath, dst, file.Target)
	case domain.EnvStrategyParent:
		return copyFromParent(ctx, params, dst, file.Target)
	default:
		return fmt.Errorf("unknown env strategy: %s", params.Strategy)
	}
}

// copyFromExample provisions a value file from its committed template. It uses the
// template pinned by detection (file.Template) when present, otherwise probes the
// known template candidates for file.Target in priority order.
func copyFromExample(ctx context.Context, mainPath string, dst string, file domain.EnvFile) error {
	src := resolveTemplateSrc(mainPath, file)
	if src == "" {
		fmt.Fprintf(os.Stderr, "warning: no template found for %s, skipping\n", file.Target)
		return nil
	}
	return copyFile(ctx, copyFileParams{Src: src, Dst: dst})
}

// resolveTemplateSrc returns the absolute path of the template to copy for file,
// or "" when none exists on disk. Both Target and Template are repo-relative.
func resolveTemplateSrc(mainPath string, file domain.EnvFile) string {
	if file.Template != "" {
		src := filepath.Join(mainPath, file.Template)
		if fileExists(src) {
			return src
		}
		return ""
	}
	for _, candidate := range rules.TemplateCandidates(file.Target) {
		src := filepath.Join(mainPath, candidate)
		if fileExists(src) {
			return src
		}
	}
	return ""
}

func copyFromDir(ctx context.Context, dir string, dst string, file string) error {
	src := filepath.Join(dir, file)
	if !fileExists(src) {
		fmt.Fprintf(os.Stderr, "warning: %s not found in %s, skipping\n", file, dir)
		return nil
	}
	return copyFile(ctx, copyFileParams{Src: src, Dst: dst})
}

func copyFromParent(ctx context.Context, params CopyEnvFilesParams, dst string, file string) error {
	if params.ParentWorktreePath != "" {
		src := filepath.Join(params.ParentWorktreePath, file)
		if fileExists(src) {
			return copyFile(ctx, copyFileParams{Src: src, Dst: dst})
		}
		fmt.Fprintf(os.Stderr, "warning: %s not found in parent worktree, falling back to main\n", file)
	}
	return copyFromDir(ctx, params.MainWorktreePath, dst, file)
}

type copyFileParams struct {
	Src string
	Dst string
}

func copyFile(ctx context.Context, params copyFileParams) error {
	data, err := os.ReadFile(params.Src)
	if err != nil {
		return fmt.Errorf("read %s: %w", params.Src, err)
	}

	_, err = fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     params.Dst,
		LockPath: fs.ScratchLockPath(params.Dst),
		Mutate:   func([]byte) (fs.Change, error) { return fs.Write{Data: data}, nil },
	})
	return err
}

func fileExists(path string) bool {
	return fs.FileExists(path)
}
