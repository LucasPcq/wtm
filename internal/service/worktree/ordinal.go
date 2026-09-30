package worktree

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// WorktreeRef names one worktree: the repository it belongs to, where its state
// is kept, and which branch it holds. It is what every worktree-scoped lookup
// needs and all any of them needs.
type WorktreeRef struct {
	ProjectDir string
	StateDir   string
	Branch     string
}

// EnsureOrdinal returns the worktree's stable number, allocating and persisting
// one the first time it is asked for. The main checkout is ordinal 0 and is
// never written: it has no meta.json, so 0 in a linked worktree's metadata can
// only mean "not allocated yet".
//
// Allocation reads the ordinals of the worktrees git still lists, not every
// meta.json in the state dir — a removed worktree must give its number back.
func EnsureOrdinal(params WorktreeRef) (int, error) {
	if params.ProjectDir == "" || params.StateDir == "" || params.Branch == "" {
		return 0, domain.ErrOrdinalRefIncomplete
	}

	claim, err := readClaim(params)
	if err != nil {
		return 0, err
	}
	if claim.settled {
		return claim.ordinal, nil
	}

	allocated := 0
	lockErr := infra.WithFileLock(infra.WithFileLockParams{
		Path: filepath.Join(params.StateDir, domain.OrdinalLockFileName),
		Do: func() error {
			// Everything is read again here, git included. The claim above was
			// taken outside the lock, so the worktree list it saw may predate a
			// worktree another process has since created and numbered.
			fresh, err := readClaim(params)
			if err != nil {
				return err
			}
			if fresh.settled {
				allocated = fresh.ordinal
				return nil
			}
			allocated, err = persistOrdinal(params, rules.AllocateOrdinal(rules.TakenOrdinals(fresh.others)))
			return err
		},
	})
	if lockErr != nil {
		return 0, lockErr
	}
	return allocated, nil
}

// claim is what one look at the repository says about a worktree's number:
// either it is settled, or it has to be allocated against what the others hold.
type claim struct {
	settled bool
	ordinal int
	others  []rules.OrdinalHolder
}

func readClaim(params WorktreeRef) (claim, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return claim{}, fmt.Errorf("list worktrees: %w", err)
	}

	for _, wt := range worktrees {
		if wt.Branch == params.Branch && wt.IsMain {
			return claim{settled: true, ordinal: domain.MainWorktreeOrdinal}, nil
		}
	}

	others, err := otherHolders(otherHoldersParams{Worktrees: worktrees, Ref: params})
	if err != nil {
		return claim{}, err
	}

	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return claim{others: others}, nil
	}

	settled := rules.KeepsOrdinal(rules.KeepsOrdinalParams{
		Branch:  params.Branch,
		Ordinal: meta.Ordinal,
		Others:  others,
	})
	return claim{settled: settled, ordinal: meta.Ordinal, others: others}, nil
}

func persistOrdinal(params WorktreeRef, ordinal int) (int, error) {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		meta = domain.WorktreeMetadata{}
	}
	meta.Ordinal = ordinal

	if err := writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), meta); err != nil {
		return 0, err
	}
	return ordinal, nil
}

type otherHoldersParams struct {
	Worktrees []domain.GitWorktree
	Ref       WorktreeRef
}

// otherHolders reads what every live worktree but this one holds. The main
// checkout holds 0 by definition and has no metadata to read.
//
// A worktree whose metadata exists but cannot be read is an error, not a
// worktree holding nothing: handing its number to someone else is exactly the
// collision this package exists to prevent.
func otherHolders(params otherHoldersParams) ([]rules.OrdinalHolder, error) {
	holders := make([]rules.OrdinalHolder, 0, len(params.Worktrees))
	for _, wt := range params.Worktrees {
		if wt.Branch == params.Ref.Branch {
			continue
		}
		if wt.IsMain {
			holders = append(holders, rules.OrdinalHolder{Branch: wt.Branch, Ordinal: domain.MainWorktreeOrdinal})
			continue
		}
		// A worktree git cannot name (a detached HEAD) has no metadata to look
		// up: it is skipped, and keeps whatever number it was given under the
		// branch it left.
		if wt.Branch == "" {
			continue
		}

		meta, err := loadMetadata(params.Ref.StateDir, wt.Branch)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", domain.ErrOrdinalUnreadable, wt.Branch, err)
		}
		holders = append(holders, rules.OrdinalHolder{Branch: wt.Branch, Ordinal: meta.Ordinal})
	}
	return holders, nil
}

// purgeState drops a removed worktree's metadata directory, which nothing else
// ever deletes — leaving it behind would keep its ordinal reserved for good.
// Best effort: leftover state is not worth failing a removal that happened.
func purgeState(ref WorktreeRef) {
	dir := rules.PurgeableMetaDir(rules.PurgeableMetaDirParams{StateDir: ref.StateDir, Branch: ref.Branch})
	if dir == "" {
		return
	}
	_ = os.RemoveAll(dir)
}
