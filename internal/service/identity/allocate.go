package identity

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
)

type AllocateParams struct {
	Git      git.Reader
	Meta     store.Meta
	StateDir string
	Branch   string
}

// Claim is a worktree's number, and whether this call gave it: only an
// allocation is a change for the caller to publish. Unlocked is an allocation
// made without its locks, which the caller reports.
type Claim struct {
	Ordinal   int
	Allocated bool
	Unlocked  bool
}

// Allocate gives the worktree the smallest number no live worktree holds, the
// first time one is needed, and keeps it after. The main checkout is 0 and
// never written.
func Allocate(ctx context.Context, params AllocateParams) (Claim, error) {
	if params.StateDir == "" || params.Branch == "" {
		return Claim{}, domain.ErrOrdinalRefIncomplete
	}
	if claim, settled, err := settledClaim(ctx, params); settled || err != nil {
		return claim, err
	}

	var claim Claim
	lock, err := fs.WithFileLock(ctx, fs.WithFileLockParams{
		Path: filepath.Join(params.StateDir, domain.OrdinalLockFileName),
		Do: func() error {
			// Everything is read again under the lock, git included: the
			// snapshot above may predate a worktree another process has since
			// created and numbered.
			snapshot, err := Read(ctx, ReadParams{Git: params.Git, Meta: params.Meta})
			if err != nil {
				return err
			}
			claim, err = allocateIn(ctx, allocateInParams{Allocate: params, Snapshot: snapshot})
			return err
		},
	})
	if err != nil {
		return Claim{}, err
	}
	claim.Unlocked = claim.Unlocked || !lock.Held
	return claim, nil
}

func settledClaim(ctx context.Context, params AllocateParams) (Claim, bool, error) {
	snapshot, err := Read(ctx, ReadParams{Git: params.Git, Meta: params.Meta})
	if err != nil {
		return Claim{}, false, err
	}
	ordinal, err := snapshot.Ordinal(params.Branch)
	if errors.Is(err, domain.ErrOrdinalUnallocated) {
		return Claim{}, false, nil
	}
	return Claim{Ordinal: ordinal}, err == nil, err
}

type allocateInParams struct {
	Allocate AllocateParams
	Snapshot Snapshot
}

func allocateIn(ctx context.Context, params allocateInParams) (Claim, error) {
	branch := params.Allocate.Branch
	ordinal, err := params.Snapshot.Ordinal(branch)
	if !errors.Is(err, domain.ErrOrdinalUnallocated) {
		return Claim{Ordinal: ordinal}, err
	}
	others, err := params.Snapshot.others(branch)
	if err != nil {
		return Claim{}, err
	}
	next := rules.AllocateOrdinal(rules.TakenOrdinals(others))
	written, err := params.Allocate.Meta.Update(ctx, store.MetaUpdate{
		Branch:  branch,
		Missing: store.MissingCreate,
		Change: func(meta *domain.WorktreeMetadata) error {
			meta.Ordinal = next
			return nil
		},
	})
	if err != nil {
		return Claim{}, err
	}
	return Claim{Ordinal: next, Allocated: true, Unlocked: written.Unlocked}, nil
}
