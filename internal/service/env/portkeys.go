package env

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type WritePortKeysParams struct {
	ProjectDir string
	Writes     []domain.PortKeyWrite
}

// WritePortKeys materializes each declared port as a .env key, in the value file
// and in its committed template. It returns what it actually changed, so a
// report never announces a write the files already held.
//
// The template is only ever amended, never created: adding a committed file
// nobody asked for is not this pass's business. The value file is created,
// because provisioning creates it anyway.
func WritePortKeys(params WritePortKeysParams) ([]domain.PortKeyWrite, error) {
	applied := make([]domain.PortKeyWrite, 0, len(params.Writes))
	for _, write := range params.Writes {
		value := strconv.Itoa(write.Base)

		changed, err := upsertEnvKey(upsertEnvKeyParams{
			Path: filepath.Join(params.ProjectDir, write.File), Key: write.Port, Value: value, Create: true,
		})
		if err != nil {
			return nil, err
		}

		if write.Template != "" {
			templateChanged, templateErr := upsertEnvKey(upsertEnvKeyParams{
				Path: filepath.Join(params.ProjectDir, write.Template), Key: write.Port, Value: value,
			})
			if templateErr != nil {
				return nil, templateErr
			}
			changed = changed || templateChanged
		}

		if changed {
			applied = append(applied, write)
		}
	}
	return applied, nil
}

type upsertEnvKeyParams struct {
	Path  string
	Key   string
	Value string
	// Create writes a file that does not exist. The value file is created —
	// provisioning creates it anyway — and the committed template never is.
	Create bool
}

func upsertEnvKey(params upsertEnvKeyParams) (bool, error) {
	if !params.Create && !infra.FileExists(params.Path) {
		return false, nil
	}

	lines, err := readEnvFile(params.Path)
	if err != nil {
		return false, err
	}

	updated, changed := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: params.Key, Value: params.Value})
	if !changed {
		return false, nil
	}
	return true, writeEnvFile(params.Path, rules.RenderEnv(updated))
}
