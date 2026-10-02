package worktree

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// Ordinal reads the worktree's number without ever allocating one. A recorded
// number another worktree also claims is not settled, and reads as unallocated:
// the flow that next needs it reallocates it.
func Ordinal(ref WorktreeRef) (int, error) {
	claim, err := readClaim(ref)
	if err != nil {
		return 0, err
	}
	if !claim.settled {
		return 0, domain.ErrOrdinalUnallocated
	}
	return claim.ordinal, nil
}

// Identity is what `wtm events` reports about one worktree: read from git and
// meta.json only, never from the network.
func Identity(ref WorktreeRef) (domain.WorktreeIdentity, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: ref.ProjectDir})
	if err != nil {
		return domain.WorktreeIdentity{}, fmt.Errorf("list worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.Branch == ref.Branch {
			return identityOf(identityOfParams{Worktree: wt, Ref: ref}), nil
		}
	}
	return domain.WorktreeIdentity{}, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, ref.Branch)
}

type IdentitiesParams struct {
	ProjectDir string
	StateDir   string
}

// Identities leaves out a detached worktree: a consumer keys on the branch, and
// it has none.
func Identities(params IdentitiesParams) ([]domain.WorktreeIdentity, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	identities := make([]domain.WorktreeIdentity, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		ref := WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: wt.Branch}
		identities = append(identities, identityOf(identityOfParams{Worktree: wt, Ref: ref}))
	}
	return identities, nil
}

type identityOfParams struct {
	Worktree domain.GitWorktree
	Ref      WorktreeRef
}

func identityOf(params identityOfParams) domain.WorktreeIdentity {
	identity := domain.WorktreeIdentity{
		Branch:    params.Worktree.Branch,
		Path:      params.Worktree.Path,
		IsMain:    params.Worktree.IsMain,
		Isolation: IsolationOf(params.Ref),
	}
	if ordinal, err := Ordinal(params.Ref); err == nil {
		identity.Ordinal = &ordinal
	}
	if meta, err := loadMetadata(params.Ref.StateDir, params.Ref.Branch); err == nil {
		identity.Parent = meta.SourceBranch
		identity.CreatedAt = meta.CreatedAt
	}
	return identity
}

type RepoOfParams struct {
	ProjectDir string
}

// RepoOf names a repository the way every event does: by its main checkout, and
// by its git common dir, which is the key — the same from any of its worktrees.
// The key is resolved through symlinks: git spells a relative common dir from
// the path it was given, and a publisher and a subscriber reaching the repo by
// two spellings (macOS's /var and /private/var) would never meet.
func RepoOf(params RepoOfParams) (domain.EventRepo, error) {
	commonDir, err := infra.GitCommonDir(infra.GitCommonDirParams{Dir: params.ProjectDir})
	if err != nil {
		return domain.EventRepo{}, err
	}
	if resolved, err := filepath.EvalSymlinks(commonDir); err == nil {
		commonDir = resolved
	}
	return domain.EventRepo{Root: params.ProjectDir, CommonDir: commonDir}, nil
}
