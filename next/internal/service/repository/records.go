package repository

import (
	"errors"
	"io/fs"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Records reads the meta record of each of branches, "" (a worktree on none) skipped: a branch wtm did not create has no entry, one whose record exists but does not read has its error instead.
func Records(meta Meta, branches []string) (map[string]worktree.Meta, map[string]error) {
	records, errs := map[string]worktree.Meta{}, map[string]error{}
	for _, branch := range branches {
		if branch == "" {
			continue
		}
		record, err := meta.Load(branch)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			errs[branch] = err
		default:
			records[branch] = record
		}
	}

	return records, errs
}
