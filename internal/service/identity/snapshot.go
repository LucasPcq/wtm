// Package identity is who a worktree is, read from one snapshot; only Allocate gives it a number.
package identity

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/worktree"
	"github.com/LucasPcq/wtm/internal/store"
)

// Snapshot is read once; every question below is answered from it.
type Snapshot struct {
	Worktrees []git.Worktree
	// Records holds one entry per live worktree on a branch.
	Records map[string]Record
}

//go-sumtype:decl Record

type Record interface{ record() }

type Recorded struct {
	Meta domain.WorktreeMetadata
}

// Unrecorded is a worktree wtm never wrote for: the main checkout, or one
// created by hand.
type Unrecorded struct{}

// Unreadable is kept rather than failing the snapshot: a neighbour's broken
// record fails only the question that needs it.
type Unreadable struct {
	Err error
}

func (Recorded) record()   {}
func (Unrecorded) record() {}
func (Unreadable) record() {}

type ReadParams struct {
	Git  git.Reader
	Meta store.Meta
}

// Read loads each live worktree's record by Load rather than Records: Records
// skips what it cannot read, and an unreadable number must never be handed out
// again.
func Read(ctx context.Context, params ReadParams) (Snapshot, error) {
	worktrees, err := params.Git.Worktrees(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list worktrees: %w", err)
	}
	records := make(map[string]Record, len(worktrees))
	for _, wt := range worktrees {
		branch := branchOf(wt)
		if branch == "" {
			continue
		}
		records[branch] = recordOf(recordOfParams{Meta: params.Meta, Branch: branch})
	}
	return Snapshot{Worktrees: worktrees, Records: records}, nil
}

type recordOfParams struct {
	Meta   store.Meta
	Branch string
}

func recordOf(params recordOfParams) Record {
	stored, err := params.Meta.Load(params.Branch)
	if errors.Is(err, os.ErrNotExist) {
		return Unrecorded{}
	}
	if err != nil {
		return Unreadable{Err: err}
	}
	return Recorded{Meta: stored}
}

// branchOf is empty for a worktree git cannot name: a detached HEAD has no
// record to look up, and keeps whatever it was given under the branch it left.
func branchOf(wt git.Worktree) string {
	switch state := wt.State.(type) {
	case git.OnBranch:
		return state.Branch
	case git.Rebasing:
		return state.Branch
	case git.Detached, git.Bare:
	}
	return ""
}

func (s Snapshot) IsMain(branch string) bool {
	for _, wt := range s.Worktrees {
		if wt.Main {
			return branchOf(wt) == branch
		}
	}
	return false
}

// Ordinal never allocates: a worktree without a settled number answers
// domain.ErrOrdinalUnallocated. A number another live worktree also claims is
// not settled, so whichever of the two sorts last is reallocated.
func (s Snapshot) Ordinal(branch string) (int, error) {
	if s.IsMain(branch) {
		return domain.MainWorktreeOrdinal, nil
	}
	others, err := s.others(branch)
	if err != nil {
		return 0, err
	}
	recorded, ok := s.Records[branch].(Recorded)
	if !ok || !rules.KeepsOrdinal(rules.KeepsOrdinalParams{Branch: branch, Ordinal: recorded.Meta.Ordinal, Others: others}) {
		return 0, domain.ErrOrdinalUnallocated
	}
	return recorded.Meta.Ordinal, nil
}

// others is what every live worktree but this one holds. A record that exists
// but cannot be read is an error, not a worktree holding nothing: handing its
// number to someone else is the collision numbering exists to prevent.
func (s Snapshot) others(branch string) ([]rules.OrdinalHolder, error) {
	holders := make([]rules.OrdinalHolder, 0, len(s.Worktrees))
	for _, wt := range s.Worktrees {
		other := branchOf(wt)
		if other == "" || other == branch {
			continue
		}
		if wt.Main {
			holders = append(holders, rules.OrdinalHolder{Branch: other, Ordinal: domain.MainWorktreeOrdinal})
			continue
		}
		switch record := s.Records[other].(type) {
		case Recorded:
			holders = append(holders, rules.OrdinalHolder{Branch: other, Ordinal: record.Meta.Ordinal})
		case Unreadable:
			return nil, fmt.Errorf("%w: %s: %w", domain.ErrOrdinalUnreadable, other, record.Err)
		case Unrecorded:
		}
	}
	return holders, nil
}

// Isolation is the recorded choice; a worktree with no readable record, the
// main checkout among them, is isolated.
func (s Snapshot) Isolation(branch string) domain.Isolation {
	recorded, ok := s.Records[branch].(Recorded)
	if !ok {
		return domain.IsolationIsolated
	}
	return rules.EffectiveIsolation(recorded.Meta.Isolation)
}

func (s Snapshot) Managed(branch string) bool {
	recorded, ok := s.Records[branch].(Recorded)
	return ok && worktree.Managed(recorded.Meta)
}

// NameClash finds a live worktree whose derived names branch would share. The
// names exist for the run module only: a caller asks when run.toml declares
// jobs.
func (s Snapshot) NameClash(branch string) (domain.WorktreeNameClash, bool) {
	live := make([]string, 0, len(s.Worktrees))
	for _, wt := range s.Worktrees {
		live = append(live, branchOf(wt))
	}
	return rules.WorktreeNameClash(rules.WorktreeNameClashParams{Branch: branch, Live: live})
}

// CheckedOutAt is the worktree that has branch out, empty when none does.
func (s Snapshot) CheckedOutAt(branch string) string {
	for _, wt := range s.Worktrees {
		if branchOf(wt) == branch {
			return wt.Path
		}
	}
	return ""
}

// Occupant is the branch of the worktree registered at path, empty when none
// is or its HEAD is detached.
func (s Snapshot) Occupant(path string) string {
	for _, wt := range s.Worktrees {
		if wt.Path == path {
			return branchOf(wt)
		}
	}
	return ""
}
