// Package detect reads what a repository's files suggest of its settings: its .env files, the hooks a worktree would run and the folders one can run in.
package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ignored are the directories a scan never enters: dependencies, build output, git's own and wtm's worktrees; walking them costs more than the rest of the repository and finds nothing.
var ignored = map[string]bool{
	"node_modules": true, ".git": true, ".trees": true, ".next": true, ".turbo": true, ".venv": true,
	"vendor": true, "dist": true, "build": true, "target": true, "coverage": true,
}

// dirDepth is how deep the folders a hook can run in are listed.
const dirDepth = 2

// Repository is the working tree of a repository at Dir.
type Repository struct {
	Dir string
}

// Detect is what the repository's files suggest: each .env file and its template, the install hook its root says, the teardown of its root compose file, and its folders two levels deep. A folder holding another repository or a worktree (its own .git) is not entered; what cannot be read is left out.
func (r Repository) Detect() project.Detected {
	var envPaths, dirs, root []string
	_ = filepath.WalkDir(r.Dir, func(at string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.Dir, at)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			return r.enter(rel, entry.Name(), &dirs)
		}
		if !strings.Contains(rel, "/") {
			root = append(root, rel)
		}
		if project.EnvCandidate(entry.Name()) {
			envPaths = append(envPaths, rel)
		}
		return nil
	})

	detected := project.Detected{EnvFiles: project.EnvFilesOf(envPaths), Dirs: dirs}
	if install, found := project.InstallOf(r.packageManager(), root); found {
		detected.OnCreate = []project.FoundHook{install}
	}
	if teardown, found := project.TeardownOf(root); found {
		detected.OnClean = []project.FoundHook{teardown}
	}
	return detected
}

// enter decides whether the walk enters the directory at rel, named name, listing it among dirs when it is shallow and in no hidden folder.
func (r Repository) enter(rel, name string, dirs *[]string) error {
	if rel == "." {
		return nil
	}
	if ignored[name] {
		return filepath.SkipDir
	}
	if _, err := os.Lstat(filepath.Join(r.Dir, rel, ".git")); err == nil {
		return filepath.SkipDir
	}
	if !strings.HasPrefix(rel, ".") && !strings.Contains(rel, "/.") && strings.Count(rel, "/") < dirDepth {
		*dirs = append(*dirs, path.Clean(rel))
	}
	return nil
}

// packageManager is the packageManager field of the root's package.json, "" when it has none.
func (r Repository) packageManager() string {
	data, err := os.ReadFile(filepath.Join(r.Dir, "package.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	_ = json.Unmarshal(data, &manifest)
	return manifest.PackageManager
}
