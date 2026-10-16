package runenv

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const envFileName = ".env"

type composeNameParams struct {
	Project  string
	Worktree string
}

func composeProjectName(params composeNameParams) string {
	if params.Project == "" {
		return params.Worktree
	}
	return branch.Slug(params.Project) + "-" + params.Worktree
}

func declaredComposeProject(files [][]dotenv.Line) string {
	for _, lines := range files {
		if line, found := pairByKey(lines, envComposeProject); found && line.Value != "" {
			return line.Value
		}
	}
	return ""
}

func composeDirs(cfg run.Config) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, job := range cfg.Jobs {
		dir := jobDir(job.Cwd)
		if !job.RunsCompose() || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func ComposeEnvPaths(cfg run.Config) []string {
	dirs := composeDirs(cfg)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, envFileName))
	}
	return paths
}

func jobDir(cwd string) string {
	if cwd == "" {
		return "."
	}
	return cwd
}

func pairByKey(lines []dotenv.Line, key string) (dotenv.Line, bool) {
	for _, line := range lines {
		if line.Kind == dotenv.Pair && line.Key == key {
			return line, true
		}
	}
	return dotenv.Line{}, false
}
