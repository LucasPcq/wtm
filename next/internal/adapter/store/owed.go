package store

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

const (
	owedFileName = "pending-removals.toml"
	// owedSchema is written once an entry keeps a key the legacy wtm does not know: it reads such a file, and refuses to rewrite it.
	owedSchema = 2
)

// Owed keeps pending-removals.toml, the legacy wtm's queue of the namespaces
// removed worktrees made in shared services and still owe, under the
// project's state directory StateDir: beside the records, since a
// worktree's own state goes with it.
type Owed struct {
	StateDir string
}

type owedFile struct {
	Schema  *int        `toml:"schema,omitempty"`
	Pending []owedEntry `toml:"pending"`
}

type owedEntry struct {
	Job      string            `toml:"job"`
	Worktree string            `toml:"worktree"`
	Ordinal  int               `toml:"ordinal"`
	Name     string            `toml:"name,omitempty"`
	Env      map[string]string `toml:"env,omitempty"`
}

func (o Owed) path() string {
	return filepath.Join(o.StateDir, owedFileName)
}

// Load returns the drops owed, none when the file does not exist or does not
// decode: a debt that cannot be read cannot be paid either. A file a newer
// wtm wrote is a *domain.SchemaNewer.
func (o Owed) Load() ([]env.OwedDrop, error) {
	data, err := os.ReadFile(o.path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return o.decode(data)
}

// Update applies change to the drops owed under the file's lock, the file
// removed once none is owed. A file that does not decode counts as empty; one
// a newer wtm wrote is refused with a *domain.SchemaNewer.
func (o Owed) Update(ctx context.Context, change func([]env.OwedDrop) []env.OwedDrop) error {
	_, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: o.path(),
		Mutate: func(current []byte) (statefile.Change, error) {
			owed, err := o.decode(current)
			if err != nil {
				return nil, err
			}
			next := change(owed)
			if len(next) == 0 {
				return statefile.Remove{}, nil
			}
			return encodeOwed(next)
		},
	})
	return err
}

func (o Owed) decode(data []byte) ([]env.OwedDrop, error) {
	var file owedFile
	if _, err := toml.Decode(string(data), &file); err != nil {
		return nil, nil
	}
	var found int
	if file.Schema != nil {
		found = *file.Schema
	}
	if err := statefile.Refusal(statefile.Schema{Path: o.path(), Found: found, Supported: owedSchema}); err != nil {
		return nil, err
	}

	owed := make([]env.OwedDrop, 0, len(file.Pending))
	for _, entry := range file.Pending {
		owed = append(owed, env.OwedDrop{Job: entry.Job, Worktree: entry.Worktree, Ordinal: entry.Ordinal, Name: entry.Name, Env: entry.Env})
	}
	return owed, nil
}

func encodeOwed(owed []env.OwedDrop) (statefile.Change, error) {
	file := owedFile{}
	for _, drop := range owed {
		file.Pending = append(file.Pending, owedEntry{Job: drop.Job, Worktree: drop.Worktree, Ordinal: drop.Ordinal, Name: drop.Name, Env: drop.Env})
	}
	if slices.ContainsFunc(owed, func(drop env.OwedDrop) bool { return drop.Name != "" || len(drop.Env) > 0 }) {
		file.Schema = new(owedSchema)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(file); err != nil {
		return nil, err
	}
	return statefile.Write{Data: buf.Bytes()}, nil
}
