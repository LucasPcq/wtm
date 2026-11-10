package detect

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// placeDepth is how deep a workspace package is looked for.
const placeDepth = 6

// manifest is what a package.json says that wtm reads.
type manifest struct {
	Name           string            `json:"name"`
	PackageManager string            `json:"packageManager"`
	Scripts        map[string]string `json:"scripts"`
	Workspaces     json.RawMessage   `json:"workspaces"`
}

// Workspace is what the repository's package manifests say: its package manager, its packageManager field first, else the first lockfile at its root; then the root's package.json and those of its workspace packages, as pnpm-workspace.yaml and package.json's workspaces select them, each with its scripts. What cannot be read is left out.
func (r Repository) Workspace() project.Workspace {
	var workspace project.Workspace
	root, hasRoot := r.manifest(".")
	if manager, found := project.ManagerOf(root.PackageManager); found {
		workspace.Manager, workspace.ManagerFrom = manager, "package.json"
	} else {
		for _, lock := range project.Lockfiles() {
			if _, err := os.Stat(filepath.Join(r.Dir, lock.File)); err == nil {
				workspace.Manager, workspace.ManagerFrom = lock.Manager, lock.File
				break
			}
		}
	}

	if hasRoot {
		workspace.Places = append(workspace.Places, place(".", root))
	}
	patterns := r.patterns(root)
	for _, dir := range r.packageDirs() {
		if !selected(patterns, dir) {
			continue
		}
		if found, ok := r.manifest(dir); ok {
			workspace.Places = append(workspace.Places, place(dir, found))
		}
	}
	return workspace
}

func place(dir string, found manifest) project.Place {
	name := found.Name
	if _, unscoped, scoped := strings.Cut(name, "/"); scoped && strings.HasPrefix(name, "@") {
		name = unscoped
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	scripts := make([]project.Script, 0, len(found.Scripts))
	for _, script := range slices.Sorted(maps.Keys(found.Scripts)) {
		scripts = append(scripts, project.Script{Name: script, Cmd: found.Scripts[script]})
	}
	return project.Place{Dir: dir, Name: name, Scripts: scripts}
}

func (r Repository) manifest(dir string) (manifest, bool) {
	data, err := os.ReadFile(filepath.Join(r.Dir, dir, "package.json"))
	if err != nil {
		return manifest{}, false
	}
	var found manifest
	return found, json.Unmarshal(data, &found) == nil
}

// patterns are the workspace patterns pnpm-workspace.yaml and package.json's workspaces declare, an array or yarn's object of packages.
func (r Repository) patterns(root manifest) []string {
	var patterns []string
	if data, err := os.ReadFile(filepath.Join(r.Dir, "pnpm-workspace.yaml")); err == nil {
		var pnpm struct {
			Packages []string `yaml:"packages"`
		}
		if yaml.Unmarshal(data, &pnpm) == nil {
			patterns = append(patterns, pnpm.Packages...)
		}
	}
	var list []string
	var object struct {
		Packages []string `json:"packages"`
	}
	switch {
	case json.Unmarshal(root.Workspaces, &list) == nil:
		patterns = append(patterns, list...)
	case json.Unmarshal(root.Workspaces, &object) == nil:
		patterns = append(patterns, object.Packages...)
	}
	return patterns
}

// selected reports whether a pattern selects dir and no negated one ("!") leaves it out.
func selected(patterns []string, dir string) bool {
	in := false
	for _, pattern := range patterns {
		if negated, found := strings.CutPrefix(pattern, "!"); found {
			in = in && !project.Matches(negated, dir)
			continue
		}
		in = in || project.Matches(pattern, dir)
	}
	return in
}

// packageDirs are the folders below the root holding a package.json, in the order a walk meets them.
func (r Repository) packageDirs() []string {
	var dirs []string
	_ = filepath.WalkDir(r.Dir, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(r.Dir, at)
		rel = filepath.ToSlash(rel)
		switch {
		case rel == ".":
			return nil
		case ignored[entry.Name()] || strings.Count(rel, "/") >= placeDepth:
			return filepath.SkipDir
		}
		if _, err := os.Lstat(filepath.Join(at, ".git")); err == nil {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(at, "package.json")); err == nil {
			dirs = append(dirs, rel)
		}
		return nil
	})
	return dirs
}
