package detect

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

// BaseBranch returns the default remote branch for the repo at projectDir.
func BaseBranch(ctx context.Context, projectDir string) string {
	return infra.BaseBranch(ctx, projectDir)
}

// Branches returns the local and origin remote-tracking branches offered as
// base-branch candidates in the init wizard. Errors are swallowed (best effort):
// a repo with no branches yet simply yields an empty list and the wizard falls
// back to the detected/typed base.
func Branches(ctx context.Context, projectDir string) []domain.BranchCandidate {
	return branch.Candidates(ctx, branch.ListParams{ProjectDir: projectDir})
}
