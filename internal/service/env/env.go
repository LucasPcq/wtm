// Package env implements strategies for provisioning .env files in new worktrees.
package env

import (
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
func CopyEnvFiles(params CopyEnvFilesParams) error {
	for _, file := range params.Files {
		if err := copyEnvFile(params, file); err != nil {
			return err
		}
	}
	return nil
}

func copyEnvFile(params CopyEnvFilesParams, file domain.EnvFile) error {
	dst := filepath.Join(params.TargetDir, file.Target)

	switch params.Strategy {
	case domain.EnvStrategyExample:
		return copyFromExample(params.MainWorktreePath, dst, file)
	case domain.EnvStrategyMain:
		return copyFromDir(params.MainWorktreePath, dst, file.Target)
	case domain.EnvStrategyParent:
		return copyFromParent(params, dst, file.Target)
	default:
		return fmt.Errorf("unknown env strategy: %s", params.Strategy)
	}
}

// copyFromExample provisions a value file from its committed template. It uses the
// template pinned by detection (file.Template) when present, otherwise probes the
// known template candidates for file.Target in priority order.
func copyFromExample(mainPath string, dst string, file domain.EnvFile) error {
	src := resolveTemplateSrc(mainPath, file)
	if src == "" {
		fmt.Fprintf(os.Stderr, "warning: no template found for %s, skipping\n", file.Target)
		return nil
	}
	return copyFile(src, dst)
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

func copyFromDir(dir string, dst string, file string) error {
	src := filepath.Join(dir, file)
	if !fileExists(src) {
		fmt.Fprintf(os.Stderr, "warning: %s not found in %s, skipping\n", file, dir)
		return nil
	}
	return copyFile(src, dst)
}

func copyFromParent(params CopyEnvFilesParams, dst string, file string) error {
	if params.ParentWorktreePath != "" {
		src := filepath.Join(params.ParentWorktreePath, file)
		if fileExists(src) {
			return copyFile(src, dst)
		}
		fmt.Fprintf(os.Stderr, "warning: %s not found in parent worktree, falling back to main\n", file)
	}
	return copyFromDir(params.MainWorktreePath, dst, file)
}

func copyFile(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", dst, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}

	return nil
}

func fileExists(path string) bool {
	return fs.FileExists(path)
}
