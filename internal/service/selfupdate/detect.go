package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Install struct {
	Method domain.InstallMethod
	// BinaryPath is the resolved path, so a replacement targets the real file
	// rather than a symlink pointing at it.
	BinaryPath string
}

func DetectInstall(ctx context.Context, version string) Install {
	execPath, err := os.Executable()
	if err != nil {
		return Install{Method: domain.InstallStandalone}
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolved = execPath
	}

	return Install{
		Method: rules.ClassifyInstall(rules.ClassifyInstallParams{
			ResolvedPath: resolved,
			// Lazy: resolving it shells out to `go env`, which a Homebrew or
			// source install never needs to pay for.
			GoBinDir: func() string { return goBinDir(ctx) },
			Version:  version,
		}),
		BinaryPath: resolved,
	}
}

// goBinDir returns the go-install destination with symlinks resolved: it is
// compared against an already-resolved executable path, and on macOS /tmp alone
// is enough to make the two differ.
func goBinDir(ctx context.Context) string {
	return resolveDir(rawGoBinDir(ctx))
}

func resolveDir(dir string) string {
	if dir == "" {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}

	return resolved
}

func rawGoBinDir(ctx context.Context) string {
	if bin := os.Getenv("GOBIN"); bin != "" {
		return bin
	}

	out, err := infra.Command(ctx, "go", "env", "GOBIN", "GOPATH").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 2 {
			if lines[0] != "" {
				return lines[0]
			}
			if lines[1] != "" {
				return filepath.Join(lines[1], "bin")
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "go", "bin")
}
