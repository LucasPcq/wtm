// Package registry keeps repos.json, the repositories wtm knows, for a stream that follows every one of them.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	globalDirName = "wtm"
	fileName      = "repos.json"
	stateDirName  = "wtm"
	configName    = "config.toml"
)

// A Registry is the file at Path listing the repositories wtm knows, a JSON
// array shared with wtm; Now stamps a repository added (time.Now when nil).
type Registry struct {
	Path string
	Now  func() time.Time
}

// Path returns where the registry lies, under the user's config directory, or "" when that directory is unknown.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, globalDirName, fileName)
}

// entry is one repository of the file: the key, its common directory, its main worktree and when it was added.
type entry struct {
	CommonDir string `json:"common_dir"`
	Root      string `json:"root"`
	AddedAt   string `json:"added_at"`
}

// Add lists repo, keyed by its common directory, and reports whether it was
// not listed yet. The common case, a repository already listed, reads the file
// and nothing else. A file that does not parse is rebuilt from empty: every
// repository joins it again the next time a command runs in it.
func (r Registry) Add(ctx context.Context, repo domain.Repo) (bool, error) {
	if listed, err := r.read(); err == nil && listing(listed, repo.CommonDir) {
		return false, nil
	}

	added := false
	_, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: r.Path,
		Mutate: func(current []byte) (statefile.Change, error) {
			entries := decoded(current)
			if listing(entries, repo.CommonDir) {
				return statefile.Keep{}, nil
			}

			added = true
			return written(append(entries, entry{CommonDir: repo.CommonDir, Root: repo.Root, AddedAt: r.now().UTC().Format(time.RFC3339Nano)}))
		},
	})
	return added && err == nil, err
}

// Prune drops the repositories no longer initialized, their wtm config gone
// with them or on its own, and returns those it keeps and those it dropped. When
// every one is still initialized it reads the file and nothing else.
func (r Registry) Prune(ctx context.Context) (kept, dropped []domain.Repo, err error) {
	if listed, err := r.read(); err == nil && !slices.ContainsFunc(listed, gone) {
		return repos(listed), nil, nil
	}

	_, err = statefile.Update(ctx, statefile.UpdateParams{
		Path: r.Path,
		Mutate: func(current []byte) (statefile.Change, error) {
			var left []entry
			kept, dropped = nil, nil
			for _, e := range decoded(current) {
				if gone(e) {
					dropped = append(dropped, repoOf(e))
					continue
				}
				left = append(left, e)
				kept = append(kept, repoOf(e))
			}
			return written(left)
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return kept, dropped, nil
}

// read returns the entries of the file, none when it does not exist.
func (r Registry) read() ([]entry, error) {
	data, err := os.ReadFile(r.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []entry
	return entries, json.Unmarshal(data, &entries)
}

func (r Registry) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// decoded returns the entries of data, none when it is empty or does not parse.
func decoded(data []byte) []entry {
	var entries []entry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

// written is the file listing entries, an empty array for none.
func written(entries []entry) (statefile.Change, error) {
	if entries == nil {
		entries = []entry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, err
	}
	return statefile.Write{Data: append(data, '\n')}, nil
}

func listing(entries []entry, commonDir string) bool {
	return slices.ContainsFunc(entries, func(e entry) bool { return e.CommonDir == commonDir })
}

// gone reports whether the repository of e is no longer initialized: its wtm config is missing.
func gone(e entry) bool {
	_, err := os.Stat(filepath.Join(e.CommonDir, stateDirName, configName))
	return err != nil
}

func repos(entries []entry) []domain.Repo {
	list := make([]domain.Repo, 0, len(entries))
	for _, e := range entries {
		list = append(list, repoOf(e))
	}
	return list
}

func repoOf(e entry) domain.Repo {
	return domain.Repo{Root: e.Root, CommonDir: e.CommonDir}
}
