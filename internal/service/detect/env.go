package detect

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvFiles scans the project directory recursively for env files and classifies
// them into value targets and their committed templates (see rules.ClassifyEnvFiles).
// It recognizes .env, .env.local, and the known template suffixes (.example, .dist,
// .sample, .template, .tmpl); multi-environment files (.env.production, …) are
// ignored. Excludes node_modules, .trees, .git, vendor, and dist directories.
func EnvFiles(projectDir string) []domain.EnvFile {
	var paths []string
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if rules.IsScanIgnoredDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !rules.IsEnvCandidate(info.Name()) {
			return nil
		}

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})

	sort.Strings(paths)
	return rules.ClassifyEnvFiles(paths)
}

// DockerComposeFiles scans for docker-compose*.yml and docker-compose*.yaml files.
func DockerComposeFiles(projectDir string) []string {
	var files []string

	patterns := []string{"docker-compose*.yml", "docker-compose*.yaml"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			rel, relErr := filepath.Rel(projectDir, m)
			if relErr == nil {
				files = append(files, rel)
			}
		}
	}

	sort.Strings(files)
	return files
}

// DockerComposeCommand returns the docker-compose invocation available on the host.
func DockerComposeCommand() string {
	return infra.DockerComposeCommand()
}
