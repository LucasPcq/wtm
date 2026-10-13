package runenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
)

type composeEnvFilesParams struct {
	Dir    string
	Config domain.RunConfig
}

// composeEnvFiles reads the .env of each directory a compose stack starts from.
// A .env that cannot be read counts as one that names nothing.
func composeEnvFiles(params composeEnvFilesParams) [][]domain.EnvLine {
	dirs := rules.ComposeProjectDirs(params.Config)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	files := make([][]domain.EnvLine, 0, len(dirs))
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(params.Dir, dir, domain.EnvFileName))
		if err != nil {
			continue
		}
		files = append(files, rules.ParseEnv(string(data)))
	}
	return files
}

type mainValueRepairsParams struct {
	MainPath string
	Files    []domain.EnvFile
	Stamps   []domain.EnvOwnedEntry
}

// mainValueRepairs reads the main checkout's files and their templates for the
// [[env]] values wtm once wrote there, and plans putting them back.
func mainValueRepairs(params mainValueRepairsParams) ([]domain.EnvOwnedEntry, error) {
	current := map[string][]domain.EnvLine{}
	template := map[string][]domain.EnvLine{}
	for _, file := range params.Files {
		if !stampsFile(stampsFileParams{Stamps: params.Stamps, Target: file.Target}) {
			continue
		}
		lines, err := readEnvFile(filepath.Join(params.MainPath, file.Target))
		if err != nil {
			return nil, err
		}
		current[file.Target] = lines
		source := store.TemplateSource(params.MainPath, file)
		if source == "" {
			continue
		}
		if template[file.Target], err = readEnvFile(source); err != nil {
			return nil, err
		}
	}
	return rules.MainEnvValueRepairs(rules.MainEnvValueRepairsParams{Stamps: params.Stamps, Current: current, Template: template}), nil
}

type stampsFileParams struct {
	Stamps []domain.EnvOwnedEntry
	Target string
}

func stampsFile(params stampsFileParams) bool {
	for _, stamp := range params.Stamps {
		if stamp.File == params.Target {
			return true
		}
	}
	return false
}

// readEnvFile reads an absent file as an empty one.
func readEnvFile(path string) ([]domain.EnvLine, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return rules.ParseEnv(string(data)), nil
}
