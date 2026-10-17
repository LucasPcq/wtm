package selfupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func StatePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalStateFile), nil
}

// LoadState reads the update-check state. A missing or corrupt file means
// "never checked", never an error: the notifier must not fail a command.
func LoadState() domain.UpdateState {
	path, err := StatePath()
	if err != nil {
		return domain.UpdateState{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.UpdateState{}
	}

	var state domain.UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.UpdateState{}
	}

	return state
}

// SaveState writes atomically: the refresh goroutine dies with the process by
// design, and a partial file would cost an extra network call to recover from.
func SaveState(ctx context.Context, state domain.UpdateState) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:   path,
		Mutate: func([]byte) (fs.Change, error) { return fs.Write{Data: data}, nil },
	})
	return err
}
