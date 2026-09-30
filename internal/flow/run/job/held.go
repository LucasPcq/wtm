package job

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// heldBy names what still holds data in a shared job: the worktrees whose
// meta.json records a namespace in it, and the drops a clean left owing. Both
// are found by the job's name, which is why removing or renaming it loses them.
func heldBy(ctx flow.Context, name string) []string {
	held := worktree.NamespaceHolders(worktree.NamespaceHoldersParams{StateDir: ctx.StateDir, Job: name})
	for _, ref := range runjobs.LoadPendingRemovals(ctx.StateDir) {
		if ref.Job == name {
			held = append(held, fmt.Sprintf(domain.RunJobHeldPendingFmt, ref.Worktree))
		}
	}
	return held
}

// publishedUnderName says the job's address is derived from its name, so a
// rename moves it.
func publishedUnderName(job domain.JobConfig) bool {
	return job.URL != nil && job.URL.Port != "" && job.URL.Host == ""
}
