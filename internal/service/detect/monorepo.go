package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// WorkspacePackages resolves a monorepo's workspace declaration to the package
// directories it actually selects, relative to projectDir and slash-separated.
// Both declarations are read: pnpm-workspace.yaml, and the `workspaces` field
// npm, yarn and bun put in the root manifest. Returns nil when the project
// declares no workspace.
func WorkspacePackages(projectDir string) []string {
	patterns := rules.ParseWorkspacePatterns(workspaceDeclaration(projectDir))
	if len(patterns.Include) == 0 {
		return nil
	}

	var packages []string
	for _, dir := range packageDirs(projectDir) {
		if rules.SelectsWorkspace(patterns, dir) {
			packages = append(packages, dir)
		}
	}

	sort.Strings(packages)
	return packages
}

// workspaceDeclaration reads the patterns both ecosystems declare. A repo
// carrying pnpm-workspace.yaml *and* a `workspaces` field is not a conflict to
// arbitrate: whichever tool it was written for, both name real packages.
func workspaceDeclaration(projectDir string) []string {
	patterns := pnpmWorkspacePatterns(projectDir)
	return append(patterns, manifestWorkspacePatterns(projectDir)...)
}

func pnpmWorkspacePatterns(projectDir string) []string {
	data, err := os.ReadFile(filepath.Join(projectDir, domain.PnpmWorkspaceName))
	if err != nil {
		return nil
	}
	return parsePnpmWorkspace(string(data))
}

// manifestWorkspaces covers both shapes npm accepts: a bare array, and the
// object yarn adds `nohoist` to.
type manifestWorkspaces struct {
	Workspaces json.RawMessage `json:"workspaces"`
}

func manifestWorkspacePatterns(projectDir string) []string {
	data, err := os.ReadFile(filepath.Join(projectDir, domain.PackageJSONName))
	if err != nil {
		return nil
	}

	var manifest manifestWorkspaces
	if unmarshalErr := json.Unmarshal(data, &manifest); unmarshalErr != nil || len(manifest.Workspaces) == 0 {
		return nil
	}

	var list []string
	if json.Unmarshal(manifest.Workspaces, &list) == nil {
		return list
	}

	var object struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(manifest.Workspaces, &object) == nil {
		return object.Packages
	}
	return nil
}

// packageDirs walks the project for every directory holding a manifest, bounded
// by domain.WorkspaceScanMaxDepth. A pattern is matched against what is on disk
// rather than expanded into paths: "**" selects any depth, which no expansion
// of a glob can enumerate ahead of the walk.
func packageDirs(projectDir string) []string {
	var dirs []string
	_ = filepath.WalkDir(projectDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return filepath.SkipDir
		}
		if rel == "." {
			return nil
		}
		if rules.IsScanIgnoredDir(entry.Name()) {
			return filepath.SkipDir
		}

		slashed := filepath.ToSlash(rel)
		if strings.Count(slashed, "/")+1 >= domain.WorkspaceScanMaxDepth {
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(path, domain.PackageJSONName)); statErr == nil {
			dirs = append(dirs, slashed)
		}
		return nil
	})
	return dirs
}

// parsePnpmWorkspace extracts package patterns from pnpm-workspace.yaml content.
// Handles the simple format: packages:\n  - "pattern"\n  - 'pattern'\n  - pattern
func parsePnpmWorkspace(content string) []string {
	var patterns []string
	inPackages := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "packages:" {
			inPackages = true
			continue
		}

		if inPackages {
			if !strings.HasPrefix(trimmed, "- ") {
				break
			}
			pattern := strings.TrimPrefix(trimmed, "- ")
			pattern = strings.Trim(pattern, "\"'")
			if pattern != "" {
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}
