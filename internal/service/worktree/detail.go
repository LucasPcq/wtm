package worktree

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/store"
)

// ParentBranchParams holds inputs for resolving a worktree's parent branch.
type ParentBranchParams struct {
	StateDir string
	Branch   string
}

// ParentBranch returns the branch the given worktree was created from, read from
// its metadata. Returns an empty string when no metadata is recorded.
func ParentBranch(params ParentBranchParams) string {
	return loadSourceBranch(params.StateDir, params.Branch)
}

// Metadata returns the recorded metadata for a worktree and true, or a zero
// value and false when none is recorded. Callers that need the memorized env
// strategy or parent branch (e.g. `wtm env`) read it through here.
func Metadata(params ParentBranchParams) (domain.WorktreeMetadata, bool) {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return domain.WorktreeMetadata{}, false
	}
	return meta, true
}

func loadSourceBranch(stateDir, branch string) string {
	meta, err := loadMetadata(stateDir, branch)
	if err != nil {
		return ""
	}
	return meta.SourceBranch
}

func loadMetadata(stateDir, branch string) (domain.WorktreeMetadata, error) {
	return (store.MetaDisk{StateDir: stateDir}).Load(branch)
}

type RecordNamespacesParams struct {
	StateDir string
	Branch   string
	Jobs     []string
}

// RecordNamespaces remembers that this worktree holds a namespace in each of these
// shared services, so `clean` gives back exactly what exists. Additive and
// idempotent: a job already recorded is not recorded twice, and a worktree with
// no metadata — the main checkout — records nothing rather than creating some.
// Any other failure is returned: a namespace nobody recorded is one no clean
// will ever drop.
func RecordNamespaces(ctx context.Context, params RecordNamespacesParams) error {
	if len(params.Jobs) == 0 {
		return nil
	}
	_, err := (store.MetaDisk{StateDir: params.StateDir}).Update(ctx, store.MetaUpdate{
		Branch:  params.Branch,
		Missing: store.MissingSkip,
		Change: func(meta *domain.WorktreeMetadata) error {
			for _, job := range params.Jobs {
				if !slices.Contains(meta.Namespaces, job) {
					meta.Namespaces = append(meta.Namespaces, job)
				}
			}
			return nil
		},
	})
	return err
}

// NamespacesOf is what this worktree has to give back when it goes.
func NamespacesOf(params ParentBranchParams) []string {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return nil
	}
	return meta.Namespaces
}
