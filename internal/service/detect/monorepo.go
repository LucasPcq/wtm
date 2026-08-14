package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// Monorepo classifies how projectDir installs its dependencies. A root workspace
// declaration short-circuits: sub-directory lockfiles inside a workspace are
// managed by the root install and are never scanned for.
func Monorepo(projectDir string) domain.MonorepoLayout {
	if WorkspaceDeclaration(projectDir) {
		return domain.MonorepoLayout{
			Kind:              domain.MonorepoKindWorkspace,
			WorkspacePackages: WorkspacePackages(projectDir),
		}
	}

	subs := SubProjects(SubProjectsParams{Root: projectDir, MaxDepth: domain.MonorepoScanMaxDepth})
	if len(subs) == 0 {
		return domain.MonorepoLayout{Kind: domain.MonorepoKindNone}
	}

	return domain.MonorepoLayout{Kind: domain.MonorepoKindIndependent, SubProjects: subs}
}

// WorkspaceDeclaration reports whether projectDir declares a workspace at its
// root — i.e. whether a single root install covers every package. Recognizes
// pnpm-workspace.yaml, go.work, a package.json "workspaces" field (npm/yarn/bun
// array form and the yarn-v1 {"packages": [...]} object form), and the
// turbo.json / nx.json / lerna.json monorepo markers.
func WorkspaceDeclaration(projectDir string) bool {
	if infra.FileExists(filepath.Join(projectDir, domain.WorkspaceFilePnpm)) {
		return true
	}
	if infra.FileExists(filepath.Join(projectDir, domain.WorkspaceFileGo)) {
		return true
	}
	if len(packageJSONWorkspaces(projectDir)) > 0 {
		return true
	}

	// Turbo/Nx/Lerna always sit on top of a workspace. They are the safety net for
	// a declaration the parsers above could not read.
	for _, marker := range []string{domain.WorkspaceFileTurbo, domain.WorkspaceFileNx, domain.WorkspaceFileLerna} {
		if infra.FileExists(filepath.Join(projectDir, marker)) {
			return true
		}
	}

	return false
}

// WorkspacePackages resolves the workspace package patterns declared at the
// project root — pnpm-workspace.yaml first, else package.json "workspaces" — to
// directories that actually contain a package.json. Paths are relative to
// projectDir, slash-separated, deduplicated and sorted. Returns nil when no
// workspace patterns are declared.
func WorkspacePackages(projectDir string) []string {
	patterns := workspacePatterns(projectDir)
	if len(patterns) == 0 {
		return nil
	}

	seen := map[string]bool{}
	for _, pattern := range patterns {
		for _, dir := range expandWorkspacePattern(expandPatternParams{Root: projectDir, Pattern: pattern}) {
			seen[dir] = true
		}
	}

	packages := make([]string, 0, len(seen))
	for dir := range seen {
		if !infra.FileExists(filepath.Join(projectDir, dir, domain.PackageJSONFile)) {
			continue
		}
		packages = append(packages, dir)
	}

	sort.Strings(packages)
	if len(packages) == 0 {
		return nil
	}
	return packages
}

// workspacePatterns returns the raw package patterns declared at the root,
// preferring pnpm-workspace.yaml over the package.json "workspaces" field.
func workspacePatterns(projectDir string) []string {
	data, err := os.ReadFile(filepath.Join(projectDir, domain.WorkspaceFilePnpm))
	if err == nil {
		return parsePnpmWorkspace(string(data))
	}
	return packageJSONWorkspaces(projectDir)
}

// packageJSONWorkspaces reads the root package.json "workspaces" field, accepting
// both the array form used by npm, yarn v2+ and bun, and the legacy yarn v1
// object form {"packages": [...]}. Returns nil when absent or empty.
func packageJSONWorkspaces(projectDir string) []string {
	data, err := os.ReadFile(filepath.Join(projectDir, domain.PackageJSONFile))
	if err != nil {
		return nil
	}

	var manifest struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	if len(manifest.Workspaces) == 0 {
		return nil
	}

	var list []string
	if err := json.Unmarshal(manifest.Workspaces, &list); err == nil {
		return list
	}

	var legacy struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(manifest.Workspaces, &legacy); err == nil {
		return legacy.Packages
	}

	return nil
}

// expandPatternParams holds the inputs for expandWorkspacePattern.
type expandPatternParams struct {
	Root    string
	Pattern string
}

// expandWorkspacePattern resolves one workspace pattern to slash-separated
// directories relative to Root. filepath.Glob has no "**" support, so a globstar
// pattern is treated as "every directory at any depth below the literal prefix"
// and the caller's package.json filter prunes the non-packages — which is what
// pnpm's "packages/**" means in practice.
func expandWorkspacePattern(params expandPatternParams) []string {
	if strings.Contains(params.Pattern, "**") {
		return expandGlobstar(params)
	}

	matches, err := filepath.Glob(filepath.Join(params.Root, params.Pattern))
	if err != nil {
		return nil
	}

	var dirs []string
	for _, m := range matches {
		rel, ok := relativeDir(relativeDirParams{Root: params.Root, Target: m})
		if !ok {
			continue
		}
		dirs = append(dirs, rel)
	}
	return dirs
}

// expandGlobstar walks every directory below the pattern's literal prefix. A
// suffix after the "**" (e.g. "packages/**/src") is matched best-effort against
// the directory's own name.
func expandGlobstar(params expandPatternParams) []string {
	prefix, suffix, _ := strings.Cut(params.Pattern, "**")
	prefix = strings.Trim(prefix, "/")
	suffix = strings.Trim(suffix, "/")

	base := filepath.Join(params.Root, filepath.FromSlash(prefix))
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return nil
	}

	var dirs []string
	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == base {
			return nil
		}
		if scanSkipDirs[d.Name()] {
			return filepath.SkipDir
		}

		rel, ok := relativeDir(relativeDirParams{Root: params.Root, Target: path})
		if !ok {
			return filepath.SkipDir
		}
		if len(strings.Split(rel, "/")) > domain.MonorepoScanMaxDepth {
			return filepath.SkipDir
		}
		if suffix != "" {
			if matched, matchErr := filepath.Match(suffix, d.Name()); matchErr != nil || !matched {
				return nil
			}
		}

		dirs = append(dirs, rel)
		return nil
	})

	return dirs
}

// relativeDirParams holds the inputs for relativeDir.
type relativeDirParams struct {
	Root   string
	Target string
}

// relativeDir renders Target relative to Root in slash form, reporting false when
// Target is not an existing directory below Root.
func relativeDir(params relativeDirParams) (string, bool) {
	info, err := os.Stat(params.Target)
	if err != nil || !info.IsDir() {
		return "", false
	}
	rel, err := filepath.Rel(params.Root, params.Target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// parsePnpmWorkspace extracts package patterns from pnpm-workspace.yaml content.
// Handles the block-sequence format under a "packages:" key, tolerating inline
// comments, blank lines and variable dash spacing. Negated ("!") patterns are
// skipped; the list ends at the next top-level key.
func parsePnpmWorkspace(content string) []string {
	var patterns []string
	inPackages := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(stripYAMLComment(line))

		if trimmed == "" {
			continue
		}

		if !inPackages {
			if trimmed == "packages:" {
				inPackages = true
			}
			continue
		}

		if !strings.HasPrefix(trimmed, "-") {
			break
		}

		pattern := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		pattern = strings.Trim(pattern, "\"'")
		if pattern != "" && !strings.HasPrefix(pattern, "!") {
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

// stripYAMLComment drops a trailing "# …" comment. Quoted "#" characters are rare
// in workspace patterns and not worth a full YAML parser here.
func stripYAMLComment(line string) string {
	idx := strings.Index(line, "#")
	if idx < 0 {
		return line
	}
	return line[:idx]
}
