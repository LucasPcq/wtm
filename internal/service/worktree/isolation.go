package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// IsolationOf is the choice recorded for this worktree. A worktree with no
// metadata — main, or one wtm never touched — is isolated.
func IsolationOf(ref WorktreeRef) domain.Isolation {
	meta, err := loadMetadata(ref.StateDir, ref.Branch)
	if err != nil {
		return domain.IsolationIsolated
	}
	return rules.EffectiveIsolation(meta.Isolation)
}

// RecordedIsolation is the choice meta.json holds, empty for a worktree that
// never made one — created before the choice existed, or never by wtm.
func RecordedIsolation(ref WorktreeRef) domain.Isolation {
	meta, err := loadMetadata(ref.StateDir, ref.Branch)
	if err != nil {
		return ""
	}
	return meta.Isolation
}

type SetIsolationParams struct {
	Ref       WorktreeRef
	Isolation domain.Isolation
}

// SetIsolation records a new choice for an existing worktree. It writes the
// record only: bringing the .env in line with it is the caller's next step.
func SetIsolation(params SetIsolationParams) error {
	isMain, err := isMainBranch(params.Ref)
	if err != nil {
		return err
	}
	if isMain {
		if rules.IsVerbatim(params.Isolation) {
			return domain.ErrIsolationMain
		}
		return nil
	}

	meta, err := loadMetadata(params.Ref.StateDir, params.Ref.Branch)
	if err != nil {
		meta = domain.WorktreeMetadata{}
	}
	meta.Isolation = rules.EffectiveIsolation(params.Isolation)
	return writeMetadata(rules.WorktreeMetaDir(params.Ref.StateDir, params.Ref.Branch), meta)
}

func isMainBranch(ref WorktreeRef) (bool, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: ref.ProjectDir})
	if err != nil {
		return false, fmt.Errorf("list worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.IsMain && wt.Branch == ref.Branch {
			return true, nil
		}
	}
	return false, nil
}
