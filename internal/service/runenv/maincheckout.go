package runenv

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type composeEnvFilesParams struct {
	Params
	Root string
}

func composeEnvFiles(params composeEnvFilesParams) ([][]domain.EnvLine, error) {
	dirs := rules.ComposeProjectDirs(params.Run)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	files := make([][]domain.EnvLine, 0, len(dirs))
	for _, dir := range dirs {
		lines, err := params.Files.Read(filepath.Join(params.Root, dir, domain.EnvFileName))
		if err != nil {
			return nil, err
		}
		files = append(files, lines)
	}
	return files, nil
}

type mainValueRepairsParams struct {
	Env    EnvParams
	Stamps []domain.EnvOwnedEntry
}

func mainValueRepairs(params mainValueRepairsParams) ([]domain.EnvOwnedEntry, error) {
	files := params.Env.Files
	mainPath := params.Env.WorktreePath
	current := map[string][]domain.EnvLine{}
	template := map[string][]domain.EnvLine{}
	for _, file := range params.Env.EnvFiles {
		if !stampsFile(stampsFileParams{Stamps: params.Stamps, Target: file.Target}) {
			continue
		}
		lines, err := files.Read(filepath.Join(mainPath, file.Target))
		if err != nil {
			return nil, err
		}
		current[file.Target] = lines
		source, err := files.Template(mainPath, file)
		if err != nil {
			return nil, err
		}
		if source == "" {
			continue
		}
		if template[file.Target], err = files.Read(source); err != nil {
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
