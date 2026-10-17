package jobenv

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

func ComposeDirs(cfg run.Config) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, job := range cfg.Jobs {
		dir := cmp.Or(job.Cwd, ".")
		if !job.RunsCompose() || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func ComposeEnvPaths(cfg run.Config) []string {
	dirs := ComposeDirs(cfg)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, ".env"))
	}
	return paths
}

func declaredComposeProject(files [][]dotenv.Line) string {
	for _, lines := range files {
		at := slices.IndexFunc(lines, func(line dotenv.Line) bool { return line.Kind == dotenv.Pair && line.Key == ComposeProjectKey })
		if at >= 0 && lines[at].Value != "" {
			return lines[at].Value
		}
	}
	return ""
}
