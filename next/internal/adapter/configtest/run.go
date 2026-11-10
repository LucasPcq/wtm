package configtest

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// SavedRun returns run.toml with a digest as config.Project.SavedRun does: an empty run config and no digest when there is none. The digest changes with each SaveRun and EditRun.
func (f *Fake) SavedRun() (project.SavedRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("SavedRun"); err != nil {
		return project.SavedRun{}, err
	}
	return project.SavedRun{Run: f.run, Digest: f.runDigest()}, nil
}

// SaveRun replaces run.toml as config.Project.SaveRun does, refusing with project.ErrRunChanged a rewrite of a run.toml it no longer holds; it does not check the run config, which config.Project does.
func (f *Fake) SaveRun(_ context.Context, rewrite project.RunRewrite) (domain.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("SaveRun"); err != nil {
		return domain.Written{}, err
	}
	if rewrite.Replaces != f.runDigest() {
		return domain.Written{}, project.ErrRunChanged
	}

	f.setRun(rewrite.Run)
	return domain.Written{Changed: true}, nil
}

// EditRun replaces run.toml behind the use case's back, as a user editing it would.
func (f *Fake) EditRun(run project.Run) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setRun(run)
}

func (f *Fake) setRun(run project.Run) {
	run.Declared = true
	f.run = run
	f.runVersion++
}

func (f *Fake) runDigest() string {
	if !f.run.Declared {
		return ""
	}
	return fmt.Sprintf("run.toml#%d", f.runVersion)
}
